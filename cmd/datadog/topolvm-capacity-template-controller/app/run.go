package app

import (
	"fmt"

	"github.com/topolvm/topolvm/internal/datadog/capacitytemplate"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func run(opts Options) error {
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts.Zap)))
	setupLog := ctrl.Log.WithName("setup").WithName("datadog-capacity-template")

	controllerConfig, err := loadControllerConfig(opts.ConfigFile)
	if err != nil {
		return err
	}

	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("get Kubernetes configuration: %w", err)
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	capacitytemplate.AddNodeGroupToScheme(scheme)

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme: scheme,
		Cache: cache.Options{
			SyncPeriod: &opts.SyncPeriod,
		},
		Metrics: metricsserver.Options{
			BindAddress: opts.MetricsBindAddress,
		},
		HealthProbeBindAddress:  opts.HealthProbeBindAddress,
		LeaderElection:          opts.LeaderElect,
		LeaderElectionID:        opts.LeaderElectionID,
		LeaderElectionNamespace: opts.LeaderElectionNamespace,
	})
	if err != nil {
		return fmt.Errorf("create manager: %w", err)
	}

	reconciler, err := capacitytemplate.NewReconciler(
		mgr.GetClient(),
		mgr.GetEventRecorder("datadog-topolvm-capacity-template-controller"),
		controllerConfig,
	)
	if err != nil {
		return fmt.Errorf("create capacity controller: %w", err)
	}
	if err := reconciler.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("set up capacity controller: %w", err)
	}
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return err
	}

	for i, template := range controllerConfig.Templates {
		setupLog.Info("configured capacity template",
			"index", i,
			"storageClass", template.StorageClass,
			"deviceClass", template.DeviceClass,
			"spareGB", template.SpareGB,
			"handler", template.Handler)
	}
	setupLog.Info("starting manager",
		"config", opts.ConfigFile,
		"templates", len(controllerConfig.Templates),
		"syncPeriod", opts.SyncPeriod)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("run manager: %w", err)
	}
	return nil
}
