package hook

import (
	"context"
	"encoding/json"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cybozu-go/topolvm"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var pvcLogger = logf.Log.WithName("persistentvolumeclaim-mutator")

type persistentVolumeClaimMutator struct {
	client  client.Client
	decoder *admission.Decoder
}

// +kubebuilder:webhook:path=/pvc/mutate,mutating=true,failurePolicy=fail,groups="",resources=persistentvolumeclaims,verbs=patch,versions=v1,name=pvc-hook.topolvm.cybozu.com
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// persistentVolumeClaimMutator has admission.Handler checked by compiler
var _ admission.Handler = &persistentVolumeClaimMutator{}

// Handle implements admission.Handler interface.
func (m persistentVolumeClaimMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	pvc := &corev1.PersistentVolumeClaim{}
	err := m.decoder.Decode(req, pvc)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	targets, err := m.targetStorageClasses(ctx)
	if err != nil {
		pvcLogger.Error(err, "fetching targetStorageClasses failed")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// StorageClassName can be nil
	if pvc.Spec.StorageClassName == nil {
		return admission.Allowed("no request for TopoLVM")
	}

	if !targets[*pvc.Spec.StorageClassName] {
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

func (m persistentVolumeClaimMutator) targetStorageClasses(ctx context.Context) (map[string]bool, error) {
	var scl storagev1.StorageClassList
	if err := m.client.List(ctx, &scl); err != nil {
		return nil, err
	}

	targets := make(map[string]bool)
	for _, sc := range scl.Items {
		if sc.Provisioner != topolvm.PluginName {
			targets[sc.Name] = false
		} else {
			targets[sc.Name] = true
		}
	}
	return targets, nil
}
