package hook

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/cybozu-go/topolvm"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var pvcLogger = logf.Log.WithName("persistentvolumeclaim-mutator")

type persistentVolumeClaimMutator struct {
	client  client.Client
	decoder *admission.Decoder
}

// PVCMutator creates a mutating webhook for PVCs.
func PVCMutator(c client.Client, dec *admission.Decoder) http.Handler {
	return &webhook.Admission{Handler: persistentVolumeClaimMutator{c, dec}}
}

// +kubebuilder:webhook:path=/pvc/mutate,mutating=true,failurePolicy=fail,groups="",resources=persistentvolumeclaims,verbs=create,versions=v1,name=pvc-hook.topolvm.cybozu.com
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// Handle implements admission.Handler interface.
func (m persistentVolumeClaimMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
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
	err = m.client.Get(ctx, types.NamespacedName{Name: *pvc.Spec.StorageClassName}, &sc)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		// StorageClass appeared in the manifest is not exists
		return admission.Errored(http.StatusBadRequest, err)
	default:
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if sc.Provisioner != topolvm.PluginName {
		return admission.Allowed("no request for TopoLVM")
	}

	pvcPatch := pvc.DeepCopy()
	pvcPatch.Finalizers = append(pvcPatch.Finalizers, topolvm.PVCFinalizer)
	marshaled, err := json.Marshal(pvcPatch)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
}
