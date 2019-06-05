/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

// Reconcile manages LogicalVolume according to Spec and Status.Phase.
func (r *LogicalVolumeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.logger.WithValues("logicalvolume", req.NamespacedName)
	lvService := proto.NewLVServiceClient(r.lvmdClient)
	vgService := proto.NewVGServiceClient(r.lvmdClient)

	// your logic here
	lv := new(topolvmv1.LogicalVolume)
	if err := r.k8sClient.Get(ctx, req.NamespacedName, lv); err != nil {
		log.Error(err, "unable to fetch LogicalVolume")
		return ctrl.Result{}, ignoreNotFound(err)
	}

	log.Info("RECONCILE!!", "LogicalVolume", lv)

	if lv.ObjectMeta.DeletionTimestamp.IsZero() {
		if lv.Status.Code != codes.OK {
			return ctrl.Result{}, nil
		}

		if lv.Status.VolumeID == "" {
			// todo: set finalizer

			reqBytes, ok := lv.Spec.Size.AsInt64()
			if !ok {
				err := r.updateStatusWithError(ctx, log, lv, codes.Internal, "failed to interpret LogicalVolume's Spec.Size as int64")
				return ctrl.Result{}, err
			}

			respList, err := vgService.GetLVList(ctx, &proto.Empty{})
			if err != nil {
				err := r.updateStatusWithError(ctx, log, lv, codes.Internal, "failed to get list of LV")
				return ctrl.Result{}, err
			}

			for _, v := range respList.Volumes {
				if v.Name == lv.Name {
					lv.Status.VolumeID = lv.Name
					// todo: update and return
				}
			}

			resp, err := lvService.CreateLV(ctx, &proto.CreateLVRequest{Name: lv.Name, SizeGb: uint64(reqBytes >> 30)})
			if err != nil {
				s, ok := status.FromError(err)
				if !ok {
					err := r.updateStatusWithError(ctx, log, lv, codes.Internal, err.Error())
					return ctrl.Result{}, err
				}
				err := r.updateStatusWithError(ctx, log, lv, s.Code(), s.Message())
				return ctrl.Result{}, err
			}

			lv.Status.VolumeID = resp.Volume.Name
			// todo: update and return
		}
	} else {
		// todo: finalizer!!!
		_, err := lvService.RemoveLV(ctx, &proto.RemoveLVRequest{Name: lv.Name})
		_ = err
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
	err := r.k8sClient.Status().Update(ctx, lv)
	if err != nil {
		logger.Error(err, "unable to update LogicalVolume status")
	}
	return err
}

type logicalVolumeFilter struct {
	log      logr.Logger
	nodeName string
}

func (f logicalVolumeFilter) Create(e event.CreateEvent) bool {
	f.log.Info("CREATE", "event", e)
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
	f.log.Info("DELETE", "event", e)
	return false
}

func (f logicalVolumeFilter) Update(e event.UpdateEvent) bool {
	f.log.Info("UPDATE", "event", e)
	return true
}

func (f logicalVolumeFilter) Generic(e event.GenericEvent) bool {
	f.log.Info("GENERIC", "event", e)
	return true
}
