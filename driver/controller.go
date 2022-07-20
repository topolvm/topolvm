package driver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/topolvm/topolvm"
	v1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/csi"
	"github.com/topolvm/topolvm/driver/k8s"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	ctrl "sigs.k8s.io/controller-runtime"
)

var ctrlLogger = ctrl.Log.WithName("driver").WithName("controller")

// NewControllerService returns a new ControllerServer.
func NewControllerService(lvService *k8s.LogicalVolumeService, nodeService *k8s.NodeService) csi.ControllerServer {
	return &controllerService{lvService: lvService, nodeService: nodeService}
}

type controllerService struct {
	csi.UnimplementedControllerServer

	lvService   *k8s.LogicalVolumeService
	nodeService *k8s.NodeService
}

func (s controllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	capabilities := req.GetVolumeCapabilities()
	source := req.GetVolumeContentSource()
	deviceClass := req.GetParameters()[topolvm.GetDeviceClassKey()]

	ctrlLogger.Info("CreateVolume called",
		"name", req.GetName(),
		"device_class", deviceClass,
		"required", req.GetCapacityRange().GetRequiredBytes(),
		"limit", req.GetCapacityRange().GetLimitBytes(),
		"parameters", req.GetParameters(),
		"num_secrets", len(req.GetSecrets()),
		"capabilities", capabilities,
		"content_source", source,
		"accessibility_requirements", req.GetAccessibilityRequirements().String())

	var (
		//sourceID   string
		sourceName string
		sourceVol  *v1.LogicalVolume
		err        error
	)

	if capabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "no volume capabilities are provided")
	}

	// check required volume capabilities
	for _, capability := range capabilities {
		if block := capability.GetBlock(); block != nil {
			ctrlLogger.Info("CreateVolume specifies volume capability", "access_type", "block")
		} else if mount := capability.GetMount(); mount != nil {
			ctrlLogger.Info("CreateVolume specifies volume capability",
				"access_type", "mount",
				"fs_type", mount.GetFsType(),
				"flags", mount.GetMountFlags())
		} else {
			return nil, status.Error(codes.InvalidArgument, "unknown or empty access_type")
		}

		if mode := capability.GetAccessMode(); mode != nil {
			modeName := csi.VolumeCapability_AccessMode_Mode_name[int32(mode.GetMode())]
			ctrlLogger.Info("CreateVolume specifies volume capability", "access_mode", modeName)
			// we only support SINGLE_NODE_WRITER
			switch mode.GetMode() {
			case csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER:
			default:
				return nil, status.Errorf(codes.InvalidArgument, "unsupported access mode: %s", modeName)
			}
		}
	}

	requestGb, err := convertRequestCapacity(req.GetCapacityRange().GetRequiredBytes(), req.GetCapacityRange().GetLimitBytes())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// check if the create volume request has a data source
	if source != nil {
		// get the source volumeID/snapshotID if exists
		sourceVol, _, err = s.validateContentSource(ctx, req)
		if err != nil {
			return nil, err
		}
		// check if the volume has the same size as the source volume.
		// TODO (Yuggupta27): Allow user to create a volume with more size than that of the source volume.
		sourceSizeGb := sourceVol.Spec.Size.Value() >> 30
		if sourceSizeGb != requestGb {
			return nil, status.Error(codes.OutOfRange, "requested size does not match the size of the source")
		}
		// If a volume has a source, it has to provisioned on the same node and device class as the source volume.

		if deviceClass != sourceVol.Spec.DeviceClass {
			return nil, status.Error(codes.InvalidArgument, "device class mismatch. Snapshots should be created with the same device class as the source.")
		}
		deviceClass = sourceVol.Spec.DeviceClass
		sourceName = sourceVol.Spec.Name
	}

	// process topology
	var node string
	requirements := req.GetAccessibilityRequirements()

	if source != nil {
		if requirements == nil {
			// In CSI spec, controllers are required that they response OK even if accessibility_requirements field is nil.
			// So we must create volume, and must not return error response in this case.
			// - https://github.com/container-storage-interface/spec/blob/release-1.1/spec.md#createvolume
			// - https://github.com/kubernetes-csi/csi-test/blob/6738ab2206eac88874f0a3ede59b40f680f59f43/pkg/sanity/controller.go#L404-L428
			ctrlLogger.Info("decide node because accessibility_requirements not found")
			// the snapshot must be created on the same node as the source
			node = sourceVol.Spec.NodeName
		} else {
			sourceNode := sourceVol.Spec.NodeName
			for _, topo := range requirements.Preferred {
				if v, ok := topo.GetSegments()[topolvm.GetTopologyNodeKey()]; ok {
					if v == sourceNode {
						node = v
						break
					}
				}
			}
			if node == "" {
				for _, topo := range requirements.Requisite {
					if v, ok := topo.GetSegments()[topolvm.GetTopologyNodeKey()]; ok {
						if v == sourceNode {
							node = v
							break
						}
					}
				}
			}
			if node == "" {
				return nil, status.Errorf(codes.InvalidArgument, "cannot find source volume's node '%s' in accessibility_requirements", sourceNode)
			}
		}
	} else {
		if requirements == nil {
			// In CSI spec, controllers are required that they response OK even if accessibility_requirements field is nil.
			// So we must create volume, and must not return error response in this case.
			// - https://github.com/container-storage-interface/spec/blob/release-1.1/spec.md#createvolume
			// - https://github.com/kubernetes-csi/csi-test/blob/6738ab2206eac88874f0a3ede59b40f680f59f43/pkg/sanity/controller.go#L404-L428
			ctrlLogger.Info("decide node because accessibility_requirements not found")
			nodeName, capacity, err := s.nodeService.GetMaxCapacity(ctx, deviceClass)

			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to get max capacity node %v", err)
			}
			if nodeName == "" {
				return nil, status.Error(codes.Internal, "can not find any node")
			}
			if capacity < (requestGb << 30) {
				return nil, status.Errorf(codes.ResourceExhausted, "can not find enough volume space %d", capacity)
			}
			node = nodeName
		} else {
			for _, topo := range requirements.Preferred {
				if v, ok := topo.GetSegments()[topolvm.GetTopologyNodeKey()]; ok {
					node = v
					break
				}
			}
			if node == "" {
				for _, topo := range requirements.Requisite {
					if v, ok := topo.GetSegments()[topolvm.GetTopologyNodeKey()]; ok {
						node = v
						break
					}
				}
			}
			if node == "" {
				return nil, status.Errorf(codes.InvalidArgument, "cannot find key '%s' in accessibility_requirements", topolvm.GetTopologyNodeKey())
			}
		}
	}

	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid name")
	}

	name = strings.ToLower(name)

	volumeID, err := s.lvService.CreateVolume(ctx, node, deviceClass, name, sourceName, requestGb)
	if err != nil {
		_, ok := status.FromError(err)
		if !ok {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return nil, err
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: requestGb << 30,
			VolumeId:      volumeID,
			ContentSource: source,
			AccessibleTopology: []*csi.Topology{
				{
					Segments: map[string]string{topolvm.GetTopologyNodeKey(): node},
				},
			},
		},
	}, nil
}

// validateContentSource checks if the request has a data source and returns source volume information.
func (s controllerService) validateContentSource(ctx context.Context, req *csi.CreateVolumeRequest) (*v1.LogicalVolume, string, error) {
	volumeSource := req.VolumeContentSource

	switch volumeSource.Type.(type) {
	case *csi.VolumeContentSource_Snapshot:
		snapshotID := req.VolumeContentSource.GetSnapshot().GetSnapshotId()
		if snapshotID == "" {
			return nil, "", status.Error(codes.NotFound, "Snapshot ID cannot be empty")
		}
		snapshotVol, err := s.lvService.GetVolume(ctx, snapshotID)
		if err != nil {
			if errors.Is(err, k8s.ErrVolumeNotFound) {
				return nil, "", status.Error(codes.NotFound, "failed to find source snapshot")
			}
			return nil, "", status.Error(codes.Internal, err.Error())
		}
		return snapshotVol, snapshotID, nil

	case *csi.VolumeContentSource_Volume:
		volumeID := req.VolumeContentSource.GetVolume().GetVolumeId()
		if volumeID == "" {
			return nil, "", status.Error(codes.NotFound, "Volume ID cannot be empty")
		}
		logicalVol, err := s.lvService.GetVolume(ctx, volumeID)
		if err != nil {
			if errors.Is(err, k8s.ErrVolumeNotFound) {
				return nil, "", status.Error(codes.NotFound, "failed to find source volume")
			}
			return nil, "", status.Error(codes.Internal, err.Error())
		}

		return logicalVol, volumeID, nil
	}

	return nil, "", status.Errorf(codes.InvalidArgument, "invalid volume source %v", volumeSource)
}

// CreateSnapshot creates a logical volume snapshot.
func (s controllerService) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	// Since the kubernetes snapshots are Read-Only, we set accessType as 'ro' to activate thin-snapshots as read-only volumes
	accessType := "ro"

	ctrlLogger.Info("CreateSnapshot called",
		"name", req.GetName(),
		"source_volume_id", req.GetSourceVolumeId(),
		"parameters", req.GetParameters(),
		"num_secrets", len(req.GetSecrets()))

	if req.GetSourceVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing source volume id")
	}

	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing name")
	}

	name := strings.ToLower(req.GetName())
	sourceVolID := req.GetSourceVolumeId()
	sourceVol, err := s.lvService.GetVolume(ctx, sourceVolID)
	if err != nil {
		if errors.Is(err, k8s.ErrVolumeNotFound) {
			return nil, status.Error(codes.NotFound, "failed to find source volumes")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	snapTimeStamp := &timestamp.Timestamp{
		Seconds: time.Now().Unix(),
		Nanos:   0,
	}
	// the snapshots are required to be created in the same node and device class as the source volume.
	node := sourceVol.Spec.NodeName
	deviceClass := sourceVol.Spec.DeviceClass
	size := sourceVol.Spec.Size
	sourceVolName := sourceVol.Spec.Name
	snapshotID, err := s.lvService.CreateSnapshot(ctx, node, deviceClass, sourceVolName, name, accessType, size)
	if err != nil {
		_, ok := status.FromError(err)
		if !ok {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return nil, err
	}

	return &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SnapshotId:     snapshotID,
			SourceVolumeId: sourceVolID,
			SizeBytes:      sourceVol.Spec.Size.Value(),
			CreationTime:   snapTimeStamp,
			ReadyToUse:     true,
		},
	}, nil
}

// DeleteSnapshot deletes an existing logical volume snapshot.
func (s controllerService) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	ctrlLogger.Info("DeleteSnapshot called",
		"snapshot_id", req.GetSnapshotId(),
		"num_secrets", len(req.GetSecrets()))

	if req.GetSnapshotId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing snapshot id")
	}

	if err := s.lvService.DeleteVolume(ctx, req.GetSnapshotId()); err != nil {
		ctrlLogger.Error(err, "DeleteSnapshot failed", "snapshot_id", req.GetSnapshotId())
		_, ok := status.FromError(err)
		if !ok {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return nil, err
	}

	return &csi.DeleteSnapshotResponse{}, nil
}

func convertRequestCapacity(requestBytes, limitBytes int64) (int64, error) {
	if requestBytes < 0 {
		return 0, errors.New("required capacity must not be negative")
	}
	if limitBytes < 0 {
		return 0, errors.New("capacity limit must not be negative")
	}

	if limitBytes != 0 && requestBytes > limitBytes {
		return 0, fmt.Errorf(
			"requested capacity exceeds limit capacity: request=%d limit=%d", requestBytes, limitBytes,
		)
	}

	if requestBytes == 0 {
		return 1, nil
	}
	return (requestBytes-1)>>30 + 1, nil
}

func (s controllerService) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	ctrlLogger.Info("DeleteVolume called",
		"volume_id", req.GetVolumeId(),
		"num_secrets", len(req.GetSecrets()))
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume_id is not provided")
	}

	err := s.lvService.DeleteVolume(ctx, req.GetVolumeId())
	if err != nil {
		ctrlLogger.Error(err, "DeleteVolume failed", "volume_id", req.GetVolumeId())
		_, ok := status.FromError(err)
		if !ok {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return nil, err
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (s controllerService) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	ctrlLogger.Info("ValidateVolumeCapabilities called",
		"volume_id", req.GetVolumeId(),
		"volume_context", req.GetVolumeContext(),
		"volume_capabilities", req.GetVolumeCapabilities(),
		"parameters", req.GetParameters(),
		"num_secrets", len(req.GetSecrets()))

	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume id is nil")
	}
	if len(req.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume capabilities are empty")
	}

	_, err := s.lvService.GetVolume(ctx, req.GetVolumeId())
	if err != nil {
		if err == k8s.ErrVolumeNotFound {
			return nil, status.Errorf(codes.NotFound, "LogicalVolume for volume id %s is not found", req.GetVolumeId())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Since TopoLVM does not provide means to pre-provision volumes,
	// any existing volume is valid.
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeContext:      req.GetVolumeContext(),
			VolumeCapabilities: req.GetVolumeCapabilities(),
			Parameters:         req.GetParameters(),
		},
	}, nil
}

func (s controllerService) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	topology := req.GetAccessibleTopology()
	capabilities := req.GetVolumeCapabilities()
	ctrlLogger.Info("GetCapacity called",
		"volume_capabilities", capabilities,
		"parameters", req.GetParameters(),
		"accessible_topology", topology)
	if capabilities != nil {
		ctrlLogger.Info("capability argument is not nil, but TopoLVM ignores it")
	}

	deviceClass := req.GetParameters()[topolvm.GetDeviceClassKey()]

	var capacity int64
	switch topology {
	case nil:
		var err error
		capacity, err = s.nodeService.GetTotalCapacity(ctx, deviceClass)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	default:
		v, ok := topology.Segments[topolvm.GetTopologyNodeKey()]
		if !ok {
			err := fmt.Errorf("%s is not found in req.AccessibleTopology", topolvm.GetTopologyNodeKey())
			ctrlLogger.Error(err, "target node key is not found")
			return &csi.GetCapacityResponse{AvailableCapacity: 0}, nil
		}
		var err error
		capacity, err = s.nodeService.GetCapacityByTopologyLabel(ctx, v, deviceClass)
		switch err {
		case k8s.ErrNodeNotFound:
			ctrlLogger.Info("target is not found", "accessible_topology", req.AccessibleTopology)
			return &csi.GetCapacityResponse{AvailableCapacity: 0}, nil
		case k8s.ErrDeviceClassNotFound:
			ctrlLogger.Info("target device class is not found on the specified node", "accessible_topology", req.AccessibleTopology, "device-class", deviceClass)
			return &csi.GetCapacityResponse{AvailableCapacity: 0}, nil
		case nil:
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &csi.GetCapacityResponse{
		AvailableCapacity: capacity,
	}, nil
}

func (s controllerService) ControllerGetCapabilities(context.Context, *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	capabilities := []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_CLONE_VOLUME,
		csi.ControllerServiceCapability_RPC_GET_CAPACITY,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
	}

	csiCaps := make([]*csi.ControllerServiceCapability, len(capabilities))
	for i, capability := range capabilities {
		csiCaps[i] = &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: capability,
				},
			},
		}
	}

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: csiCaps,
	}, nil
}

func (s controllerService) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	ctrlLogger.Info("ControllerExpandVolume called",
		"volumeID", volumeID,
		"required", req.GetCapacityRange().GetRequiredBytes(),
		"limit", req.GetCapacityRange().GetLimitBytes(),
		"num_secrets", len(req.GetSecrets()))

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume id is nil")
	}

	lv, err := s.lvService.GetVolume(ctx, volumeID)
	if err != nil {
		if err == k8s.ErrVolumeNotFound {
			return nil, status.Errorf(codes.NotFound, "LogicalVolume for volume id %s is not found", volumeID)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	requestGb, err := convertRequestCapacity(req.GetCapacityRange().GetRequiredBytes(), req.GetCapacityRange().GetLimitBytes())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	currentSize := lv.Status.CurrentSize
	if currentSize == nil {
		// fill currentGb for old volume created in v0.3.0 or before.
		err := s.lvService.UpdateCurrentSize(ctx, volumeID, &lv.Spec.Size)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		currentSize = &lv.Spec.Size
	}

	currentGb := currentSize.Value() >> 30
	if requestGb <= currentGb {
		// "NodeExpansionRequired" is still true because it is unknown
		// whether node expansion is completed or not.
		return &csi.ControllerExpandVolumeResponse{
			CapacityBytes:         currentGb << 30,
			NodeExpansionRequired: true,
		}, nil
	}
	capacity, err := s.nodeService.GetCapacityByName(ctx, lv.Spec.NodeName, lv.Spec.DeviceClass)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if capacity < (requestGb<<30 - currentGb<<30) {
		return nil, status.Error(codes.Internal, "not enough space")
	}

	err = s.lvService.ExpandVolume(ctx, volumeID, requestGb)
	if err != nil {
		_, ok := status.FromError(err)
		if !ok {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return nil, err
	}
	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         requestGb << 30,
		NodeExpansionRequired: true,
	}, nil
}
