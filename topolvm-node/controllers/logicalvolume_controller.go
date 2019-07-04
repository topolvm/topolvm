package controllers

import (
	"context"

	"github.com/cybozu-go/topolvm/lvmd/proto"
	topolvmv1 "github.com/cybozu-go/topolvm/topolvm-node/api/v1"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

const finalizerName = "logicalvolume.topolvm.cybozu.com"

// NewLogicalVolumeReconciler returns LogicalVolumeReconciler.
func NewLogicalVolumeReconciler(client client.Client, log logr.Logger, nodeName string, conn *grpc.ClientConn) *LogicalVolumeReconciler {
	return &LogicalVolumeReconciler{
		k8sClient:  client,
		logger:     log,
		nodeName:   nodeName,
		lvmdClient: conn,
	}
}

// LogicalVolumeReconciler reconciles a LogicalVolume object
type LogicalVolumeReconciler struct {
	k8sClient  client.Client
	logger     logr.Logger
	nodeName   string
	lvmdClient *grpc.ClientConn
}

func ignoreNotFound(err error) error {
	if apierrs.IsNotFound(err) {
		return nil
	}
	return err
}

// +kubebuilder:rbac:groups=topolvm.cybozu.com,resources=logicalvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=topolvm.cybozu.com,resources=logicalvolumes/status,verbs=get;update;patch

// Reconcile manages LogicalVolume and its LV
func (r *LogicalVolumeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.logger.WithValues("logicalvolume", req.NamespacedName)
	lvService := proto.NewLVServiceClient(r.lvmdClient)
	vgService := proto.NewVGServiceClient(r.lvmdClient)

	lv := new(topolvmv1.LogicalVolume)
	if err := r.k8sClient.Get(ctx, req.NamespacedName, lv); err != nil {
		if !apierrs.IsNotFound(err) {
			log.Error(err, "unable to fetch LogicalVolume")
		}
		return ctrl.Result{}, ignoreNotFound(err)
	}

	if lv.Spec.NodeName != r.nodeName {
		log.Info("unfilterd logical volue", "nodeName", lv.Spec.NodeName)
		return ctrl.Result{}, nil
	}

	if lv.ObjectMeta.DeletionTimestamp.IsZero() {
		// When lv.Status.Code is not codes.OK (== 0), CreateLV has already failed.
		// LogicalVolume CRD will be deleted soon by the controller.
		if lv.Status.Code != codes.OK {
			return ctrl.Result{}, nil
		}

		if !containsString(lv.Finalizers, finalizerName) {
			lv2 := lv.DeepCopy()
			lv2.Finalizers = append(lv2.Finalizers, finalizerName)
			patch := client.MergeFrom(lv)
			if err := r.k8sClient.Patch(ctx, lv2, patch); err != nil {
				log.Error(err, "failed to add finalizer", "name", lv.Name)
				return ctrl.Result{Requeue: true}, err
			}
		}

		if lv.Status.VolumeID == "" {
			if err := r.createLV(ctx, log, lv, vgService, lvService); err != nil {
				log.Error(err, "failed to create LV", "name", lv.Name)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
		//
		// TODO: handle requests to expand volume, here
		//
	}

	// finalization
	if !containsString(lv.Finalizers, finalizerName) {
		// Our finalizer has finished, so the reconciler can do nothing.
		return ctrl.Result{}, nil
	}

	log.Info("start finalizing LogicalVolume", "name", lv.Name)
	err := r.removeLVIfExists(ctx, log, lv, vgService, lvService)
	if err != nil {
		return ctrl.Result{}, err
	}

	lv2 := lv.DeepCopy()
	lv2.Finalizers = removeString(lv2.Finalizers, finalizerName)
	patch := client.MergeFrom(lv)
	if err := r.k8sClient.Patch(ctx, lv2, patch); err != nil {
		log.Error(err, "failed to remove finalizer", "name", lv.Name)
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up Reconciler with Manager.
func (r *LogicalVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topolvmv1.LogicalVolume{}).
		WithEventFilter(&logicalVolumeFilter{r.logger, r.nodeName}).
		Complete(r)
}

func (r *LogicalVolumeReconciler) updateStatusWithError(ctx context.Context, logger logr.Logger, lv *topolvmv1.LogicalVolume, code codes.Code, message string) error {
	lv2 := lv.DeepCopy()
	lv2.Status.Code = code
	lv2.Status.Message = message
	patch := client.MergeFrom(lv)

	if err := r.k8sClient.Status().Patch(ctx, lv2, patch); err != nil {
		logger.Error(err, "unable to update LogicalVolume status")
		return err
	}
	return nil
}

func (r *LogicalVolumeReconciler) removeLVIfExists(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume, vgService proto.VGServiceClient, lvService proto.LVServiceClient) error {
	// Finalizer's process ( RemoveLV then removeString ) is not atomic,
	// so checking existence of LV to ensure its idempotence
	respList, err := vgService.GetLVList(ctx, &proto.Empty{})
	if err != nil {
		log.Error(err, "failed to list LV")
		return err
	}

	for _, v := range respList.Volumes {
		if v.Name == string(lv.UID) {
			_, err := lvService.RemoveLV(ctx, &proto.RemoveLVRequest{Name: string(lv.UID)})
			if err != nil {
				log.Error(err, "failed to remove LV", "name", lv.Name, "uid", lv.UID)
				return err
			}
			log.Info("removed LV", "name", lv.Name, "uid", lv.UID)
			return nil
		}
	}
	log.Info("LV already removed", "name", lv.Name, "uid", lv.UID)
	return nil
}

func (r *LogicalVolumeReconciler) updateVolumeIfExists(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume, vgService proto.VGServiceClient) (found bool, err error) {
	respList, err := vgService.GetLVList(ctx, &proto.Empty{})
	if err != nil {
		log.Error(err, "failed to get list of LV")
		err := r.updateStatusWithError(ctx, log, lv, codes.Internal, "failed to get list of LV")
		return false, err
	}

	for _, v := range respList.Volumes {
		if v.Name == string(lv.UID) {
			lv2 := lv.DeepCopy()
			lv2.Status.VolumeID = v.Name
			patch := client.MergeFrom(lv)
			if err := r.k8sClient.Status().Patch(ctx, lv2, patch); err != nil {
				log.Error(err, "failed to update VolumeID in status", "name", lv.Name)
				return true, err
			}
			return true, nil
		}
	}
	return false, nil
}

func (r *LogicalVolumeReconciler) createLV(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume,
	vgService proto.VGServiceClient, lvService proto.LVServiceClient) error {

	reqBytes := lv.Spec.Size.Value()

	// In case the controller crashed just after LVM LV creation, LV may already exist.
	found, err := r.updateVolumeIfExists(ctx, log, lv, vgService)
	if err != nil {
		return err
	}
	if found {
		log.Info("set volumeID to existing LogicalVolume", "name", lv.Name, "uid", lv.UID, "status.volumeID", lv.Status.VolumeID)
		return nil
	}

	resp, err := lvService.CreateLV(ctx, &proto.CreateLVRequest{Name: string(lv.UID), SizeGb: uint64(reqBytes >> 30)})
	if err != nil {
		s, ok := status.FromError(err)
		if !ok {
			log.Error(err, err.Error())
			err := r.updateStatusWithError(ctx, log, lv, codes.Internal, err.Error())
			return err
		}
		log.Error(err, s.Message())
		err := r.updateStatusWithError(ctx, log, lv, s.Code(), s.Message())
		return err
	}

	lv2 := lv.DeepCopy()
	lv2.Status.VolumeID = resp.Volume.Name
	patch := client.MergeFrom(lv)
	if err := r.k8sClient.Status().Patch(ctx, lv2, patch); err != nil {
		log.Error(err, "failed to update VolumeID", "name", lv.Name, "uid", lv.UID)
		return err
	}

	log.Info("created new LV", "name", lv2.Name, "uid", lv2.UID, "status.volumeID", lv2.Status.VolumeID)
	return nil
}

type logicalVolumeFilter struct {
	log      logr.Logger
	nodeName string
}

func (f logicalVolumeFilter) filter(lv *topolvmv1.LogicalVolume) bool {
	if lv == nil {
		return false
	}
	if lv.Spec.NodeName == f.nodeName {
		return true
	}
	return false
}

func (f logicalVolumeFilter) Create(e event.CreateEvent) bool {
	return f.filter(e.Object.(*topolvmv1.LogicalVolume))
}

func (f logicalVolumeFilter) Delete(e event.DeleteEvent) bool {
	return f.filter(e.Object.(*topolvmv1.LogicalVolume))
}

func (f logicalVolumeFilter) Update(e event.UpdateEvent) bool {
	return f.filter(e.ObjectNew.(*topolvmv1.LogicalVolume))
}

func (f logicalVolumeFilter) Generic(e event.GenericEvent) bool {
	return f.filter(e.Object.(*topolvmv1.LogicalVolume))
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
