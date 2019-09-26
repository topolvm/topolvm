package hook

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/cybozu-go/topolvm"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var pvcLogger = logf.Log.WithName("persistentvolumeclaim-mutator")

type persistentVolumeClaimMutator struct {
	client  client.Client
	decoder *admission.Decoder
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
	if err != nil {
		return admission.Allowed("cannot get StorageClass")
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
