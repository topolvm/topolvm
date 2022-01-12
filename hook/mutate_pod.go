package hook

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/topolvm/topolvm"
	"github.com/topolvm/topolvm/getter"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var pmLogger = ctrl.Log.WithName("pod-mutator")

//+kubebuilder:webhook:failurePolicy=fail,matchPolicy=equivalent,groups=core,resources=pods,verbs=create,versions=v1,name=pod-hook.topolvm.cybozu.com,path=/pod/mutate,mutating=true,sideEffects=none,admissionReviewVersions={v1,v1beta1}
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// podMutator mutates pods using PVC for TopoLVM.
type podMutator struct {
	getter  *getter.RetryMissingGetter
	decoder *admission.Decoder
}

// PodMutator creates a mutating webhook for Pods.
func PodMutator(r client.Reader, apiReader client.Reader, dec *admission.Decoder) http.Handler {
	return &webhook.Admission{
		Handler: &podMutator{
			getter:  getter.NewRetryMissingGetter(r, apiReader),
			decoder: dec,
		},
	}
}

// Handle implements admission.Handler interface.
func (m *podMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := m.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	if len(pod.Spec.Containers) == 0 {
		return admission.Denied("pod has no containers")
	}

	// short cut
	if len(pod.Spec.Volumes) == 0 {
		return admission.Allowed("no volumes")
	}

	// Pods instantiated from templates may have empty name/namespace.
	// To lookup PVC in the same namespace, we set namespace obtained from req.
	if pod.Namespace == "" {
		pmLogger.Info("infer pod namespace from req", "namespace", req.Namespace)
		pod.Namespace = req.Namespace
	}

	pvcCapacities, err := m.requestedPVCCapacity(ctx, pod)
	if err != nil {
		pmLogger.Error(err, "requestedPVCCapacity failed")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	ephemeralCapacity, err := m.requestedEphemeralCapacity(pod)
	if err != nil {
		pmLogger.Error(err, "requestedEphemeralCapacity failed")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if ephemeralCapacity != 0 {
		if pvcCapacities == nil {
			pvcCapacities = make(map[string]int64)
		}
		pvcCapacities[topolvm.DefaultDeviceClassAnnotationName] += ephemeralCapacity
	}

	if len(pvcCapacities) == 0 {
		return admission.Allowed("no request for TopoLVM")
	}

	ctnr := &pod.Spec.Containers[0]
	quantity := resource.NewQuantity(1, resource.DecimalSI)
	if ctnr.Resources.Requests == nil {
		ctnr.Resources.Requests = corev1.ResourceList{}
	}
	ctnr.Resources.Requests[topolvm.CapacityResource] = *quantity
	if ctnr.Resources.Limits == nil {
		ctnr.Resources.Limits = corev1.ResourceList{}
	}
	ctnr.Resources.Limits[topolvm.CapacityResource] = *quantity

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	for dc, capacity := range pvcCapacities {
		pod.Annotations[topolvm.CapacityKeyPrefix+dc] = strconv.FormatInt(capacity, 10)
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

type targetSC struct {
	getter *getter.RetryMissingGetter
	cache  map[string]*storagev1.StorageClass
}

func (t *targetSC) Get(ctx context.Context, name string) (*storagev1.StorageClass, error) {
	if sc, ok := t.cache[name]; ok {
		return sc, nil
	}

	var sc storagev1.StorageClass
	err := t.getter.Get(ctx, types.NamespacedName{Name: name}, &sc)
	if err != nil {
		if apierrs.IsNotFound(err) {
			t.cache[name] = nil
			return nil, nil
		}
		return nil, err
	}
	if sc.Provisioner != topolvm.PluginName {
		t.cache[name] = nil
		return nil, nil
	}
	t.cache[name] = &sc
	return &sc, nil
}

func (m *podMutator) requestedPVCCapacity(ctx context.Context, pod *corev1.Pod) (map[string]int64, error) {
	targetSC := targetSC{m.getter, map[string]*storagev1.StorageClass{}}
	capacities := make(map[string]int64)
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim == nil {
			// CSI volume type does not support direct reference from Pod
			// and may only be referenced in a Pod via a PersistentVolumeClaim
			// https://kubernetes.io/docs/concepts/storage/volumes/#csi
			continue
		}
		pvcName := vol.PersistentVolumeClaim.ClaimName
		name := types.NamespacedName{
			Namespace: pod.Namespace,
			Name:      pvcName,
		}

		var pvc corev1.PersistentVolumeClaim
		if err := m.getter.Get(ctx, name, &pvc); err != nil {
			if !apierrs.IsNotFound(err) {
				pmLogger.Error(err, "failed to get pvc",
					"pod", pod.Name,
					"namespace", pod.Namespace,
					"pvc", pvcName,
				)
				return nil, err
			}
			// Pods should be created even if their PVCs do not exist yet.
			// TopoLVM does not care about such pods after they are created, though.
			continue
		}

		if pvc.Spec.StorageClassName == nil {
			// empty class name may appear when DefaultStorageClass admission plugin
			// is turned off, or there are no default StorageClass.
			// https://kubernetes.io/docs/concepts/storage/persistent-volumes/#class-1
			continue
		}
		sc, err := targetSC.Get(ctx, *pvc.Spec.StorageClassName)
		if err != nil {
			return nil, err
		}
		if sc == nil {
			continue
		}

		// If the Pod has a bound PVC of TopoLVM, the pod will be scheduled
		// to the node of the existing PV.
		if pvc.Status.Phase != corev1.ClaimPending {
			return nil, nil
		}

		var requested int64 = topolvm.DefaultSize
		if req, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			if req.Value() > topolvm.DefaultSize {
				requested = ((req.Value()-1)>>30 + 1) << 30
			}
		}
		dc, ok := sc.Parameters[topolvm.DeviceClassKey]
		if !ok {
			dc = topolvm.DefaultDeviceClassAnnotationName
		}

		capacities[dc] += requested
	}
	return capacities, nil
}

func (m *podMutator) requestedEphemeralCapacity(pod *corev1.Pod) (int64, error) {
	var total int64
	for _, vol := range pod.Spec.Volumes {
		if vol.CSI == nil {
			// We only want to look at CSI volumes
			continue
		}
		if vol.CSI.Driver == topolvm.PluginName {
			if volSizeStr, ok := vol.CSI.VolumeAttributes[topolvm.EphemeralVolumeSizeKey]; ok {
				volSize, err := strconv.ParseInt(volSizeStr, 10, 64)
				if err != nil {
					pmLogger.Error(err, "Invalid volume size",
						topolvm.EphemeralVolumeSizeKey, volSizeStr,
					)
					return 0, err
				}
				total += volSize << 30
			} else {
				total += topolvm.DefaultSize
			}
		}
	}
	return total, nil
}
