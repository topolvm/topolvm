package driver

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	topolvmv1 "github.com/cybozu-go/topolvm/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

// LogicalVolumeService represents service for LogicalVolume.
type LogicalVolumeService struct {
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
func NewLogicalVolumeService(mgr manager.Manager) (*LogicalVolumeService, error) {
	err := mgr.GetFieldIndexer().IndexField(&topolvmv1.LogicalVolume{}, indexFieldVolumeID,
		func(o runtime.Object) []string {
			return []string{o.(*topolvmv1.LogicalVolume).Status.VolumeID}
		})
	if err != nil {
		return nil, err
	}

	return &LogicalVolumeService{Client: mgr.GetClient()}, nil
}

// Volume represents abstracted Volume.
type Volume struct {
	node      string
	name      string
	volumeID  string
	requestGb int64
	currentGb *int64
}

// GetCurrentGb returns value of currentGb and ok flag.
func (v *Volume) GetCurrentGb() (int64, bool) {
	if v.currentGb == nil {
		return 0, false
	}
	return *v.currentGb, true
}

// SetCurrentGb sets currentGb.
func (v *Volume) SetCurrentGb(n int64) {
	v.currentGb = &n
}

// CreateVolume creates volume
func (s *LogicalVolumeService) CreateVolume(ctx context.Context, vol *Volume) (string, error) {
	logger.Info("k8s.CreateVolume called", "name", vol.name, "node", vol.node, "size_gb", vol.requestGb)
	s.mu.Lock()
	defer s.mu.Unlock()

	lv := &topolvmv1.LogicalVolume{
		TypeMeta: metav1.TypeMeta{
			Kind:       "LogicalVolume",
			APIVersion: "topolvm.cybozu.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: vol.name,
		},
		Spec: topolvmv1.LogicalVolumeSpec{
			Name:     vol.name,
			NodeName: vol.node,
			Size:     *resource.NewQuantity(vol.requestGb<<30, resource.BinarySI),
		},
	}

	existingLV := new(topolvmv1.LogicalVolume)
	err := s.Get(ctx, client.ObjectKey{Name: vol.name}, existingLV)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return "", err
		}

		err := s.Create(ctx, lv)
		if err != nil {
			return "", err
		}
		logger.Info("created LogicalVolume CRD", "name", vol.name)
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
		logger.Info("waiting for setting 'status.volumeID'", "name", vol.name)
		select {
		case <-ctx.Done():
			return "", errors.New("timed out")
		case <-time.After(1 * time.Second):
		}

		var newLV topolvmv1.LogicalVolume
		err := s.Get(ctx, client.ObjectKey{Name: vol.name}, &newLV)
		if err != nil {
			logger.Error(err, "failed to get LogicalVolume", "name", vol.name)
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

// DeleteVolume deletes volume
func (s *LogicalVolumeService) DeleteVolume(ctx context.Context, volumeID string) error {
	logger.Info("k8s.DeleteVolume called", "volumeID", volumeID)
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

// VolumeExists returns non-error if the volume exists
func (s *LogicalVolumeService) VolumeExists(ctx context.Context, volumeID string) error {
	_, err := s.getLogicalVolume(ctx, volumeID)
	return err
}

// ExpandVolume expands volume
func (s *LogicalVolumeService) ExpandVolume(ctx context.Context, vol *Volume) error {
	logger.Info("k8s.ExpandVolume called", "name", vol.name, "node", vol.node, "size_gb", vol.requestGb)
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.UpdateVolumeSizes(ctx, vol)
	if err != nil {
		return err
	}

	// wait until topolvm-node expands the target volume
	lvName := vol.name
	for {
		logger.Info("waiting for update of 'status.currentSize'", "name", lvName)
		select {
		case <-ctx.Done():
			return errors.New("timed out")
		case <-time.After(1 * time.Second):
		}

		var changedLV topolvmv1.LogicalVolume
		err := s.Get(ctx, client.ObjectKey{Name: lvName}, &changedLV)
		if err != nil {
			logger.Error(err, "failed to get LogicalVolume", "name", lvName)
			continue
		}
		if changedLV.Status.CurrentSize == nil {
			return errors.New("status.currentSize should not be nil")
		}
		if changedLV.Status.CurrentSize.Value() != changedLV.Spec.Size.Value() {
			logger.Info("failed to match current size and requested size", "current", changedLV.Status.CurrentSize.Value(), "requested", changedLV.Spec.Size.Value())
			continue
		}

		if changedLV.Status.Code != codes.OK {
			return status.Error(changedLV.Status.Code, changedLV.Status.Message)
		}

		return nil
	}
}

// GetVolume returns volume specified by volume ID.
func (s *LogicalVolumeService) GetVolume(ctx context.Context, volumeID string) (*Volume, error) {
	lv, err := s.getLogicalVolume(ctx, volumeID)
	if err != nil {
		return nil, err
	}

	vol := &Volume{
		name:      lv.GetName(),
		node:      lv.Spec.NodeName,
		volumeID:  lv.Status.VolumeID,
		requestGb: lv.Spec.Size.Value() >> 30,
	}

	if lv.Status.CurrentSize != nil {
		vol.SetCurrentGb(lv.Status.CurrentSize.Value() >> 30)
	}

	return vol, nil
}

func (s *LogicalVolumeService) getLogicalVolume(ctx context.Context, volumeID string) (*topolvmv1.LogicalVolume, error) {
	lvList := new(topolvmv1.LogicalVolumeList)
	err := s.List(ctx, lvList, client.MatchingFields{indexFieldVolumeID: volumeID})
	if err != nil {
		return nil, err
	}

	if len(lvList.Items) == 0 {
		return nil, ErrVolumeNotFound
	} else if len(lvList.Items) > 1 {
		return nil, fmt.Errorf("multiple LogicalVolume is found for VolumeID %s", volumeID)
	}
	return &lvList.Items[0], nil
}

// UpdateVolumeSizes updates LogicalVolume sizes.
func (s *LogicalVolumeService) UpdateVolumeSizes(ctx context.Context, vol *Volume) error {
	lv, err := s.getLogicalVolume(ctx, vol.volumeID)
	if err != nil {
		return err
	}

	lv2 := lv.DeepCopy()
	lv2.Spec.Size = *resource.NewQuantity(vol.requestGb<<30, resource.BinarySI)
	if currentGb, ok := vol.GetCurrentGb(); ok {
		lv2.Status.CurrentSize = resource.NewQuantity(currentGb<<30, resource.BinarySI)
	}
	patch := client.MergeFrom(lv)
	if err := s.Patch(ctx, lv2, patch); err != nil {
		logger.Error(err, "failed to patch LogicalVolume", "name", lv.Name)
		return err
	}
	return nil
}
