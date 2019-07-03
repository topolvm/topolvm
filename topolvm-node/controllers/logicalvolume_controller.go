package controllers

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/cybozu-go/topolvm/lvmd/proto"
	topolvmv1 "github.com/cybozu-go/topolvm/topolvm-node/api/v1"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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

	if lv.ObjectMeta.DeletionTimestamp.IsZero() {
		lv = &topolvmv1.LogicalVolume{}
		lv.Namespace = req.NamespacedName.Namespace
		lv.Name = req.NamespacedName.Name
		_, err := ctrl.CreateOrUpdate(ctx, r.k8sClient, lv, func() error {
			if !containsString(lv.Finalizers, finalizerName) {
				lv.Finalizers = append(lv.Finalizers, finalizerName)
			}
			return nil
		})
		if err != nil {
			log.Error(err, "failed to set finalizer", "name", lv.Name)
			return ctrl.Result{}, err
		}

		if lv.Status.VolumeID == "" {
			err = r.createLV(ctx, log, lv, vgService, lvService)
			if err != nil {
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
		log.Error(err, "failed to remove LV", "name", lv.Name)
		return ctrl.Result{}, err
	}

	lv = &topolvmv1.LogicalVolume{}
	lv.Namespace = req.NamespacedName.Namespace
	lv.Name = req.NamespacedName.Name
	_, err = ctrl.CreateOrUpdate(ctx, r.k8sClient, lv, func() error {
		lv.Finalizers = removeString(lv.Finalizers, finalizerName)
		return nil
	})
	if err != nil {
		log.Error(err, "failed to remove finalizers from LogicalVolume", "name", lv.Name)
		return ctrl.Result{}, err
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
	lv.Status.Code = code
	lv.Status.Message = message
	patch, err := json.Marshal(map[string]interface{}{
		"status": map[string]interface{}{
			"code":     code,
			"message":  message,
			"volumeID": lv.Status.VolumeID,
		},
	})
	if err != nil {
		return err
	}

	err = r.k8sClient.Status().Patch(ctx, lv, client.ConstantPatch(types.MergePatchType, patch))
	if err != nil {
		logger.Error(err, "unable to update LogicalVolume status")
	}
	return err
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
		if v.Name == lv.Name {
			lv.Status.VolumeID = lv.Name
			patch, err := json.Marshal(map[string]interface{}{
				"status": map[string]interface{}{
					"volumeID": lv.Name,
					"code":     lv.Status.Code,
					"message":  lv.Status.Message,
				},
			})
			if err != nil {
				return false, err
			}
			err = r.k8sClient.Status().Patch(ctx, lv, client.ConstantPatch(types.MergePatchType, patch))
			if err != nil {
				log.Error(err, "failed to add VolumeID", "name", lv.Name)
				return true, err
			}
			return true, nil
		}
	}
	return false, nil
}

func (r *LogicalVolumeReconciler) createLV(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume,
	vgService proto.VGServiceClient, lvService proto.LVServiceClient) error {

	reqBytes, ok := lv.Spec.Size.AsInt64()
	if !ok {
		log.Error(errors.New("not ok"), "failed to interpret spec.size as int64")
		err := r.updateStatusWithError(ctx, log, lv, codes.Internal, "failed to interpret spec.size as int64")
		return err
	}

	// This function's logic is not atomic, so it is possible that LV is created but LogicalVolume.status is not updated.
	// We check existence the LV to ensure idempotence of createLV.
	found, err := r.updateVolumeIfExists(ctx, log, lv, vgService)
	if err != nil {
		return err
	} else if found {
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

	lv.Status.VolumeID = resp.Volume.Name
	patch, err := json.Marshal(map[string]interface{}{
		"status": map[string]interface{}{
			"volumeID": resp.Volume.Name,
			"code":     lv.Status.Code,
			"message":  lv.Status.Message,
		},
	})
	if err != nil {
		return err
	}

	err = r.k8sClient.Status().Patch(ctx, lv, client.ConstantPatch(types.MergePatchType, patch))
	if err != nil {
		log.Error(err, "failed to update VolumeID", "name", lv.Name, "uid", lv.UID)
		return err
	}

	log.Info("created new LV", "name", lv.Name, "uid", lv.UID, "status.volumeID", lv.Status.VolumeID)
	return nil
}

type logicalVolumeFilter struct {
	log      logr.Logger
	nodeName string
}

func (f logicalVolumeFilter) Create(e event.CreateEvent) bool {
	var lv *topolvmv1.LogicalVolume
	lv = e.Object.(*topolvmv1.LogicalVolume)
	if lv == nil {
		return false
	}
	if lv.Spec.NodeName == f.nodeName {
		return true
	}
	return false
}

func (f logicalVolumeFilter) Delete(e event.DeleteEvent) bool {
	return false
}

func (f logicalVolumeFilter) Update(e event.UpdateEvent) bool {
	return true
}

func (f logicalVolumeFilter) Generic(e event.GenericEvent) bool {
	return true
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
