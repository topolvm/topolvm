package cmd

import (
	"context"
	"fmt"
	snapshotscheme "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/controllers"
	"github.com/topolvm/topolvm/csi"
	"github.com/topolvm/topolvm/driver"
	"github.com/topolvm/topolvm/driver/k8s"
	"github.com/topolvm/topolvm/hook"
	"github.com/topolvm/topolvm/runners"
	"google.golang.org/grpc"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"net"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"time"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(snapshotscheme.AddToScheme(scheme))
	utilruntime.Must(topolvmv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

//+kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch

// Run builds and starts the manager with leader election.
func subMain() error {
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&config.zapOpts)))

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return err
	}

	hookHost, portStr, err := net.SplitHostPort(config.webhookAddr)
	if err != nil {
		return fmt.Errorf("invalid webhook addr: %v", err)
	}
	hookPort, err := net.LookupPort("tcp", portStr)
	if err != nil {
		return fmt.Errorf("invalid webhook port: %v", err)
	}
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: config.metricsAddr,
		LeaderElection:     true,
		LeaderElectionID:   config.leaderElectionID,
		Host:               hookHost,
		Port:               hookPort,
		CertDir:            config.certDir,
	})
	if err != nil {
		return err
	}

	// register webhook handlers
	// admissoin.NewDecoder never returns non-nil error
	dec, _ := admission.NewDecoder(scheme)
	wh := mgr.GetWebhookServer()
	wh.Register("/pod/mutate", hook.PodMutator(mgr.GetClient(), dec))
	wh.Register("/pvc/mutate", hook.PVCMutator(mgr.GetClient(), dec))

	// register controllers
	nodecontroller := &controllers.NodeReconciler{
		Client:           mgr.GetClient(),
		SkipNodeFinalize: config.skipNodeFinalize,
	}
	if err := nodecontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Node")
		return err
	}

	pvccontroller := &controllers.PersistentVolumeClaimReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
	}
	if err := pvccontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PersistentVolumeClaim")
		return err
	}

	//+kubebuilder:scaffold:builder

	// Add health checker to manager
	ctx := context.Background()
	check := func() error {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		var drv storagev1.CSIDriver
		return mgr.GetAPIReader().Get(ctx, types.NamespacedName{Name: topolvm.PluginName}, &drv)
	}
	checker := runners.NewChecker(check, 1*time.Minute)
	if err := mgr.Add(checker); err != nil {
		return err
	}

	// Add gRPC server to manager.
	s, err := k8s.NewLogicalVolumeService(mgr)
	if err != nil {
		return err
	}
	n := k8s.NewNodeService(mgr)

	grpcServer := grpc.NewServer()
	csi.RegisterIdentityServer(grpcServer, driver.NewIdentityService(checker.Ready))
	csi.RegisterControllerServer(grpcServer, driver.NewControllerService(s, n))

	// gRPC service itself should run even when the manager is *not* a leader
	// because CSI sidecar containers choose a leader.
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
