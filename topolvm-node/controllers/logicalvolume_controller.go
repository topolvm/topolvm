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

func (r *LogicalVolumeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.logger.WithValues("logicalvolume", req.NamespacedName)
	lvService := proto.NewLVServiceClient(r.lvmdClient)

	// your logic here
	var lv topolvmv1.LogicalVolume
	if err := r.k8sClient.Get(ctx, req.NamespacedName, &lv); err != nil {
		log.Error(err, "unable to fetch LogicalVolume")
		return ctrl.Result{}, ignoreNotFound(err)
	}

	log.Info("RECONCILE!!", "LogicalVolume", lv)

	switch lv.Status.Phase {
	case topolvmv1.PhaseInitial:
		reqBytes, ok := lv.Spec.Size.AsInt64()
		if !ok {
			lv.Status.Phase = topolvmv1.PhaseCreateFailed
			lv.Status.Code = codes.Internal
			lv.Status.Message = "failed to interpret LogicalVolume's Spec.Size as int64"
			break
		}

		resp, err := lvService.CreateLV(ctx, &proto.CreateLVRequest{Name: lv.Name, SizeGb: uint64(reqBytes >> 30)})
		if err != nil {
			lv.Status.Phase = topolvmv1.PhaseCreateFailed
			s, ok := status.FromError(err)
			if !ok {
				lv.Status.Code = codes.Internal
				lv.Status.Message = err.Error()
			} else {
				lv.Status.Code = s.Code()
				lv.Status.Message = s.Message()
			}
			break
		}

		lv.Status.Phase = topolvmv1.PhaseCreated
		lv.Status.VolumeID = resp.Volume.Name

	case topolvmv1.PhaseTerminating:
		_, err := lvService.RemoveLV(ctx, &proto.RemoveLVRequest{Name: lv.Name})
		if err != nil {
			lv.Status.Phase = topolvmv1.PhaseTerminateFailed
			s, ok := status.FromError(err)
			if !ok {
				lv.Status.Code = codes.Internal
				lv.Status.Message = err.Error()
			} else {
				lv.Status.Code = s.Code()
				lv.Status.Message = s.Message()
			}
			break
		}

		lv.Status.Phase = topolvmv1.PhaseTerminated

	default:
		return ctrl.Result{}, nil

	}

	if err := r.k8sClient.Status().Update(ctx, &lv); err != nil {
		log.Error(err, "unable to update LogicalVolume status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *LogicalVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topolvmv1.LogicalVolume{}).
		WithEventFilter(&LogicalVolumeFilter{r.logger, r.nodeName}).
		Complete(r)
}

type LogicalVolumeFilter struct {
	log      logr.Logger
	nodeName string
}

func (f LogicalVolumeFilter) Create(e event.CreateEvent) bool {
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

func (f LogicalVolumeFilter) Delete(e event.DeleteEvent) bool {
	f.log.Info("DELETE", "event", e)
	return false
}

func (f LogicalVolumeFilter) Update(e event.UpdateEvent) bool {
	f.log.Info("UPDATE", "event", e)
	return true
}

func (f LogicalVolumeFilter) Generic(e event.GenericEvent) bool {
	f.log.Info("GENERIC", "event", e)
	return true
}
