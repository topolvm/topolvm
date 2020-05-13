package hook

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/cybozu-go/topolvm"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var pmLogger = logf.Log.WithName("pod-mutator")

// +kubebuilder:webhook:path=/pod/mutate,mutating=true,failurePolicy=fail,matchPolicy=equivalent,groups="",resources=pods,verbs=create,versions=v1,name=pod-hook.topolvm.cybozu.com
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// podMutator mutates pods using PVC for TopoLVM.
type podMutator struct {
	client    client.Client
	decoder   *admission.Decoder
	defaultVG string
}

// PodMutator creates a mutating webhook for Pods.
func PodMutator(c client.Client, dec *admission.Decoder, defaultVG string) http.Handler {
	return &webhook.Admission{Handler: podMutator{c, dec, defaultVG}}
}

// Handle implements admission.Handler interface.
func (m podMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
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

	targets, err := m.targetStorageClasses(ctx)
	if err != nil {
		pmLogger.Error(err, "targetStorageClasses failed")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	pvcCapacities, err := m.requestedPVCCapacity(ctx, pod, targets)
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
		if v, ok := pvcCapacities[m.defaultVG]; ok {
			pvcCapacities[m.defaultVG] = v + ephemeralCapacity
		} else {
			pvcCapacities[m.defaultVG] = ephemeralCapacity
		}
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

	pod.Annotations = make(map[string]string)
	for vg, capacity := range pvcCapacities {
		pod.Annotations[topolvm.CapacityKey+vg] = strconv.FormatInt(capacity, 10)
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

func (m podMutator) targetStorageClasses(ctx context.Context) (map[string]storagev1.StorageClass, error) {
	var scl storagev1.StorageClassList
	if err := m.client.List(ctx, &scl); err != nil {
		return nil, err
	}

	targets := make(map[string]storagev1.StorageClass)
	for _, sc := range scl.Items {
		if sc.Provisioner != topolvm.PluginName {
			continue
		}
		targets[sc.Name] = sc
	}
	return targets, nil
}

func (m podMutator) requestedPVCCapacity(ctx context.Context, pod *corev1.Pod, targets map[string]storagev1.StorageClass) (map[string]int64, error) {
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
		if err := m.client.Get(ctx, name, &pvc); err != nil {
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
		sc, ok := targets[*pvc.Spec.StorageClassName]
		if !ok {
			continue
		}

		// If the Pod has a bound PVC of TopoLVM, the pod will be scheduled
		// to the node of the existing PV.
		if pvc.Status.Phase != corev1.ClaimPending {
			return make(map[string]int64), nil
		}

		var requested int64 = topolvm.DefaultSize
		if req, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			if req.Value() > topolvm.DefaultSize {
				requested = ((req.Value()-1)>>30 + 1) << 30
			}
		}
		vgName, ok := sc.Parameters[topolvm.VolumeGroupKey]
		if !ok {
			vgName = m.defaultVG
		}
		total, ok := capacities[vgName]
		if !ok {
			total = 0
		}
		total += requested
		capacities[vgName] = total
	}
	return capacities, nil
}

func (m podMutator) requestedEphemeralCapacity(pod *corev1.Pod) (int64, error) {
	var total int64
	for _, vol := range pod.Spec.Volumes {
		if vol.CSI == nil {
			// We only want to look at CSI volumes
			continue
		}
		if vol.CSI.Driver == topolvm.PluginName {
			if volSizeStr, ok := vol.CSI.VolumeAttributes[topolvm.SizeVolConKey]; ok {
				volSize, err := strconv.ParseInt(volSizeStr, 10, 64)
				if err != nil {
					pmLogger.Error(err, "Invalid volume size",
						topolvm.SizeVolConKey, volSizeStr,
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
