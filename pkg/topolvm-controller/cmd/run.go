package cmd

import (
	"fmt"
	"net"
	"os"
	"time"

	topolvmv1 "github.com/cybozu-go/topolvm/api/v1"
	"github.com/cybozu-go/topolvm/controllers"
	"github.com/cybozu-go/topolvm/hook"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

// Run builds and starts the manager includes the controllers.
func subMain() error {
	ctrl.SetLogger(zap.Logger(config.development))

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

	lvcontroller := &controllers.LogicalVolumeReconciler{
		Client:      mgr.GetClient(),
		Log:         ctrl.Log.WithName("controllers").WithName("LogicalVolume"),
		Events:      events,
		StalePeriod: config.stalePeriod,
	}
	if err := lvcontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LogicalVolume")
		os.Exit(1)
	}

	nodecontroller := &controllers.NodeReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Node"),
	}
	if err := nodecontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Node")
		os.Exit(1)
	}

	pvccontroller := &controllers.PersistentVolumeClaimReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("PersistentVolumeClaim"),
	}
	if err := pvccontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PersistentVolumeClaim")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	err = mgr.GetFieldIndexer().IndexField(&corev1.PersistentVolumeClaim{}, controllers.KeySelectedNode, func(o runtime.Object) []string {
		return []string{o.(*corev1.PersistentVolumeClaim).Annotations[controllers.AnnSelectedNode]}
	})
	if err != nil {
		return err
	}
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

	setupLog.Info("starting manager")
	if err := mgr.Start(stopCh); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}
	return nil
}
