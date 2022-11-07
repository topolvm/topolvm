package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/topolvm/topolvm"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/lvmd/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// NewLogicalVolumeReconciler returns LogicalVolumeReconciler with creating lvService and vgService.
func NewLogicalVolumeReconciler(client client.Client, nodeName string, conn *grpc.ClientConn) *LogicalVolumeReconciler {
	return &LogicalVolumeReconciler{
		client:    client,
		nodeName:  nodeName,
		vgService: proto.NewVGServiceClient(conn),
		lvService: proto.NewLVServiceClient(conn),
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

	if lv.ObjectMeta.DeletionTimestamp == nil {
		if !containsString(lv.Finalizers, topolvm.GetLogicalVolumeFinalizer()) {
			lv2 := lv.DeepCopy()
			lv2.Finalizers = append(lv2.Finalizers, topolvm.GetLogicalVolumeFinalizer())
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
	if !containsString(lv.Finalizers, topolvm.GetLogicalVolumeFinalizer()) {
		// Our finalizer has finished, so the reconciler can do nothing.
		return ctrl.Result{}, nil
	}

	log.Info("start finalizing LogicalVolume", "name", lv.Name)
	err := r.removeLVIfExists(ctx, log, lv)
	if err != nil {
		return ctrl.Result{}, err
	}

	lv2 := lv.DeepCopy()
	lv2.Finalizers = removeString(lv2.Finalizers, topolvm.GetLogicalVolumeFinalizer())
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
	respList, err := r.vgService.GetLVList(ctx, &proto.GetLVListRequest{DeviceClass: lv.Spec.DeviceClass})
	if err != nil {
		log.Error(err, "failed to list LV")
		return err
	}

	for _, v := range respList.Volumes {
		if v.Name != string(lv.UID) {
			continue
		}
		_, err := r.lvService.RemoveLV(ctx, &proto.RemoveLVRequest{Name: string(lv.UID), DeviceClass: lv.Spec.DeviceClass})
		if err != nil {
			log.Error(err, "failed to remove LV", "name", lv.Name, "uid", lv.UID)
			return err
		}
		log.Info("removed LV", "name", lv.Name, "uid", lv.UID)
		return nil
	}
	log.Info("LV already removed", "name", lv.Name, "uid", lv.UID)
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
				SizeGb:       uint64(reqBytes >> 30),
				AccessType:   lv.Spec.AccessType,
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
				Name:        string(lv.UID),
				DeviceClass: lv.Spec.DeviceClass,
				SizeGb:      uint64(reqBytes >> 30),
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
	// lv.Status.CurrentSize is added in v0.4.0 and filled by topolvm-controller when resizing is triggered.
	// The reconciliation loop of LogicalVolume may call expandLV before resizing is triggered.
	// So, lv.Status.CurrentSize could be nil here.
	if lv.Status.CurrentSize == nil {
		return nil
	}

	if lv.Spec.Size.Cmp(*lv.Status.CurrentSize) <= 0 {
		return nil
	}

	origBytes := (*lv.Status.CurrentSize).Value()
	reqBytes := lv.Spec.Size.Value()

	err := func() error {
		_, err := r.lvService.ResizeLV(ctx, &proto.ResizeLVRequest{Name: string(lv.UID), SizeGb: uint64(reqBytes >> 30), DeviceClass: lv.Spec.DeviceClass})
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

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
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
