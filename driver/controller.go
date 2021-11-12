package driver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/topolvm/topolvm"
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
	deviceClass := req.GetParameters()[topolvm.DeviceClassKey]

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

	var parentId string
	var err error

	if source != nil {
		parentId, err = checkContentSource(req)
		if err != nil {
			return nil, err
		}
	}

	if capabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "no volume capabilities are provided")
	}

	// check required volume capabilities
	var volumeMode, fsType string
	for _, capability := range capabilities {
		if block := capability.GetBlock(); block != nil {
			volumeMode = "Block"
			ctrlLogger.Info("CreateVolume specifies volume capability", "access_type", "block")
		} else if mount := capability.GetMount(); mount != nil {
			volumeMode = "Filesystem"
			fsType = mount.GetFsType()
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

	// process topology
	var node string
	requirements := req.GetAccessibilityRequirements()
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
			if v, ok := topo.GetSegments()[topolvm.TopologyNodeKey]; ok {
				node = v
				break
			}
		}
		if node == "" {
			for _, topo := range requirements.Requisite {
				if v, ok := topo.GetSegments()[topolvm.TopologyNodeKey]; ok {
					node = v
					break
				}
			}
		}
		if node == "" {
			return nil, status.Errorf(codes.InvalidArgument, "cannot find key '%s' in accessibility_requirements", topolvm.TopologyNodeKey)
		}
	}

	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid name")
	}

	name = strings.ToLower(name)

	volumeID, err := s.lvService.CreateVolume(ctx, node, deviceClass, name, requestGb, parentId, volumeMode, fsType)
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
			ContentSource: req.GetVolumeContentSource(),
			AccessibleTopology: []*csi.Topology{
				{
					Segments: map[string]string{topolvm.TopologyNodeKey: node},
				},
			},
		},
	}, nil
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

	deviceClass := req.GetParameters()[topolvm.DeviceClassKey]

	var capacity int64
	switch topology {
	case nil:
		var err error
		capacity, err = s.nodeService.GetTotalCapacity(ctx, deviceClass)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	default:
		v, ok := topology.Segments[topolvm.TopologyNodeKey]
		if !ok {
			err := fmt.Errorf("%s is not found in req.AccessibleTopology", topolvm.TopologyNodeKey)
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

func (s controllerService) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	ctrlLogger.Info("CreateSnapshot called",
		"name", req.Name,
		"SourceVolumeId", req.SourceVolumeId)

	snapName := req.GetName()
	sourceVolID := req.GetSourceVolumeId()

	// validate snapshot request
	if req.Name == "" || req.SourceVolumeId == "" {
		return nil, status.Errorf(codes.InvalidArgument,
			"CreateSnapshot error invalid request %s: %s", snapName, sourceVolID)
	}

	// Get Source Volume
	sourceVolume, err := s.lvService.GetVolume(ctx, sourceVolID)
	if err != nil {
		return nil, err
	}

	snapTimeStamp := &timestamp.Timestamp{
		Seconds: time.Now().Unix(),
		Nanos:   0,
	}

	snapName = strings.ToLower(snapName)
	// Create snapshot lv
	snapID, err := s.lvService.CreateSnapshot(ctx, snapName, sourceVolume)
	if err != nil {
		return nil, err
	}

	return &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SizeBytes:      sourceVolume.Spec.Size.Value(),
			SnapshotId:     snapID,
			SourceVolumeId: req.SourceVolumeId,
			CreationTime:   snapTimeStamp,
			ReadyToUse:     err == nil,
		},
	}, err
}

func (s controllerService) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	ctrlLogger.Info("DeleteSnapshot called",
		"snapshot_id", req.GetSnapshotId(),
		"num_secrets", len(req.GetSecrets()))
	if len(req.GetSnapshotId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume ID is not provided")
	}

	err := s.lvService.DeleteVolume(ctx, req.GetSnapshotId())
	if err != nil {
		ctrlLogger.Error(err, "DeleteSnapshot failed", "snapshot IdD", req.GetSnapshotId())
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.DeleteSnapshotResponse{}, nil
}

func checkContentSource(req *csi.CreateVolumeRequest) (string, error) {
	volumeSource := req.VolumeContentSource

	switch volumeSource.Type.(type) {
	case *csi.VolumeContentSource_Snapshot:
		snapshot := req.VolumeContentSource.GetSnapshot()
		if snapshot == nil {
			return "", status.Error(codes.NotFound, "volume Snapshot cannot be empty")
		}
		snapshotID := snapshot.GetSnapshotId()
		if snapshotID == "" {
			return "", status.Errorf(codes.NotFound, "volume Snapshot ID cannot be empty")
		}
		return snapshotID, nil

	case *csi.VolumeContentSource_Volume:
		return "", status.Error(codes.InvalidArgument, "volume_content_source volume not supported")
	}

	return "", status.Errorf(codes.InvalidArgument, "not a proper volume source %v", volumeSource)
}
