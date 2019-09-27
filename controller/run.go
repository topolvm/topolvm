package controller

import (
	"os"
	"time"

	"github.com/cybozu-go/topolvm/controller/controllers"
	logicalvolumev1 "github.com/cybozu-go/topolvm/topolvm-node/api/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	if err := logicalvolumev1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		panic(err)
	}

	// +kubebuilder:scaffold:scheme
}

// Run builds and starts the manager includes the controllers.
func Run(cfg *rest.Config, metricsAddr string, stalePeriod time.Duration, cleanupInterval time.Duration, development bool) error {
	ctrl.SetLogger(zap.Logger(development))

	if cfg == nil {
		c, err := ctrl.GetConfig()
		if err != nil {
			return err
		}
		cfg = c
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     true,
	})
	if err != nil {
		return err
	}

	events := make(chan event.GenericEvent, 1)
	stopCh := ctrl.SetupSignalHandler()
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		for {
			select {
			case <-stopCh:
				ticker.Stop()
				return
			case <-ticker.C:
				select {
				case events <- event.GenericEvent{
					Meta: &logicalvolumev1.LogicalVolume{},
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
		StalePeriod: stalePeriod,
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
