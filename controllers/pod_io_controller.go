package controllers

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/topolvm/topolvm"
	"github.com/topolvm/topolvm/iolimit"
	"k8s.io/kubectl/pkg/util/qos"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
)

// PodReconciler reconciles a Node object
type PodIOReconciler struct {
	client.Client
	nodeName string
	ioCache  sync.Map
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;delete

func NewPodIOReconciler(
	client client.Client,
	nodeName string,
) *PodIOReconciler {
	return &PodIOReconciler{
		Client:   client,
		nodeName: nodeName,
		ioCache:  sync.Map{},
	}
}

// Reconcile finalize Node
func (r *PodIOReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)
	pod := &corev1.Pod{}
	if err := r.Client.Get(ctx, req.NamespacedName, pod); err != nil {
		log.Error(err, " unable to fetch pod")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if pod.DeletionTimestamp != nil {
		r.ioCache.Delete(pod.UID)
		return ctrl.Result{}, nil
	}

	log.Info(fmt.Sprintf("Try to update pod's cgroup blkio, pod namespace: %s, pod name: %s", pod.Namespace, pod.Name))

	if err := r.handleSinglePodCGroupConfig(ctx, pod); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up Reconciler with Manager.
func (r *PodIOReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()
	err := mgr.GetFieldIndexer().IndexField(ctx, &corev1.Pod{}, "combinedIndex", func(object client.Object) []string {
		return []string{object.(*corev1.Pod).Spec.NodeName}
	})
	if err != nil {
		return err
	}

	err = mgr.GetFieldIndexer().IndexField(ctx, &corev1.PersistentVolume{}, "pvIndex", func(object client.Object) []string {
		pv := object.(*corev1.PersistentVolume)
		if pv == nil {
			return nil
		}
		if pv.Spec.CSI == nil {
			return nil
		}
		if pv.Spec.CSI.Driver != topolvm.PluginName {
			return nil
		}
		if pv.Status.Phase != corev1.VolumeBound {
			return nil
		}
		if pv.Spec.ClaimRef == nil {
			return nil
		}
		return []string{fmt.Sprintf("%s-%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)}
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(podFilter{r.nodeName}).
		WithOptions(controller.Options{
			RateLimiter:             workqueue.NewItemFastSlowRateLimiter(10*time.Second, 60*time.Second, 5),
			MaxConcurrentReconciles: 5,
		}).
		For(&corev1.Pod{}).
		Complete(r)
}

func (r *PodIOReconciler) handleSinglePodCGroupConfig(ctx context.Context, pod *corev1.Pod) error {
	log := crlog.FromContext(ctx)
	newPodIOLimit := r.getPodIOLimit(pod)
	oldPodIOLimit, ok := r.ioCache.Load(pod.UID)
	if ok && newPodIOLimit.Equal(oldPodIOLimit.(*iolimit.IOLimit)) {
		log.Info(fmt.Sprintf("Pod's io throttles hasn't changed, ignore it, namespace: %s, name: %s", pod.Namespace, pod.Name))
		return nil
	}

	log.Info(fmt.Sprintf("Need to update pod's cgroup blkio, namespace: %s, name: %s", pod.Namespace, pod.Name))
	if err := iolimit.SetIOLimit(r.getPodBlkIO(ctx, pod)); err != nil {
		return err
	}
	r.ioCache.Store(pod.UID, newPodIOLimit)
	return nil
}

func (r *PodIOReconciler) getPodBlkIO(ctx context.Context, pod *corev1.Pod) *iolimit.PodBlkIO {
	log := crlog.FromContext(ctx)
	if pod == nil {
		return &iolimit.PodBlkIO{}
	}
	deviceIOSet := iolimit.DeviceIOSet{}
	iolt := r.getPodIOLimit(pod)
	for _, volume := range pod.Spec.Volumes {
		if volume.VolumeSource.PersistentVolumeClaim == nil {
			continue
		}
		pvList := &corev1.PersistentVolumeList{}
		err := r.Client.List(ctx, pvList, client.MatchingFields{"pvIndex": fmt.Sprintf("%s-%s", pod.GetNamespace(), volume.VolumeSource.PersistentVolumeClaim.ClaimName)})
		if err != nil {
			log.Error(err, "Failed to get pv", volume.VolumeSource.PersistentVolumeClaim.ClaimName)
			continue
		}

		if len(pvList.Items) != 1 {
			log.Info("Get pv count not equal one", len(pvList.Items))
			continue
		}
		pvInfo := pvList.Items[0]
		deviceMajor := pvInfo.Spec.CSI.VolumeAttributes[topolvm.DeviceMajorKey]
		deviceMinor := pvInfo.Spec.CSI.VolumeAttributes[topolvm.DeviceMinorKey]

		if deviceMajor == "" || deviceMinor == "" {
			continue
		}
		deviceNo := fmt.Sprintf("%s:%s", deviceMajor, deviceMinor)

		deviceIOSet[deviceNo] = iolt
	}
	return &iolimit.PodBlkIO{
		PodUid:      string(pod.UID),
		PodQos:      qos.GetPodQOS(pod),
		DeviceIOSet: deviceIOSet,
	}
}

func (r *PodIOReconciler) getPodIOLimit(pod *corev1.Pod) *iolimit.IOLimit {
	if pod == nil {
		return &iolimit.IOLimit{}
	}
	iolt := &iolimit.IOLimit{}
	for _, throttle := range iolimit.GetSupportedIOThrottles() {
		var throttleVal uint64
		var err error
		newValue, ok := pod.Annotations[fmt.Sprintf("%s/%s", topolvm.PluginName, throttle)]
		if ok {
			if throttleVal, err = strconv.ParseUint(newValue, 10, 64); err != nil {
				fmt.Printf("Failed to parse %s， will use default value 0", newValue)
			}
		}
		switch throttle {
		case iolimit.BlkIOThrottleReadBPS:
			iolt.Rbps = throttleVal
		case iolimit.BlkIOThrottleReadIOPS:
			iolt.Riops = throttleVal
		case iolimit.BlkIOThrottleWriteBPS:
			iolt.Wbps = throttleVal
		case iolimit.BlkIOThrottleWriteIOPS:
			iolt.Wiops = throttleVal
		default:
			fmt.Printf("Unsupported throttle type %s", throttle)
		}
	}
	return iolt
}

// filter carina pod
type podFilter struct {
	nodeName string
}

func (p podFilter) filter(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	if pod.Spec.NodeName != p.nodeName {
		return false
	}
	// check static pod
	if pod.Annotations != nil {
		if source, ok := pod.Annotations[topolvm.ConfigSourceKey]; ok {
			if source != topolvm.ApiServerSource {
				return false
			}
		}
	}
	if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodSucceeded {
		return false
	}
	var ioThrottleExist bool
	for _, ioThrottle := range iolimit.GetSupportedIOThrottles() {
		if _, ok := pod.Annotations[fmt.Sprintf("%s/%s", topolvm.PluginName, ioThrottle)]; ok {
			ioThrottleExist = true
			break
		}
	}
	return ioThrottleExist
}

func (p podFilter) Create(e event.CreateEvent) bool {
	return p.filter(e.Object.(*corev1.Pod))
}

func (p podFilter) Delete(e event.DeleteEvent) bool {
	return p.filter(e.Object.(*corev1.Pod))
}

func (p podFilter) Update(e event.UpdateEvent) bool {
	newPod := e.ObjectNew.(*corev1.Pod)
	oldPod := e.ObjectOld.(*corev1.Pod)
	if newPod.ResourceVersion == oldPod.ResourceVersion {
		return false
	}
	return p.filter(newPod) || p.filter(oldPod)
}

func (p podFilter) Generic(e event.GenericEvent) bool {
	return false
}
