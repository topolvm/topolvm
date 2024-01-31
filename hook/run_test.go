package hook

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	// +kubebuilder:scaffold:imports
)

func run(ctx context.Context, cfg *rest.Config, scheme *runtime.Scheme, opts *envtest.WebhookInstallOptions) error {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "localhost:8999",
		},
		LeaderElection: false,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    opts.LocalServingHost,
			Port:    opts.LocalServingPort,
			CertDir: opts.LocalServingCertDir,
		}),
	})
	if err != nil {
		return err
	}

	// +kubebuilder:scaffold:builder

	dec := admission.NewDecoder(scheme)
	wh := mgr.GetWebhookServer()
	wh.Register(podMutatingWebhookPath, PodMutator(mgr.GetClient(), mgr.GetAPIReader(), dec))
	wh.Register(pvcMutatingWebhookPath, PVCMutator(mgr.GetClient(), mgr.GetAPIReader(), dec))

	if err := mgr.Start(ctx); err != nil {
		return err
	}
	return nil
}
