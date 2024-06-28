package driver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/topolvm/topolvm"
	v1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/internal/driver/internal/k8s"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var ctrlLogger = ctrl.Log.WithName("driver").WithName("controller")

var (
	ErrNoNegativeRequestBytes = errors.New("required capacity must not be negative")
	ErrNoNegativeLimitBytes   = errors.New("capacity limit must not be negative")
	ErrRequestedExceedsLimit  = errors.New("requested capacity exceeds limit capacity")
	ErrResultingRequestIsZero = errors.New("requested capacity is 0")
)

// ControllerServerSettings hold all settings that should be passed to the controller server.
type ControllerServerSettings struct {
	MinimumAllocationSettings `json:"allocation" ,yaml:"allocation"`
}

// NewControllerServer returns a new ControllerServer.
func NewControllerServer(mgr manager.Manager, settings ControllerServerSettings) (csi.ControllerServer, error) {
	lvService, err := k8s.NewLogicalVolumeService(mgr)
	if err != nil {
		return nil, err
	}

	return &controllerServer{
		lockByName:     NewLockWithID(),
		lockByVolumeID: NewLockWithID(),
		server: &controllerServerNoLocked{
			lvService:   lvService,
			nodeService: k8s.NewNodeService(mgr.GetClient()),
			settings:    settings,
		},
	}, nil
}

// This is a wrapper for controllerServerNoLocked to protect concurrent method call.
type controllerServer struct {
	csi.UnimplementedControllerServer

	// This protects server methods using a volume name.
	lockByName *LockByID
	// This protects server methods using a volume id.
	lockByVolumeID *LockByID
	server         *controllerServerNoLocked
}

func (s *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	s.lockByName.LockByID(req.GetName())
	defer s.lockByName.UnlockByID(req.GetName())

	return s.server.CreateVolume(ctx, req)
}

func (s *controllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	s.lockByVolumeID.LockByID(req.GetVolumeId())
	defer s.lockByVolumeID.UnlockByID(req.GetVolumeId())

	return s.server.DeleteVolume(ctx, req)
}

func (s *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	s.lockByVolumeID.LockByID(req.GetVolumeId())
	defer s.lockByVolumeID.UnlockByID(req.GetVolumeId())

	return s.server.ValidateVolumeCapabilities(ctx, req)
}

func (s *controllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	// This reads kube-apiserver only and even if reads dirty state, it is not harmless.
	// Therefore, it is unnecessary to take lock.
	return s.server.GetCapacity(ctx, req)
}

func (s *controllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	// This returns constants only, it is unnecessary to take lock.
	return s.server.ControllerGetCapabilities(ctx, req)
}

func (s *controllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	s.lockByName.LockByID(req.GetName())
	defer s.lockByName.UnlockByID(req.GetName())

	return s.server.CreateSnapshot(ctx, req)
}

func (s *controllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	s.lockByVolumeID.LockByID(req.GetSnapshotId())
	defer s.lockByVolumeID.UnlockByID(req.GetSnapshotId())

	return s.server.DeleteSnapshot(ctx, req)
}

func (s *controllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	s.lockByVolumeID.LockByID(req.GetVolumeId())
	defer s.lockByVolumeID.UnlockByID(req.GetVolumeId())

	return s.server.ControllerExpandVolume(ctx, req)
}

// controllerServerNoLocked implements csi.ControllerServer.
// It does not take any lock, gRPC calls may be interleaved.
// Therefore, must not use it directly.
type controllerServerNoLocked struct {
	csi.UnimplementedControllerServer

	lvService   *k8s.LogicalVolumeService
	nodeService *k8s.NodeService

	settings ControllerServerSettings
}

func (s controllerServerNoLocked) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	capabilities := req.GetVolumeCapabilities()
	source := req.GetVolumeContentSource()
	deviceClass := req.GetParameters()[topolvm.GetDeviceClassKey()]
	lvcreateOptionClass := req.GetParameters()[topolvm.GetLvcreateOptionClassKey()]

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
		// sourceID   string
		sourceName string
		sourceVol  *v1.LogicalVolume
		err        error
	)

	if capabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "no volume capabilities are provided")
	}

	required, limit := s.settings.MinimumAllocationSettings.MinMaxAllocationsFromSettings(
		req.GetCapacityRange().GetRequiredBytes(),
		req.GetCapacityRange().GetLimitBytes(),
		capabilities,
	)

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

	requestCapacityBytes, err := convertRequestCapacityBytes(required, limit)
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

		// check if the volume is equal or bigger than the source volume.
		sourceSizeBytes := sourceVol.Spec.Size.Value()
		if requestCapacityBytes < sourceSizeBytes {
			return nil, status.Error(codes.OutOfRange, "requested size is smaller than the size of the source")
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
			if capacity < requestCapacityBytes {
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

	volumeID, err := s.lvService.CreateVolume(ctx, node, deviceClass, lvcreateOptionClass, name, sourceName, requestCapacityBytes)
	if err != nil {
		_, ok := status.FromError(err)
		if !ok {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return nil, err
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: requestCapacityBytes,
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
func (s controllerServerNoLocked) validateContentSource(ctx context.Context, req *csi.CreateVolumeRequest) (*v1.LogicalVolume, string, error) {
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
func (s controllerServerNoLocked) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
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
func (s controllerServerNoLocked) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
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

// convertRequestCapacityBytes converts requestBytes and limitBytes to a valid capacity.
func convertRequestCapacityBytes(requestBytes, limitBytes int64) (int64, error) {
	if requestBytes < 0 {
		return 0, ErrNoNegativeRequestBytes
	}
	if limitBytes < 0 {
		return 0, ErrNoNegativeLimitBytes
	}

	if limitBytes != 0 && requestBytes > limitBytes {
		return 0, fmt.Errorf("%w: request=%d limit=%d", ErrRequestedExceedsLimit, requestBytes, limitBytes)
	}

	// if requestBytes is 0 and
	// 1. if limitBytes == 0, default to 1Gi
	// 2. if limitBytes >= 1Gi, default to 1Gi
	// 3. if limitBytes < 1Gi, default to rounded-down 4096 multiple of limitBytes.
	//    if rounded-down 4096 multiple of limitBytes == 0, return error
	if requestBytes == 0 {
		// if there is no limit or the limit is bigger or equal to the default, use the default
		if limitBytes == 0 || limitBytes >= topolvm.DefaultSize {
			return topolvm.DefaultSize, nil
		}

		roundedLimit := roundDown(limitBytes, topolvm.MinimumSectorSize)
		if roundedLimit == 0 {
			return 0, fmt.Errorf("%w, because it defaulted to the limit (%d) and was rounded down to the nearest sector size (%d). "+
				"specify the limit to be at least %d bytes", ErrResultingRequestIsZero, limitBytes, topolvm.MinimumSectorSize, topolvm.MinimumSectorSize)
		}

		return roundedLimit, nil
	}

	if requestBytes%topolvm.MinimumSectorSize != 0 {
		// round up to the nearest multiple of the sector size
		requestBytes = roundUp(requestBytes, topolvm.MinimumSectorSize)
		// after rounding up, we might overshoot the limit
		if limitBytes > 0 && requestBytes > limitBytes {
			return 0, fmt.Errorf(
				"%w, either specify a lower request or a higher limit (derived from %d sector size): request=%d limit=%d",
				ErrRequestedExceedsLimit, topolvm.MinimumSectorSize, requestBytes, limitBytes,
			)
		}
	}

	return requestBytes, nil
}

// roundUp rounds up the size to the nearest given multiple.
func roundUp(size int64, multiple int64) int64 {
	return (size + multiple - 1) / multiple * multiple
}

// roundDown rounds down the size to the nearest given multiple.
func roundDown(size int64, multiple int64) int64 {
	return size - size%multiple
}

func (s controllerServerNoLocked) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
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

func (s controllerServerNoLocked) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
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

func (s controllerServerNoLocked) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	topology := req.GetAccessibleTopology()
	capabilities := req.GetVolumeCapabilities()
	ctrlLogger.V(1).Info("GetCapacity called",
		"volume_capabilities", capabilities,
		"parameters", req.GetParameters(),
		"accessible_topology", topology)
	if capabilities != nil {
		ctrlLogger.V(1).Info("capability argument is not nil, but TopoLVM ignores it")
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

func (s controllerServerNoLocked) ControllerGetCapabilities(context.Context, *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
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

func (s controllerServerNoLocked) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	logger := ctrlLogger.WithValues("volumeID", volumeID,
		"required", req.GetCapacityRange().GetRequiredBytes(),
		"limit", req.GetCapacityRange().GetLimitBytes(),
		"num_secrets", len(req.GetSecrets()))

	logger.Info("ControllerExpandVolume called")

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume id is nil")
	}

	lv, err := s.lvService.GetVolume(ctx, volumeID)
	if err != nil {
		if errors.Is(err, k8s.ErrVolumeNotFound) {
			return nil, status.Errorf(codes.NotFound, "LogicalVolume for volume id %s is not found", volumeID)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	requestCapacityBytes, err := convertRequestCapacityBytes(
		req.GetCapacityRange().GetRequiredBytes(),
		req.GetCapacityRange().GetLimitBytes(),
	)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	currentSize := lv.Status.CurrentSize
	if currentSize == nil {
		// WA: since CurrentSize is added in v0.4.0, use Spec.Size if it is missing.
		currentSize = &lv.Spec.Size
	}

	if requestCapacityBytes <= currentSize.Value() {
		logger.Info("ControllerExpandVolume is waiting for node expansion to complete")
		// "NodeExpansionRequired" is still true because it is unknown
		// whether node expansion is completed or not.
		return &csi.ControllerExpandVolumeResponse{
			CapacityBytes:         currentSize.Value(),
			NodeExpansionRequired: true,
		}, nil
	}
	capacity, err := s.nodeService.GetCapacityByName(ctx, lv.Spec.NodeName, lv.Spec.DeviceClass)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if capacity < (requestCapacityBytes - currentSize.Value()) {
		return nil, status.Error(codes.Internal, "not enough space")
	}

	logger.Info("ControllerExpandVolume triggering lvService.ExpandVolume")
	err = s.lvService.ExpandVolume(ctx, volumeID, requestCapacityBytes)
	if err != nil {
		_, ok := status.FromError(err)
		if !ok {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return nil, err
	}

	logger.Info("ControllerExpandVolume has succeeded")

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         requestCapacityBytes,
		NodeExpansionRequired: false,
	}, nil
}
