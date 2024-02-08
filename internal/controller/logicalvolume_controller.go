package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/topolvm/topolvm"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/internal/lvmd/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
)

// LogicalVolumeReconciler reconciles a LogicalVolume object
type LogicalVolumeReconciler struct {
	client    client.Client
	nodeName  string
	vgService proto.VGServiceClient
	lvService proto.LVServiceClient
}

//+kubebuilder:rbac:groups=topolvm.io,resources=logicalvolumes,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=topolvm.io,resources=logicalvolumes/status,verbs=get;update;patch

func NewLogicalVolumeReconcilerWithServices(client client.Client, nodeName string, vgService proto.VGServiceClient, lvService proto.LVServiceClient) *LogicalVolumeReconciler {
	return &LogicalVolumeReconciler{
		client:    client,
		nodeName:  nodeName,
		vgService: vgService,
		lvService: lvService,
	}
}

// Reconcile creates/deletes LVM logical volume for a LogicalVolume.
func (r *LogicalVolumeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)

	lv := new(topolvmv1.LogicalVolume)
	if err := r.client.Get(ctx, req.NamespacedName, lv); err != nil {
		if !apierrs.IsNotFound(err) {
			log.Error(err, "unable to fetch LogicalVolume")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	if lv.Spec.NodeName != r.nodeName {
		log.Info("unfiltered logical value", "nodeName", lv.Spec.NodeName)
		return ctrl.Result{}, nil
	}

	if lv.Annotations != nil {
		_, pendingDeletion := lv.Annotations[topolvm.GetLVPendingDeletionKey()]
		if pendingDeletion {
			if controllerutil.ContainsFinalizer(lv, topolvm.GetLogicalVolumeFinalizer()) {
				log.Error(nil, "logical volume was pending deletion but still has finalizer", "name", lv.Name)
			} else {
				log.Info("skipping finalizer for logical volume due to its pending deletion", "name", lv.Name)
			}
			return ctrl.Result{}, nil
		}
	}

	if lv.ObjectMeta.DeletionTimestamp == nil {
		if !controllerutil.ContainsFinalizer(lv, topolvm.GetLogicalVolumeFinalizer()) {
			lv2 := lv.DeepCopy()
			controllerutil.AddFinalizer(lv2, topolvm.GetLogicalVolumeFinalizer())
			patch := client.MergeFrom(lv)
			if err := r.client.Patch(ctx, lv2, patch); err != nil {
				log.Error(err, "failed to add finalizer", "name", lv.Name)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}

		if !containsKeyAndValue(lv.Labels, topolvm.CreatedbyLabelKey, topolvm.CreatedbyLabelValue) {
			lv2 := lv.DeepCopy()
			if lv2.Labels == nil {
				lv2.Labels = map[string]string{}
			}
			lv2.Labels[topolvm.CreatedbyLabelKey] = topolvm.CreatedbyLabelValue
			patch := client.MergeFrom(lv)
			if err := r.client.Patch(ctx, lv2, patch); err != nil {
				log.Error(err, "failed to add label", "name", lv.Name)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}

		if lv.Status.VolumeID == "" {
			err := r.createLV(ctx, log, lv)
			if err != nil {
				log.Error(err, "failed to create LV", "name", lv.Name)
			}
			return ctrl.Result{}, err
		}

		err := r.expandLV(ctx, log, lv)
		if err != nil {
			log.Error(err, "failed to expand LV", "name", lv.Name)
		}
		return ctrl.Result{}, err
	}

	// finalization
	if !controllerutil.ContainsFinalizer(lv, topolvm.GetLogicalVolumeFinalizer()) {
		// Our finalizer has finished, so the reconciler can do nothing.
		return ctrl.Result{}, nil
	}

	log.Info("start finalizing LogicalVolume", "name", lv.Name)
	err := r.removeLVIfExists(ctx, log, lv)
	if err != nil {
		return ctrl.Result{}, err
	}

	lv2 := lv.DeepCopy()
	controllerutil.RemoveFinalizer(lv2, topolvm.GetLogicalVolumeFinalizer())
	patch := client.MergeFrom(lv)
	if err := r.client.Patch(ctx, lv2, patch); err != nil {
		log.Error(err, "failed to remove finalizer", "name", lv.Name)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LogicalVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr)
	if topolvm.UseLegacy() {
		builder = builder.For(&topolvmlegacyv1.LogicalVolume{})
	} else {
		builder = builder.For(&topolvmv1.LogicalVolume{})
	}
	return builder.WithEventFilter(&logicalVolumeFilter{r.nodeName}).Complete(r)
}

func (r *LogicalVolumeReconciler) removeLVIfExists(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) error {
	// Finalizer's process ( RemoveLV then removeString ) is not atomic,
	// so checking existence of LV to ensure its idempotence
	_, err := r.lvService.RemoveLV(ctx, &proto.RemoveLVRequest{Name: string(lv.UID), DeviceClass: lv.Spec.DeviceClass})
	if status.Code(err) == codes.NotFound {
		log.Info("LV already removed", "name", lv.Name, "uid", lv.UID)
		return nil
	}
	if err != nil {
		log.Error(err, "failed to remove LV", "name", lv.Name, "uid", lv.UID)
		return err
	}
	log.Info("removed LV", "name", lv.Name, "uid", lv.UID)
	return nil
}

func (r *LogicalVolumeReconciler) volumeExists(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) (bool, error) {
	respList, err := r.vgService.GetLVList(ctx, &proto.GetLVListRequest{DeviceClass: lv.Spec.DeviceClass})
	if err != nil {
		log.Error(err, "failed to get list of LV")
		return false, err
	}

	for _, v := range respList.Volumes {
		if v.Name != string(lv.UID) {
			continue
		}
		return true, nil
	}
	return false, nil
}

func (r *LogicalVolumeReconciler) createLV(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) error {
	// When lv.Status.Code is not codes.OK (== 0), CreateLV has already failed.
	// LogicalVolume CRD will be deleted soon by the controller.
	if lv.Status.Code != codes.OK {
		return nil
	}

	reqBytes := lv.Spec.Size.Value()

	err := func() error {
		// In case the controller crashed just after LVM LV creation, LV may already exist.
		found, err := r.volumeExists(ctx, log, lv)
		if err != nil {
			lv.Status.Code = codes.Internal
			lv.Status.Message = "failed to check volume existence"
			return err
		}
		if found {
			log.Info("set volumeID to existing LogicalVolume", "name", lv.Name, "uid", lv.UID, "status.volumeID", lv.Status.VolumeID)
			// Don't set CurrentSize here because the Spec.Size field may be updated after the LVM LV is created.
			lv.Status.VolumeID = string(lv.UID)
			lv.Status.Code = codes.OK
			lv.Status.Message = ""
			return nil
		}

		var volume *proto.LogicalVolume

		// Create a snapshot LV
		if lv.Spec.Source != "" {
			// accessType should be either "readonly" or "readwrite".
			if lv.Spec.AccessType != "ro" && lv.Spec.AccessType != "rw" {
				return fmt.Errorf("invalid access type for source volume: %s", lv.Spec.AccessType)
			}
			sourcelv := new(topolvmv1.LogicalVolume)
			if err := r.client.Get(ctx, types.NamespacedName{Namespace: lv.Namespace, Name: lv.Spec.Source}, sourcelv); err != nil {
				log.Error(err, "unable to fetch source LogicalVolume", "name", lv.Name)
				return err
			}
			sourceVolID := sourcelv.Status.VolumeID

			// Create a snapshot lv
			resp, err := r.lvService.CreateLVSnapshot(ctx, &proto.CreateLVSnapshotRequest{
				Name:         string(lv.UID),
				DeviceClass:  lv.Spec.DeviceClass,
				SourceVolume: sourceVolID,
				// convert to uint64 because lvmd internals and lvm use uint64 but CSI uses int64.
				// still set sizeGB for legacy purposes, can (but not has to) be removed in next minor release.
				SizeGb:     uint64(reqBytes >> 30),
				SizeBytes:  reqBytes,
				AccessType: lv.Spec.AccessType,
			})
			if err != nil {
				code, message := extractFromError(err)
				log.Error(err, message)
				lv.Status.Code = code
				lv.Status.Message = message
				return err
			}
			volume = resp.Snapshot
		} else {
			// Create a regular lv
			resp, err := r.lvService.CreateLV(ctx, &proto.CreateLVRequest{
				Name:                string(lv.UID),
				DeviceClass:         lv.Spec.DeviceClass,
				LvcreateOptionClass: lv.Spec.LvcreateOptionClass,
				// convert to uint64 because lvmd internals and lvm use uint64 but CSI uses int64.
				// still set sizeGB for legacy purposes, can (but not has to) be removed in next minor release.
				SizeGb:    uint64(reqBytes >> 30),
				SizeBytes: reqBytes,
			})
			if err != nil {
				code, message := extractFromError(err)
				log.Error(err, message)
				lv.Status.Code = code
				lv.Status.Message = message
				return err
			}
			volume = resp.Volume
		}

		lv.Status.VolumeID = volume.Name
		lv.Status.CurrentSize = resource.NewQuantity(reqBytes, resource.BinarySI)
		lv.Status.Code = codes.OK
		lv.Status.Message = ""
		return nil
	}()

	if err != nil {
		if err2 := r.client.Status().Update(ctx, lv); err2 != nil {
			// err2 is logged but not returned because err is more important
			log.Error(err2, "failed to update status", "name", lv.Name, "uid", lv.UID)
		}
		return err
	}

	if err := r.client.Status().Update(ctx, lv); err != nil {
		log.Error(err, "failed to update status", "name", lv.Name, "uid", lv.UID)
		return err
	}

	log.Info("created new LV", "name", lv.Name, "uid", lv.UID, "status.volumeID", lv.Status.VolumeID)
	return nil
}

func (r *LogicalVolumeReconciler) expandLV(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) error {
	// We denote unknown size as -1.
	var origBytes int64 = -1
	switch {
	case lv.Status.CurrentSize == nil:
		// topolvm-node may be crashed before setting Status.CurrentSize.
		// Since the actual volume size is unknown,
		// we need to do resizing to set Status.CurrentSize to the same value as Spec.Size.
	case lv.Spec.Size.Cmp(*lv.Status.CurrentSize) <= 0:
		return nil
	default:
		origBytes = (*lv.Status.CurrentSize).Value()
	}

	reqBytes := lv.Spec.Size.Value()

	err := func() error {
		_, err := r.lvService.ResizeLV(ctx, &proto.ResizeLVRequest{
			Name: string(lv.UID),
			// convert to uint64 because lvmd internals and lvm use uint64 but CSI uses int64.
			// still set sizeGB for legacy purposes, can (but not has to) be removed in next minor release.
			SizeGb:      uint64(reqBytes >> 30),
			SizeBytes:   reqBytes,
			DeviceClass: lv.Spec.DeviceClass,
		})
		if err != nil {
			code, message := extractFromError(err)
			log.Error(err, message)
			lv.Status.Code = code
			lv.Status.Message = message
			return err
		}

		lv.Status.CurrentSize = resource.NewQuantity(reqBytes, resource.BinarySI)
		lv.Status.Code = codes.OK
		lv.Status.Message = ""
		return nil
	}()

	if err != nil {
		if err2 := r.client.Status().Update(ctx, lv); err2 != nil {
			// err2 is logged but not returned because err is more important
			log.Error(err2, "failed to update status", "name", lv.Name, "uid", lv.UID)
		}
		return err
	}

	if err := r.client.Status().Update(ctx, lv); err != nil {
		log.Error(err, "failed to update status", "name", lv.Name, "uid", lv.UID)
		return err
	}

	log.Info("expanded LV", "name", lv.Name, "uid", lv.UID, "status.volumeID", lv.Status.VolumeID,
		"original status.currentSize", origBytes, "status.currentSize", reqBytes)
	return nil
}

type logicalVolumeFilter struct {
	nodeName string
}

func (f logicalVolumeFilter) filter(obj client.Object) bool {
	var name string
	if topolvm.UseLegacy() {
		lv, ok := obj.(*topolvmlegacyv1.LogicalVolume)
		if !ok {
			return false
		}
		name = lv.Spec.NodeName
	} else {
		lv, ok := obj.(*topolvmv1.LogicalVolume)
		if !ok {
			return false
		}
		name = lv.Spec.NodeName
	}
	if name == f.nodeName {
		return true
	}
	return false
}

func (f logicalVolumeFilter) Create(e event.CreateEvent) bool {
	return f.filter(e.Object)
}

func (f logicalVolumeFilter) Delete(e event.DeleteEvent) bool {
	return f.filter(e.Object)
}

func (f logicalVolumeFilter) Update(e event.UpdateEvent) bool {
	return f.filter(e.ObjectNew)
}

func (f logicalVolumeFilter) Generic(e event.GenericEvent) bool {
	return f.filter(e.Object)
}

func extractFromError(err error) (codes.Code, string) {
	s, ok := status.FromError(err)
	if !ok {
		return codes.Internal, err.Error()
	}
	return s.Code(), s.Message()
}

func containsKeyAndValue(labels map[string]string, key, value string) bool {
	for k, v := range labels {
		if k == key && v == value {
			return true
		}
	}
	return false
}
