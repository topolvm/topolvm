package k8s

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/topolvm"
	"github.com/cybozu-go/topolvm/csi"
	"github.com/cybozu-go/topolvm/driver"
	topolvmv1 "github.com/cybozu-go/topolvm/topolvm-node/api/v1"
	"github.com/cybozu-go/well"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
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

const (
	indexFieldVolumeID = "status.volumeID"
)

var (
	scheme = runtime.NewScheme()
)

// NewLogicalVolumeService returns LogicalVolumeService.
func NewLogicalVolumeService(namespace string) (driver.LogicalVolumeService, error) {
	err := topolvmv1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	err = corev1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	err = storagev1.AddToScheme(scheme)
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

	well.Go(func(ctx context.Context) error {
		if err := mgr.Start(ctx.Done()); err != nil {
			log.Error("failed to start manager", map[string]interface{}{log.FnError: err})
			return err
		}
		return nil
	})

	return &logicalVolumeService{
		mgr:       mgr,
		namespace: namespace,
	}, nil

}

func (s *logicalVolumeService) CreateVolume(ctx context.Context, node string, name string, sizeGb int64, capabilities []*csi.VolumeCapability) (string, error) {
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
		log.Info("created LogicalVolume CRD", map[string]interface{}{
			"name":      name,
			"namespace": s.namespace,
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
		log.Info("waiting for setting 'status.volumeID'", map[string]interface{}{
			"namespace": s.namespace,
			"name":      name,
		})
		select {
		case <-ctx.Done():
			log.Info("context is done", map[string]interface{}{})
			return "", errors.New("timed out")
		case <-time.After(1 * time.Second):
		}

		var newLV topolvmv1.LogicalVolume
		err := s.mgr.GetClient().Get(ctx, client.ObjectKey{Namespace: s.namespace, Name: name}, &newLV)
		if err != nil {
			log.Error("failed to get LogicalVolume", map[string]interface{}{
				log.FnError: err,
				"namespace": s.namespace,
				"name":      name,
			})
			continue
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
	}
}

func (s *logicalVolumeService) DeleteVolume(ctx context.Context, volumeID string) error {
	lvList := new(topolvmv1.LogicalVolumeList)
	err := s.mgr.GetClient().List(ctx, lvList,
		client.InNamespace(s.namespace),
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

func (s *logicalVolumeService) getLogicalVolumeListByVolumeID(ctx context.Context, volumeID string) (*topolvmv1.LogicalVolumeList, error) {
	lvList := new(topolvmv1.LogicalVolumeList)
	err := s.mgr.GetClient().List(ctx, lvList,
		client.InNamespace(s.namespace),
		client.MatchingField(indexFieldVolumeID, volumeID))
	if err != nil {
		return nil, err
	}
	return lvList, nil
}

func (s *logicalVolumeService) VolumeExists(ctx context.Context, volumeID string) error {
	lvList, err := s.getLogicalVolumeListByVolumeID(ctx, volumeID)
	if err != nil {
		return err
	}

	if len(lvList.Items) > 1 {
		return errors.New("found multiple volumes with volumeID " + volumeID)
	}
	if len(lvList.Items) == 0 {
		return driver.ErrVolumeNotFound
	}
	return nil
}

func (s *logicalVolumeService) listNodes(ctx context.Context) (*corev1.NodeList, error) {
	nl := new(corev1.NodeList)
	err := s.mgr.GetClient().List(ctx, nl)
	if err != nil {
		return nil, err
	}
	return nl, nil
}

func (s *logicalVolumeService) ExpandVolume(ctx context.Context, volumeID string, sizeGb int64) error {
	panic("implement me")
}

func (s *logicalVolumeService) GetCapacity(ctx context.Context, requestNodeNumber string) (int64, error) {
	nl, err := s.listNodes(ctx)
	if err != nil {
		return 0, err
	}

	capacity := int64(0)
	if len(requestNodeNumber) == 0 {
		for _, node := range nl.Items {
			capacity += s.getNodeCapacity(node)
		}
		return capacity, nil
	}

	for _, node := range nl.Items {
		if nodeNumber, ok := node.Annotations[topolvm.TopologyNodeKey]; ok {
			if requestNodeNumber != nodeNumber {
				continue
			}
			c, ok := node.Annotations[topolvm.CapacityKey]
			if !ok {
				return 0, fmt.Errorf("%s is not found", topolvm.CapacityKey)
			}
			return strconv.ParseInt(c, 10, 64)
		}
	}

	return 0, errors.New("capacity not found")
}

func (s *logicalVolumeService) GetMaxCapacity(ctx context.Context) (string, int64, error) {
	nl, err := s.listNodes(ctx)
	if err != nil {
		return "", 0, err
	}
	var nodeName string
	var maxCapacity int64
	for _, node := range nl.Items {
		c := s.getNodeCapacity(node)

		if maxCapacity < c {
			maxCapacity = c
			nodeName = node.Name
		}
	}
	return nodeName, maxCapacity, nil
}

func (s *logicalVolumeService) getNodeCapacity(node corev1.Node) int64 {
	c, ok := node.Annotations[topolvm.CapacityKey]
	if !ok {
		return 0
	}
	val, _ := strconv.ParseInt(c, 10, 64)
	return val
}
