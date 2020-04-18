package cmd

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/cybozu-go/topolvm"
	topolvmv1 "github.com/cybozu-go/topolvm/api/v1"
	"github.com/cybozu-go/topolvm/controllers"
	"github.com/cybozu-go/topolvm/csi"
	"github.com/cybozu-go/topolvm/driver"
	"github.com/cybozu-go/topolvm/driver/k8s"
	"github.com/cybozu-go/topolvm/hook"
	"github.com/cybozu-go/topolvm/runners"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	if err := topolvmv1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		panic(err)
	}

	// +kubebuilder:scaffold:scheme
}

// Run builds and starts the manager with leader election.
func subMain() error {
	ctrl.SetLogger(zap.New(zap.UseDevMode(config.development)))

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
	wh.Register("/pod/mutate", hook.PodMutator(mgr.GetClient(), dec, config.defaultVG))
	wh.Register("/pvc/mutate", hook.PVCMutator(mgr.GetClient(), dec))

	// register controllers
	events := make(chan event.GenericEvent, 1)
	stopCh := ctrl.SetupSignalHandler()
	go func() {
		ticker := time.NewTicker(config.cleanupInterval)
		for {
			select {
			case <-stopCh:
				ticker.Stop()
				return
			case <-ticker.C:
				select {
				case events <- event.GenericEvent{
					Meta: &topolvmv1.LogicalVolume{},
				}:
				default:
				}
			}
		}
	}()

	lvcontroller := &controllers.LogicalVolumeCleanupReconciler{
		Client:      mgr.GetClient(),
		Log:         ctrl.Log.WithName("controllers").WithName("LogicalVolumeCleanup"),
		Events:      events,
		StalePeriod: config.stalePeriod,
	}
	if err := lvcontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LogicalVolumeCleanup")
		return err
	}

	nodecontroller := &controllers.NodeReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Node"),
	}
	if err := nodecontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Node")
		return err
	}

	pvccontroller := &controllers.PersistentVolumeClaimReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
		Log:       ctrl.Log.WithName("controllers").WithName("PersistentVolumeClaim"),
	}
	if err := pvccontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PersistentVolumeClaim")
		return err
	}

	// +kubebuilder:scaffold:builder

	// pre-cache objects
	if _, err := mgr.GetCache().GetInformer(&storagev1.StorageClass{}); err != nil {
		return err
	}
	if _, err := mgr.GetCache().GetInformer(&corev1.Pod{}); err != nil {
		return err
	}
	if _, err := mgr.GetCache().GetInformer(&corev1.PersistentVolumeClaim{}); err != nil {
		return err
	}

	// Add health checker to manager
	check := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var drv storagev1beta1.CSIDriver
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
	csi.RegisterControllerServer(grpcServer, driver.NewControllerService(s, n, config.defaultVG))

	// gRPC service itself should run even when the manager is *not* a leader
	// because CSI sidecar containers choose a leader.
	err = mgr.Add(runners.NewGRPCRunner(grpcServer, config.csiSocket, false))
	if err != nil {
		return err
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(stopCh); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}
	return nil
}
