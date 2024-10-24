package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/viper"
	"github.com/topolvm/topolvm"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	clientwrapper "github.com/topolvm/topolvm/internal/client"
	"github.com/topolvm/topolvm/internal/runners"
	"github.com/topolvm/topolvm/pkg/controller"
	"github.com/topolvm/topolvm/pkg/driver"
	"github.com/topolvm/topolvm/pkg/lvmd"
	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
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

	metricsServerOptions := metricsserver.Options{
		BindAddress: config.metricsAddr,
	}
	if config.secureMetricsServer {
		metricsServerOptions.SecureServing = true
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		Metrics:          metricsServerOptions,
		LeaderElection:   false,
		PprofBindAddress: config.profilingBindAddress,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}
	client := clientwrapper.NewWrappedClient(mgr.GetClient())
	apiReader := clientwrapper.NewWrappedReader(mgr.GetAPIReader(), mgr.GetClient().Scheme())

	var lvService proto.LVServiceClient
	var vgService proto.VGServiceClient
	var health grpc_health_v1.HealthClient

	if config.embedLvmd {
		if err := loadConfFile(ctx, cfgFilePath); err != nil {
			return err
		}

		lvService, vgService = lvmd.NewEmbeddedServiceClients(
			ctx,
			config.lvmd.DeviceClasses,
			config.lvmd.LvcreateOptionClasses,
		)
	} else {
		conn, err := grpc.NewClient(
			"unix:"+config.lvmdSocket,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		lvService, vgService = proto.NewLVServiceClient(conn), proto.NewVGServiceClient(conn)
		health = grpc_health_v1.NewHealthClient(conn)
	}

	lvmd.SetLVMPath(config.lvmPath)

	if err := controller.SetupLogicalVolumeReconcilerWithServices(
		mgr, client, nodename, vgService, lvService); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LogicalVolume")
		return err
	}
	//+kubebuilder:scaffold:builder

	// Add health checker to manager
	checker := runners.NewChecker(checkFunc(health, apiReader), 1*time.Minute) // adjusted signature
	if err := mgr.Add(checker); err != nil {
		return err
	}

	// Add metrics exporter to manager.
	// Note that grpc.ClientConn can be shared with multiple stubs/services.
	// https://github.com/grpc/grpc-go/tree/master/examples/features/multiplex
	if err := mgr.Add(runners.NewMetricsExporter(vgService, client, nodename)); err != nil { // adjusted signature
		return err
	}

	// Add gRPC server to manager.
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(ErrorLoggingInterceptor))
	csi.RegisterIdentityServer(grpcServer, driver.NewIdentityServer(checker.Ready))
	nodeServer, err := driver.NewNodeServer(nodename, vgService, lvService, mgr) // adjusted signature
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

func checkFunc(health grpc_health_v1.HealthClient, r client.Reader) func() error {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if health != nil {
			res, err := health.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
			if err != nil {
				return err
			}
			if status := res.GetStatus(); status != grpc_health_v1.HealthCheckResponse_SERVING {
				return fmt.Errorf("lvmd does not working: %s", status.String())
			}
		}

		var drv storagev1.CSIDriver
		return r.Get(ctx, types.NamespacedName{Name: topolvm.GetPluginName()}, &drv)
	}
}

func ErrorLoggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	resp, err = handler(ctx, req)
	if err != nil {
		ctrl.Log.Error(err, "error on grpc call", "method", info.FullMethod)
	}
	return resp, err
}

func loadConfFile(ctx context.Context, cfgFilePath string) error {
	b, err := os.ReadFile(cfgFilePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(b, &config.lvmd)
	if err != nil {
		return err
	}
	log.FromContext(ctx).Info("configuration file loaded",
		"device_classes", config.lvmd.DeviceClasses,
		"file_name", cfgFilePath,
	)
	return nil
}
