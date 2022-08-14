package cmd

import (
	"context"
	"errors"
	"net"
	"os"
	"time"

	"github.com/spf13/viper"
	"github.com/topolvm/topolvm"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/controllers"
	"github.com/topolvm/topolvm/csi"
	"github.com/topolvm/topolvm/driver"
	"github.com/topolvm/topolvm/driver/k8s"
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
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(topolvmv1.AddToScheme(scheme))
	utilruntime.Must(topolvmlegacyv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func subMain() error {
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

	dialer := &net.Dialer{}
	dialFunc := func(ctx context.Context, a string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", a)
	}
	conn, err := grpc.Dial(config.lvmdSocket, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialFunc))
	if err != nil {
		return err
	}
	defer conn.Close()

	lvcontroller := controllers.NewLogicalVolumeReconciler(
		mgr.GetClient(),
		nodename,
		conn,
	)

	if err := lvcontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LogicalVolume")
		return err
	}
	//+kubebuilder:scaffold:builder

	// Add health checker to manager
	checker := runners.NewChecker(checkFunc(conn, mgr.GetAPIReader()), 1*time.Minute)
	if err := mgr.Add(checker); err != nil {
		return err
	}

	// Add metrics exporter to manager.
	// Note that grpc.ClientConn can be shared with multiple stubs/services.
	// https://github.com/grpc/grpc-go/tree/master/examples/features/multiplex
	if err := mgr.Add(runners.NewMetricsExporter(conn, mgr, nodename)); err != nil {
		return err
	}

	// Add gRPC server to manager.
	s, err := k8s.NewLogicalVolumeService(mgr)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(driver.DeviceDirectory, 0755); err != nil {
		return err
	}
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(ErrorLoggingInterceptor))
	csi.RegisterIdentityServer(grpcServer, driver.NewIdentityService(checker.Ready))
	csi.RegisterNodeServer(grpcServer, driver.NewNodeService(nodename, conn, s))
	err = mgr.Add(runners.NewGRPCRunner(grpcServer, config.csiSocket, false))
	if err != nil {
		return err
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch

func checkFunc(conn *grpc.ClientConn, r client.Reader) func() error {
	vgs := proto.NewVGServiceClient(conn)
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if _, err := vgs.GetFreeBytes(ctx, &proto.GetFreeBytesRequest{DeviceClass: topolvm.DefaultDeviceClassName}); err != nil {
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
