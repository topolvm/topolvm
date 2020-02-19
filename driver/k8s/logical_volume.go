package k8s

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/cybozu-go/topolvm"
	topolvmv1 "github.com/cybozu-go/topolvm/api/v1"
	"github.com/cybozu-go/topolvm/csi"
	"github.com/cybozu-go/topolvm/driver"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

type logicalVolumeService struct {
	client.Client
	mu sync.Mutex
}

const (
	indexFieldVolumeID = "status.volumeID"
)

var (
	scheme = runtime.NewScheme()
	logger = logf.Log.WithName("LogicalVolume")
)

// +kubebuilder:rbac:groups=topolvm.cybozu.com,resources=logicalvolumes,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// NewLogicalVolumeService returns LogicalVolumeService.
func NewLogicalVolumeService(mgr manager.Manager) (driver.LogicalVolumeService, error) {
	err := mgr.GetFieldIndexer().IndexField(&topolvmv1.LogicalVolume{}, indexFieldVolumeID,
		func(o runtime.Object) []string {
			return []string{o.(*topolvmv1.LogicalVolume).Status.VolumeID}
		})
	if err != nil {
		return nil, err
	}

	return &logicalVolumeService{Client: mgr.GetClient()}, nil
}

func (s *logicalVolumeService) CreateVolume(ctx context.Context, node string, name string, sizeGb int64, capabilities []*csi.VolumeCapability) (string, error) {
	logger.Info("k8s.CreateVolume called", "name", name, "node", node, "size_gb", sizeGb)
	s.mu.Lock()
	defer s.mu.Unlock()

	lv := &topolvmv1.LogicalVolume{
		TypeMeta: metav1.TypeMeta{
			Kind:       "LogicalVolume",
			APIVersion: "topolvm.cybozu.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: topolvmv1.LogicalVolumeSpec{
			Name:     name,
			NodeName: node,
			Size:     *resource.NewQuantity(sizeGb<<30, resource.BinarySI),
		},
	}

	existingLV := new(topolvmv1.LogicalVolume)
	err := s.Get(ctx, client.ObjectKey{Name: name}, existingLV)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return "", err
		}

		err := s.Create(ctx, lv)
		if err != nil {
			return "", err
		}
		logger.Info("created LogicalVolume CRD", "name", name)
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
		logger.Info("waiting for setting 'status.volumeID'", "name", name)
		select {
		case <-ctx.Done():
			return "", errors.New("timed out")
		case <-time.After(1 * time.Second):
		}

		var newLV topolvmv1.LogicalVolume
		err := s.Get(ctx, client.ObjectKey{Name: name}, &newLV)
		if err != nil {
			logger.Error(err, "failed to get LogicalVolume", "name", name)
			continue
		}
		if newLV.Status.VolumeID != "" {
			logger.Info("end k8s.LogicalVolume", "volume_id", newLV.Status.VolumeID)
			return newLV.Status.VolumeID, nil
		}
		if newLV.Status.Code != codes.OK {
			err := s.Delete(ctx, &newLV)
			if err != nil {
				// log this error but do not return this error, because newLV.Status.Message is more important
				logger.Error(err, "failed to delete LogicalVolume")
			}
			return "", status.Error(newLV.Status.Code, newLV.Status.Message)
		}
	}
}

func (s *logicalVolumeService) DeleteVolume(ctx context.Context, volumeID string) error {
	lvList := new(topolvmv1.LogicalVolumeList)
	err := s.List(ctx, lvList, client.MatchingFields{indexFieldVolumeID: volumeID})
	if err != nil {
		return err
	}
	if len(lvList.Items) == 0 {
		logger.Info("volume is not found", "volume_id", volumeID)
		return nil
	} else if len(lvList.Items) > 1 {
		return fmt.Errorf("multiple LogicalVolume is found for VolumeID %s", volumeID)
	}

	return s.Delete(ctx, &lvList.Items[0])
}

func (s *logicalVolumeService) getLogicalVolume(ctx context.Context, volumeID string) (*topolvmv1.LogicalVolume, error) {
	lvList := new(topolvmv1.LogicalVolumeList)
	err := s.List(ctx, lvList, client.MatchingFields{indexFieldVolumeID: volumeID})
	if err != nil {
		return nil, err
	}

	if len(lvList.Items) > 1 {
		return nil, errors.New("found multiple volumes with volumeID " + volumeID)
	}
	if len(lvList.Items) == 0 {
		return nil, driver.ErrVolumeNotFound
	}
	return &lvList.Items[0], nil
}

func (s *logicalVolumeService) VolumeExists(ctx context.Context, volumeID string) error {
	_, err := s.getLogicalVolume(ctx, volumeID)
	return err
}

func (s *logicalVolumeService) listNodes(ctx context.Context) (*corev1.NodeList, error) {
	nl := new(corev1.NodeList)
	err := s.List(ctx, nl)
	if err != nil {
		return nil, err
	}
	return nl, nil
}

func (s *logicalVolumeService) ExpandVolume(ctx context.Context, volumeID string, sizeGb int64) error {
	currentSize, err := s.GetCurrentSize(ctx, volumeID)
	if err != nil {
		return err
	}

	// If `status.currentSize` is not set, first set `status.currentSize` as the value of `spec.size`
	var cs int64
	if currentSize != nil {
		cs = *currentSize
	} else {
		// patch if currentSize is not nil
		lv, err := s.getLogicalVolume(ctx, volumeID)
		if err != nil {
			return err
		}

		lv2 := lv.DeepCopy()
		lv2.Status.CurrentSize = &lv2.Spec.Size

		patch := client.MergeFrom(lv)
		if err := s.Patch(ctx, lv2, patch); err != nil {
			logger.Error(err, "failed to patch .status.currentSize", "name", lv.Name)
			return err
		}
		currentSize2, err := s.GetCurrentSize(ctx, volumeID)
		if err != nil {
			return err
		}
		if currentSize2 == nil {
			return errors.New("should not be nil")
		}
		cs = *currentSize2
	}

	// Expand volume
	if sizeGb<<30-cs <= 0 {
		logger.Info("no need to extend volume", "requested", sizeGb<<30, "current", cs)
		return nil
	}

	lv, err := s.getLogicalVolume(ctx, volumeID)
	if err != nil {
		return err
	}
	targetNodeName := lv.Spec.NodeName
	node, err := s.getNode(ctx, targetNodeName)
	if err != nil {
		return err
	}
	cap := s.getNodeCapacity(*node)
	if cap < (sizeGb<<30 - cs) {
		return errors.New("not enough space")
	}

	lv2 := lv.DeepCopy()
	lv2.Spec.Size = *resource.NewQuantity(sizeGb<<30, resource.BinarySI)
	// TODO: handling codes and messages should be considered more.
	lv2.Status.Code = codes.OK
	lv2.Status.Message = ""
	patch := client.MergeFrom(lv)
	if err := s.Patch(ctx, lv2, patch); err != nil {
		logger.Error(err, "failed to patch .spec.size", "name", lv.Name)
		return err
	}

	// wait until topolvm-node extends the target volume
	for {
		lvname := lv.Name
		logger.Info("waiting for extending 'status.currentSize'", "name", lvname)
		select {
		case <-ctx.Done():
			return errors.New("timed out")
		case <-time.After(1 * time.Second):
		}

		var newLV topolvmv1.LogicalVolume
		err := s.Get(ctx, client.ObjectKey{Name: lvname}, &newLV)
		if err != nil {
			logger.Error(err, "failed to get LogicalVolume", "name", lvname)
			continue
		}
		if newLV.Status.CurrentSize == nil {
			return errors.New("status.currentSize should not be nil")
		}
		if newLV.Status.CurrentSize.Value() != newLV.Spec.Size.Value() {
			logger.Info("failed to match current size and requested size", "current", newLV.Status.CurrentSize.Value(), "requested", newLV.Spec.Size.Value())
			continue
		}

		if newLV.Status.Code != codes.OK {
			return status.Error(newLV.Status.Code, newLV.Status.Message)
		}
	}
	return nil
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
		if nodeNumber, ok := node.Labels[topolvm.TopologyNodeKey]; ok {
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

func (s *logicalVolumeService) getNode(ctx context.Context, name string) (*corev1.Node, error) {
	nl, err := s.listNodes(ctx)
	if err != nil {
		return nil, err
	}
	for _, node := range nl.Items {
		if node.Name == name {
			return &node, nil
		}
	}
	return nil, errors.New("node not found")
}

func (s *logicalVolumeService) GetCurrentSize(ctx context.Context, volumeID string) (*int64, error) {
	lv, err := s.getLogicalVolume(ctx, volumeID)
	if err != nil {
		return nil, err
	}

	cs := lv.Status.CurrentSize
	if cs == nil {
		return nil, nil
	}
	val := cs.Value()
	return &val, nil
}
