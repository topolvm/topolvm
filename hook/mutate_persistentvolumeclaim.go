package hook

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/topolvm/topolvm"
	"github.com/topolvm/topolvm/getter"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type persistentVolumeClaimMutator struct {
	getter  *getter.RetryMissingGetter
	decoder *admission.Decoder
}

// PVCMutator creates a mutating webhook for PVCs.
func PVCMutator(r client.Reader, apiReader client.Reader, dec *admission.Decoder) http.Handler {
	return &webhook.Admission{
		Handler: &persistentVolumeClaimMutator{
			getter: getter.NewRetryMissingGetter(r, apiReader),
			decoder: dec,
		},
	}
}

//+kubebuilder:webhook:failurePolicy=fail,matchPolicy=equivalent,groups=core,resources=persistentvolumeclaims,verbs=create,versions=v1,name=pvc-hook.topolvm.cybozu.com,path=/pvc/mutate,mutating=true,sideEffects=none,admissionReviewVersions={v1,v1beta1}
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// Handle implements admission.Handler interface.
func (m *persistentVolumeClaimMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	pvc := &corev1.PersistentVolumeClaim{}
	err := m.decoder.Decode(req, pvc)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// StorageClassName can be nil
	if pvc.Spec.StorageClassName == nil {
		return admission.Allowed("no request for TopoLVM")
	}

	var sc storagev1.StorageClass
	err = m.getter.Get(ctx, types.NamespacedName{Name: *pvc.Spec.StorageClassName}, &sc)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		// StorageClassName can be simple name linked PV
		return admission.Allowed("no request for TopoLVM")
	default:
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if sc.Provisioner != topolvm.PluginName {
		return admission.Allowed("no request for TopoLVM")
	}

	if controllerutil.ContainsFinalizer(pvc, topolvm.PVCFinalizer) {
		return admission.Allowed("already added finalizer")
	}

	pvcPatch := pvc.DeepCopy()
	pvcPatch.Finalizers = append(pvcPatch.Finalizers, topolvm.PVCFinalizer)
	marshaled, err := json.Marshal(pvcPatch)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
}
