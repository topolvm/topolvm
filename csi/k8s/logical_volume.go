package k8s

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/topolvm"
	"github.com/cybozu-go/topolvm/csi"
	topolvmv1 "github.com/cybozu-go/topolvm/topolvm-node/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type logicalVolumeService struct {
	mgr       manager.Manager
	namespace string
	mu        sync.Mutex
}

var (
	scheme = runtime.NewScheme()
)

const indexFieldVolumeID = "status.volumeID"

// NewLogicalVolumeService returns LogicalVolumeService.
func NewLogicalVolumeService(namespace string) (csi.LogicalVolumeService, error) {
	err := topolvmv1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}
	err = mgr.GetFieldIndexer().IndexField(&topolvmv1.LogicalVolume{}, indexFieldVolumeID, func(o runtime.Object) []string {
		return []string{o.(*topolvmv1.LogicalVolume).Status.VolumeID}
	})
	if err != nil {
		return nil, err
	}

	return &logicalVolumeService{
		mgr:       mgr,
		namespace: namespace,
	}, nil
}

func (s *logicalVolumeService) CreateVolume(ctx context.Context, node string, name string, sizeGb int64) (string, error) {
	log.Info("k8s.CreateVolume called", map[string]interface{}{
		"name":    name,
		"node":    node,
		"size_gb": sizeGb,
	})

	s.mu.Lock()
	defer s.mu.Unlock()

	lv := &topolvmv1.LogicalVolume{
		TypeMeta: metav1.TypeMeta{
			Kind:       "LogicalVolume",
			APIVersion: "topolvm.cybozu.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: s.namespace,
		},
		Spec: topolvmv1.LogicalVolumeSpec{
			Name:     name,
			NodeName: node,
			Size:     *resource.NewQuantity(sizeGb<<30, resource.BinarySI),
		},
	}

	existingLV := new(topolvmv1.LogicalVolume)
	err := s.mgr.GetClient().Get(ctx, client.ObjectKey{Namespace: s.namespace, Name: name}, existingLV)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return "", err
		}

		err := s.mgr.GetClient().Create(ctx, lv)
		if err != nil {
			return "", err
		}
		log.Info("creat LogicalVolume", map[string]interface{}{
			"name": name,
		})
	} else {
		// LV with same name was found; check compatibility
		// skip check of capabilities because (1) we allow both of two access types, and (2) we allow only one access mode
		// for ease of comparison, sizes are compared strictly, not by compatibility of ranges
		if !existingLV.IsCompatibleWith(lv) {
			return "", status.Error(codes.AlreadyExists, "Incompatible LogicalVolume already exists")
		}
		// compatible LV was found
	}

	for {
		var newLV topolvmv1.LogicalVolume
		err := s.mgr.GetClient().Get(ctx, client.ObjectKey{Namespace: s.namespace, Name: name}, &newLV)
		if err != nil {
			return "", err
		}
		if newLV.Status.VolumeID != "" {
			log.Info("end k8s.LogicalVolume", map[string]interface{}{
				"volume_id": newLV.Status.VolumeID,
			})
			return newLV.Status.VolumeID, nil
		}
		if newLV.Status.Code != codes.OK {
			err := s.mgr.GetClient().Delete(ctx, &newLV)
			if err != nil {
				// log this error but do not return this error, because newLV.Status.Message is more important
				log.Error("failed to delete LogicalVolume", map[string]interface{}{
					log.FnError: err,
				})
			}
			return "", status.Error(newLV.Status.Code, newLV.Status.Message)
		}

		select {
		case <-ctx.Done():
			return "", errors.New("timed out")
		case <-time.After(1 * time.Second):
		}
	}
}

func (s *logicalVolumeService) DeleteVolume(ctx context.Context, volumeID string) error {
	lvList := new(topolvmv1.LogicalVolumeList)
	err := s.mgr.GetClient().List(ctx, lvList,
		client.InNamespace(topolvm.SystemNamespace),
		client.MatchingField(indexFieldVolumeID, volumeID))
	if err != nil {
		return err
	}
	if len(lvList.Items) == 0 {
		log.Info("volume is not found", map[string]interface{}{
			"volume_id": volumeID,
		})
		return nil
	} else if len(lvList.Items) > 1 {
		return fmt.Errorf("multiple LogicalVolume is found for VolumeID %s", volumeID)
	}

	return s.mgr.GetClient().Delete(ctx, &lvList.Items[0])
}

func (s *logicalVolumeService) ExpandVolume(ctx context.Context, volumeID string, sizeGb int64) error {
	panic("implement me")
}
