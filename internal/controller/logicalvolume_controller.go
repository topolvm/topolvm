package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/topolvm/topolvm"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/internal/mounter"
	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
)

// LogicalVolumeReconciler reconciles a LogicalVolume object
type LogicalVolumeReconciler struct {
	client client.Client

	nodeName  string
	vgService proto.VGServiceClient
	lvService proto.LVServiceClient
	lvMount   *mounter.LVMount
	snapshot  *snapshotHandler
}

//+kubebuilder:rbac:groups=topolvm.io,resources=logicalvolumes,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=topolvm.io,resources=logicalvolumes/status,verbs=get;update;patch

func NewLogicalVolumeReconcilerWithServices(client client.Client, nodeName string, vgService proto.VGServiceClient, lvService proto.LVServiceClient) *LogicalVolumeReconciler {
	r := &LogicalVolumeReconciler{
		client:    client,
		nodeName:  nodeName,
		vgService: vgService,
		lvService: lvService,
		lvMount:   mounter.NewLVMount(client, vgService, lvService),
	}
	r.snapshot = newSnapshotHandler(r)
	return r
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

	// Check for pending deletion annotation
	if isPendingDeletion(lv) {
		if controllerutil.ContainsFinalizer(lv, topolvm.GetLogicalVolumeFinalizer()) {
			log.Error(nil, "logical volume was pending deletion but still has finalizer", "name", lv.Name)
		} else {
			log.Info("skipping finalizer for logical volume due to its pending deletion", "name", lv.Name)
		}
		return ctrl.Result{}, nil
	}

	// Handle deletion
	if lv.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, lv, log)
	}

	// Normal reconciliation
	return r.reconcile(ctx, lv, log)
}

func (r *LogicalVolumeReconciler) reconcile(ctx context.Context, lv *topolvmv1.LogicalVolume, log logr.Logger) (ctrl.Result, error) {
	log.Info("reconciling LogicalVolume", "name", lv.Name)
	if result, err := r.ensureFinalizerAndLabels(ctx, lv, log); err != nil || result.RequeueAfter > 0 {
		return result, err
	}

	// Prepare snapshot context
	if err := r.snapshot.buildSnapshotContext(ctx, log, lv); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to build snapshot context: %w", err)
	}
	if lv.Status.VolumeID == "" {
		log.Info("LogicalVolume has no VolumeID, creating a new", "name", lv.Name)
		return r.reconcileVolumeCreation(ctx, log, r.snapshot.shouldRestore, lv)
	}

	if err := r.reconcileVolumeExpansion(ctx, lv, log); err != nil {
		return ctrl.Result{}, err
	}

	if r.snapshot.shouldRestore {
		log.Info("Processing snapshot restore", "name", lv.Name, "sourceLV", r.snapshot.sourceLV.Name)
		if result, err := r.reconcileSnapshotRestore(ctx, log, lv); err != nil || result.RequeueAfter > 0 {
			return result, err
		}
	}

	if err := r.snapshot.buildSnapshotContextFrBackup(ctx, log, lv); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to build snapshot context for backup: %w", err)
	}
	if r.snapshot.shouldBackup {
		log.Info("Processing snapshot backup", "name", lv.Name)
		if result, err := r.reconcileSnapshotBackup(ctx, log, lv); err != nil {
			return result, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *LogicalVolumeReconciler) reconcileVolumeCreation(ctx context.Context, log logr.Logger, shouldRestore bool, lv *topolvmv1.LogicalVolume) (ctrl.Result, error) {
	if err := r.createLV(ctx, log, lv, shouldRestore); err != nil {
		log.Error(err, "failed to create LV", "name", lv.Name)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *LogicalVolumeReconciler) reconcileVolumeExpansion(ctx context.Context, lv *topolvmv1.LogicalVolume, log logr.Logger) error {
	if err := r.expandLV(ctx, log, lv); err != nil {
		log.Error(err, "failed to expand LV", "name", lv.Name)
		return err
	}
	return nil
}

func (r *LogicalVolumeReconciler) reconcileSnapshotRestore(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) (ctrl.Result, error) {
	// Cleanup if complete
	if isSnapshotOperationComplete(lv) {
		log.Info("snapshot restore completed successfully", "name", lv.Name)
		if !hasSnapshotRestoreExecutorCleanupCondition(lv) {
			if err := r.snapshot.executeCleanerOperation(ctx, log, lv, topolvmv1.OperationRestore); err != nil {
				log.Error(err, "failed to execute cleaner operation", "name", lv.Name)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if hasSnapshotRestoreExecutorCondition(lv) {
		log.Info("Snapshot Restore Executor triggered previously, waiting for completion", "name", lv.Name)
		return ctrl.Result{}, nil
	}

	// Otherwise, check if PV exists if exists, restore and return
	pvExists, err := r.snapshot.checkPVExists(ctx, lv)
	if err != nil {
		log.Error(err, "Failed to check PV existence")
		return ctrl.Result{}, err
	}
	if !pvExists {
		log.Info("PV does not exist yet; waiting", "name", lv.Name)
		return ctrl.Result{RequeueAfter: requeueIntervalForSimpleUpdate}, nil
	}
	if err := r.snapshot.restoreFromSnapshot(ctx, log, lv); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *LogicalVolumeReconciler) reconcileSnapshotBackup(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) (ctrl.Result, error) {
	// Cleanup if complete
	if isSnapshotOperationComplete(lv) {
		log.Info("snapshot backup completed successfully", "name", lv.Name)
		// First, cleanup the executor pod
		if !hasSnapshotBackupExecutorCleanupCondition(lv) {
			if err := r.snapshot.executeCleanerOperation(ctx, log, lv, topolvmv1.OperationBackup); err != nil {
				log.Error(err, "failed to execute cleaner operation", "name", lv.Name)
				return ctrl.Result{}, err
			}
		}

		// Then, cleanup the LVM snapshot volume only if the snapshot operation succeeded
		if !hasLVMSnapshotCleanupCondition(lv) &&
			hasSnapshotBackupExecutorCleanupCondition(lv) &&
			lv.Status.Snapshot.Phase == topolvmv1.OperationPhaseSucceeded {
			if err := r.snapshot.cleanupLVMSnapshotAfterBackup(ctx, log, lv); err != nil {
				log.Error(err, "failed to cleanup LVM snapshot", "name", lv.Name)
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: requeueIntervalForSimpleUpdate}, nil
		}

		msg := "LVM snapshot removed after backup completion"
		if hasLVMSnapshotCleanupCondition(lv) && lv.Status.Message != msg {
			if err := r.snapshot.updateStatusMessageAfterSnapshotRemoval(ctx, log, lv, msg); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to update LV status after snapshot removal: %w", err)
			}
		}

		return ctrl.Result{}, nil
	}

	if hasSnapshotBackupExecutorCondition(lv) {
		log.Info("Snapshot Backup Executor triggered previously, waiting for completion", "name", lv.Name)
		return ctrl.Result{}, nil
	}

	// Otherwise, take backup of the snapshot and return
	err := r.snapshot.backupSnapshot(ctx, log, lv)
	return ctrl.Result{}, err
}

func (r *LogicalVolumeReconciler) ensureFinalizerAndLabels(ctx context.Context, lv *topolvmv1.LogicalVolume, log logr.Logger) (ctrl.Result, error) {
	// Ensure finalizer
	if !controllerutil.ContainsFinalizer(lv, topolvm.GetLogicalVolumeFinalizer()) {
		lv2 := lv.DeepCopy()
		controllerutil.AddFinalizer(lv2, topolvm.GetLogicalVolumeFinalizer())
		patch := client.MergeFrom(lv)
		if err := r.client.Patch(ctx, lv2, patch); err != nil {
			log.Error(err, "failed to add finalizer", "name", lv.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: requeueIntervalForSimpleUpdate}, nil
	}

	// Ensure labels
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
		return ctrl.Result{RequeueAfter: requeueIntervalForSimpleUpdate}, nil
	}

	return ctrl.Result{}, nil
}

func (r *LogicalVolumeReconciler) handleDeletion(ctx context.Context, lv *topolvmv1.LogicalVolume, log logr.Logger) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(lv, topolvm.GetLogicalVolumeFinalizer()) {
		// Finalizer already removed, nothing to do
		return ctrl.Result{}, nil
	}

	log.Info("finalizing LogicalVolume", "name", lv.Name)

	if isLVHasSnapshot(lv) {
		return r.deletionWithSnapshot(ctx, lv, log)
	}

	return r.deletionWithoutSnapshot(ctx, lv, log)
}

func (r *LogicalVolumeReconciler) deletionWithSnapshot(ctx context.Context, lv *topolvmv1.LogicalVolume, log logr.Logger) (ctrl.Result, error) {
	if !hasSnapshotDeleteExecutorCondition(lv) {
		if err := r.snapshot.executeSnapshotDeleteOperation(ctx, log, lv); err != nil {
			log.Error(err, "snapshot delete operation failed", "name", lv.Name)
			return ctrl.Result{}, err
		}
	}

	if err := r.removeLVIfExists(ctx, log, lv); err != nil {
		return ctrl.Result{}, err
	}

	if hasConditionSnapshotDeleteSucceeded(lv) {
		return r.removeFinalizer(ctx, log, lv)
	}

	return ctrl.Result{}, nil
}

func (r *LogicalVolumeReconciler) deletionWithoutSnapshot(ctx context.Context, lv *topolvmv1.LogicalVolume, log logr.Logger) (ctrl.Result, error) {
	if err := r.removeLVIfExists(ctx, log, lv); err != nil {
		return ctrl.Result{}, err
	}
	return r.removeFinalizer(ctx, log, lv)
}

func (r *LogicalVolumeReconciler) removeFinalizer(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) (ctrl.Result, error) {
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

func (r *LogicalVolumeReconciler) createLV(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume, restoreFrSnapshot bool) error {
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
		if lv.Spec.Source != "" && !restoreFrSnapshot {
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
			currentSize := sourcelv.Status.CurrentSize.Value()
			if reqBytes < currentSize {
				return fmt.Errorf("cannot create new LV, requested size %d is smaller than source LV size %d", reqBytes, currentSize)
			}

			// Create a snapshot lv
			resp, err := r.lvService.CreateLVSnapshot(ctx, &proto.CreateLVSnapshotRequest{
				Name:         string(lv.UID),
				DeviceClass:  lv.Spec.DeviceClass,
				SourceVolume: sourceVolID,
				SizeBytes:    reqBytes,
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
				Name:                string(lv.UID),
				DeviceClass:         lv.Spec.DeviceClass,
				LvcreateOptionClass: lv.Spec.LvcreateOptionClass,
				SizeBytes:           reqBytes,
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

		if restoreFrSnapshot {
			metav1.SetMetaDataAnnotation(&lv.ObjectMeta, topolvm.GetResticRestoreRequiredKey(), "true")
		}

		lv.Status.VolumeID = volume.Name
		lv.Status.CurrentSize = resource.NewQuantity(volume.SizeBytes, resource.BinarySI)
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

	if err := updateLVStatus(ctx, r.client, lv); err != nil {
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
		resp, err := r.lvService.ResizeLV(ctx, &proto.ResizeLVRequest{
			Name:        string(lv.UID),
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

		lv.Status.CurrentSize = resource.NewQuantity(resp.SizeBytes, resource.BinarySI)
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
		"original status.currentSize", origBytes, "status.currentSize", lv.Status.CurrentSize, "spec.size", reqBytes)
	return nil
}

// logicalVolumeFilter filters LogicalVolume by nodeName.
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
