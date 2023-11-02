package cmd

import (
	"context"
	"errors"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/cybozu-go/log"
	"github.com/spf13/viper"
	"github.com/topolvm/topolvm"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	clientwrapper "github.com/topolvm/topolvm/client"
	"github.com/topolvm/topolvm/controllers"
	"github.com/topolvm/topolvm/driver"
	"github.com/topolvm/topolvm/lvmd"
	"github.com/topolvm/topolvm/lvmd/command"
	"github.com/topolvm/topolvm/lvmd/proto"
	"github.com/topolvm/topolvm/runners"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"
	//+kubebuilder:scaffold:imports
)

var (
	scheme      = runtime.NewScheme()
	setupLog    = ctrl.Log.WithName("setup")
	cfgFilePath string
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(topolvmv1.AddToScheme(scheme))
	utilruntime.Must(topolvmlegacyv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func subMain(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	nodename := viper.GetString("nodename")
	if len(nodename) == 0 {
		return errors.New("node name is not given")
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&config.zapOpts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: config.metricsAddr,
		LeaderElection:     false,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}
	client := clientwrapper.NewWrappedClient(mgr.GetClient())
	apiReader := clientwrapper.NewWrappedReader(mgr.GetAPIReader(), mgr.GetClient().Scheme())

	var lvclnt proto.LVServiceClient
	var vgclnt proto.VGServiceClient

	if config.embedLvmd {
		log.DefaultLogger().SetFormatter(log.JSONFormat{})
		command.Containerized = true
		if err := loadConfFile(cfgFilePath); err != nil {
			return err
		}
		dcm := lvmd.NewDeviceClassManager(config.lvmd.DeviceClasses)
		ocm := lvmd.NewLvcreateOptionClassManager(config.lvmd.LvcreateOptionClasses)
		lvclnt, vgclnt = lvmd.NewEmbeddedServiceClients(ctx, dcm, ocm)
	} else {
		dialer := &net.Dialer{}
		dialFunc := func(ctx context.Context, a string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", a)
		}
		conn, err := grpc.Dial(config.lvmdSocket, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialFunc))
		if err != nil {
			return err
		}
		defer conn.Close()
		lvclnt, vgclnt = proto.NewLVServiceClient(conn), proto.NewVGServiceClient(conn)
	}

	lvcontroller := controllers.NewLogicalVolumeReconcilerWithServices(client, nodename, vgclnt, lvclnt)

	if err := lvcontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LogicalVolume")
		return err
	}
	//+kubebuilder:scaffold:builder

	// Add health checker to manager
	checker := runners.NewChecker(checkFunc(vgclnt, apiReader), 1*time.Minute) // adjusted signature
	if err := mgr.Add(checker); err != nil {
		return err
	}

	// Add metrics exporter to manager.
	// Note that grpc.ClientConn can be shared with multiple stubs/services.
	// https://github.com/grpc/grpc-go/tree/master/examples/features/multiplex
	if err := mgr.Add(runners.NewMetricsExporter(vgclnt, client, nodename)); err != nil { // adjusted signature
		return err
	}

	// Add gRPC server to manager.
	if err := os.MkdirAll(driver.DeviceDirectory, 0755); err != nil {
		return err
	}
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(ErrorLoggingInterceptor))
	csi.RegisterIdentityServer(grpcServer, driver.NewIdentityServer(checker.Ready))
	nodeServer, err := driver.NewNodeServer(nodename, vgclnt, lvclnt, mgr) // adjusted signature
	if err != nil {
		return err
	}
	csi.RegisterNodeServer(grpcServer, nodeServer)
	err = mgr.Add(runners.NewGRPCRunner(grpcServer, config.csiSocket, false))
	if err != nil {
		return err
	}

	c := make(chan os.Signal, 2)
	signal.Notify(c, []os.Signal{os.Interrupt, syscall.SIGTERM}...)
	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch

func checkFunc(clnt proto.VGServiceClient, r client.Reader) func() error {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if _, err := clnt.GetFreeBytes(ctx, &proto.GetFreeBytesRequest{DeviceClass: topolvm.DefaultDeviceClassName}); err != nil {
			return err
		}

		var drv storagev1.CSIDriver
		return r.Get(ctx, types.NamespacedName{Name: topolvm.GetPluginName()}, &drv)
	}
}

func ErrorLoggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	resp, err = handler(ctx, req)
	if err != nil {
		ctrl.Log.Error(err, "error on grpc call", "method", info.FullMethod)
	}
	return resp, err
}

func loadConfFile(cfgFilePath string) error {
	b, err := os.ReadFile(cfgFilePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(b, &config.lvmd)
	if err != nil {
		return err
	}
	log.Info("configuration file loaded: ", map[string]interface{}{
		"device_classes": config.lvmd.DeviceClasses,
		"socket_name":    config.lvmd.SocketName,
		"file_name":      cfgFilePath,
	})
	return nil
}
