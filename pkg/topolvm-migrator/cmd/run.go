package cmd

import (
	"context"
	"errors"

	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/migrator/controllers"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(topolvmlegacyv1.AddToScheme(scheme))
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

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     config.metricsAddr,
		HealthProbeBindAddress: config.healthAddr,
		LeaderElection:         true,
		LeaderElectionID:       config.leaderElectionID,
		CertDir:                config.certDir,
	})
	if err != nil {
		return err
	}

	ctx := context.TODO()

	// register controllers
	pvccontroller := &controllers.PersistentVolumeClaimReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
	}
	if err := pvccontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PersistentVolumeClaim")
		return err
	}
	go func() {
		if ok := mgr.GetCache().WaitForCacheSync(ctx); !ok {
			err := errors.New("failed to sync cache")
			setupLog.Error(err, "failed to sync cache")
		}

		if err := pvccontroller.RunOnce(ctx); err != nil {
			setupLog.Error(err, "unable to run once controller", "controller", "PersistentVolumeClaim")
		}
	}()

	nodecontroller := &controllers.NodeReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
	}
	if err := nodecontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Node")
		return err
	}
	go func() {
		if ok := mgr.GetCache().WaitForCacheSync(ctx); !ok {
			err := errors.New("failed to sync cache")
			setupLog.Error(err, "failed to sync cache")
		}

		if err := nodecontroller.RunOnce(ctx); err != nil {
			setupLog.Error(err, "unable to run once controller", "controller", "Node")
		}
	}()

	logicalvolumecontroller := &controllers.LogicalVolumeReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
	}
	if err := logicalvolumecontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Node")
		return err
	}
	go func() {
		if ok := mgr.GetCache().WaitForCacheSync(ctx); !ok {
			err := errors.New("failed to sync cache")
			setupLog.Error(err, "failed to sync cache")
		}

		if err := logicalvolumecontroller.RunOnce(ctx); err != nil {
			setupLog.Error(err, "unable to run once controller", "controller", "LogicalVolume")
		}
	}()

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return err
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}
	return nil
}
