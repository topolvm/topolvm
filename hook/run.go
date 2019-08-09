package hook

import (
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		panic(err)
	}

	// +kubebuilder:scaffold:scheme
}

// Run runs the webhook server.
func Run(cfg *rest.Config, webhookHost string, webhookPort int, metricsAddr, certDir string) error {
	ctrl.SetLogger(zap.Logger(false))

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
		LeaderElection:     false,
	})
	if err != nil {
		return err
	}

	// +kubebuilder:scaffold:builder

	// watch StorageClass objects
	if _, err := mgr.GetCache().GetInformer(&storagev1.StorageClass{}); err != nil {
		return err
	}

	wh := mgr.GetWebhookServer()
	wh.Host = webhookHost
	wh.Port = webhookPort
	wh.CertDir = certDir

	// NewDecoder never returns non-nil error
	dec, _ := admission.NewDecoder(scheme)
	wh.Register("/mutate", &webhook.Admission{Handler: podMutator{mgr.GetClient(), dec}})

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}
	return nil
}
