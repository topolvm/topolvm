package hook

import (
	"path/filepath"

	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	// +kubebuilder:scaffold:imports
)

func run(stopCh <-chan struct{}, cfg *rest.Config, scheme *runtime.Scheme, webhookHost string, webhookPort int) error {
	ctrl.SetLogger(zap.Logger(true))

	certDir, err := filepath.Abs("./testdata")
	if err != nil {
		return err
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "localhost:8999",
		LeaderElection:     false,
		Host:               webhookHost,
		Port:               webhookPort,
		CertDir:            certDir,
	})
	if err != nil {
		return err
	}

	// +kubebuilder:scaffold:builder

	// watch StorageClass objects
	if _, err := mgr.GetCache().GetInformer(&storagev1.StorageClass{}); err != nil {
		return err
	}

	dec, _ := admission.NewDecoder(scheme)
	wh := mgr.GetWebhookServer()
	wh.Register("/pod/mutate", &webhook.Admission{Handler: podMutator{mgr.GetClient(), dec}})
	wh.Register("/pvc/mutate", &webhook.Admission{Handler: persistentVolumeClaimMutator{mgr.GetClient(), dec}})

	if err := mgr.Start(stopCh); err != nil {
		return err
	}
	return nil
}
