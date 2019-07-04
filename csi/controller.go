package csi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/topolvm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrVolumeNotFound is error message when VolumeID is not found
var ErrVolumeNotFound = errors.New("VolumeID is not found")

// NewControllerService returns a new ControllerServer.
func NewControllerService(service LogicalVolumeService) ControllerServer {
	return &controllerService{service: service}
}

type controllerService struct {
	service LogicalVolumeService
}

func (s controllerService) CreateVolume(ctx context.Context, req *CreateVolumeRequest) (*CreateVolumeResponse, error) {
	capabilities := req.GetVolumeCapabilities()
	source := req.GetVolumeContentSource()

	log.Info("CreateVolume called", map[string]interface{}{
		"name":                       req.GetName(),
		"required":                   req.GetCapacityRange().GetRequiredBytes(),
		"limit":                      req.GetCapacityRange().GetLimitBytes(),
		"parameters":                 req.GetParameters(),
		"num_secrets":                len(req.GetSecrets()),
		"capabilities":               capabilities,
		"content_source":             source,
		"accessibility_requirements": req.GetAccessibilityRequirements().String(),
	})

	if source != nil {
		return nil, status.Error(codes.InvalidArgument, "volume_content_source not supported")
	}
	if capabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "no volume capabilities are provided")
	}

	// check required volume capabilities
	for _, capability := range capabilities {
		if block := capability.GetBlock(); block != nil {
			log.Info("CreateVolume specifies volume capability", map[string]interface{}{
				"access_type": "block",
			})
		} else if mount := capability.GetMount(); mount != nil {
			log.Info("CreateVolume specifies volume capability", map[string]interface{}{
				"access_type": "mount",
				"fs_type":     mount.GetFsType(),
				"flags":       mount.GetMountFlags(),
			})
		} else {
			return nil, status.Error(codes.InvalidArgument, "unknown or empty access_type")
		}

		if mode := capability.GetAccessMode(); mode != nil {
			modeName := VolumeCapability_AccessMode_Mode_name[int32(mode.GetMode())]
			log.Info("CreateVolume specifies volume capability", map[string]interface{}{
				"access_mode": modeName,
			})
			// we only support SINGLE_NODE_WRITER
			if mode.GetMode() != VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
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
		log.Info("decide node because accessibility_requirements not found", nil)
		nodeName, capacity, err := s.service.GetMaxCapacity(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get max capacity node %v", err)
		}
		if nodeName == "" {
			return nil, status.Error(codes.Internal, "can not find any node")
		}
		if capacity < (requestGb<<30) {
			return nil, status.Errorf(codes.Internal, "can not find enough volume space %d", capacity)
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

	volumeID, err := s.service.CreateVolume(ctx, node, name, requestGb, capabilities)
	if err != nil {
		_, ok := status.FromError(err)
		if !ok {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return nil, err
	}

	return &CreateVolumeResponse{
		Volume: &Volume{
			CapacityBytes: requestGb << 30,
			VolumeId:      volumeID,
			AccessibleTopology: []*Topology{
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

func (s controllerService) DeleteVolume(ctx context.Context, req *DeleteVolumeRequest) (*DeleteVolumeResponse, error) {
	log.Info("DeleteVolume called", map[string]interface{}{
		"volume_id":   req.GetVolumeId(),
		"num_secrets": len(req.GetSecrets()),
	})
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume_id is not provided")
	}

	err := s.service.DeleteVolume(ctx, req.GetVolumeId())
	if err != nil {
		log.Error("DeleteVolume failed", map[string]interface{}{
			"volume_id": req.GetVolumeId(),
			"error":     err.Error(),
		})
		_, ok := status.FromError(err)
		if !ok {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return nil, err
	}

	return &DeleteVolumeResponse{}, nil
}

func (s controllerService) ControllerPublishVolume(context.Context, *ControllerPublishVolumeRequest) (*ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ControllerPublishVolume not implemented")
}

func (s controllerService) ControllerUnpublishVolume(context.Context, *ControllerUnpublishVolumeRequest) (*ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ControllerUnpublishVolume not implemented")
}

func (s controllerService) ValidateVolumeCapabilities(ctx context.Context, req *ValidateVolumeCapabilitiesRequest) (*ValidateVolumeCapabilitiesResponse, error) {
	log.Info("ValidateVolumeCapabilities called", map[string]interface{}{
		"volume_id":           req.GetVolumeId(),
		"volume_context":      req.GetVolumeContext(),
		"volume_capabilities": req.GetVolumeCapabilities(),
		"parameters":          req.GetParameters(),
		"num_secrets":         len(req.GetSecrets()),
	})

	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume id is nil")
	}
	if len(req.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume capabilities are empty")
	}

	isValid, err := s.service.ValidateVolumeCapabilities(ctx, req.GetVolumeId(), req.GetVolumeCapabilities())
	if err != nil && err == ErrVolumeNotFound {
		return nil, status.Errorf(codes.NotFound, "LogicalVolume for volume id %s is not found", req.GetVolumeId())
	} else if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var confirmed *ValidateVolumeCapabilitiesResponse_Confirmed
	if isValid {
		confirmed = &ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.GetVolumeCapabilities(),
		}
	}
	return &ValidateVolumeCapabilitiesResponse{
		Confirmed: confirmed,
	}, nil
}

func (s controllerService) ListVolumes(ctx context.Context, req *ListVolumesRequest) (*ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ListVolumes not implemented")
}

func (s controllerService) GetCapacity(ctx context.Context, req *GetCapacityRequest) (*GetCapacityResponse, error) {
	topology := req.GetAccessibleTopology()
	capabilities := req.GetVolumeCapabilities()
	log.Info("GetCapacity called", map[string]interface{}{
		"volume_capabilities": capabilities,
		"parameters":          req.GetParameters(),
		"accessible_topology": topology,
	})
	if capabilities != nil {
		log.Warn("capability argument is not nil, but csi controller plugin ignored this argument", map[string]interface{}{})
	}

	capacity := int64(0)
	switch topology {
	case nil:
		var err error
		capacity, err = s.service.GetCapacity(ctx, "")
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	default:
		requestNodeNumber, ok := topology.Segments[topolvm.TopologyNodeKey]
		if !ok {
			return nil, status.Errorf(codes.Internal, "%s is not found in req.AccessibleTopology", topolvm.TopologyNodeKey)
		}
		var err error
		capacity, err = s.service.GetCapacity(ctx, requestNodeNumber)
		if err != nil {
			log.Info("target is not found", map[string]interface{}{
				"accessible_topology": req.AccessibleTopology,
			})
			return &GetCapacityResponse{
				AvailableCapacity: 0,
			}, nil
		}
	}

	return &GetCapacityResponse{
		AvailableCapacity: capacity,
	}, nil
}

func (s controllerService) ControllerGetCapabilities(context.Context, *ControllerGetCapabilitiesRequest) (*ControllerGetCapabilitiesResponse, error) {
	return &ControllerGetCapabilitiesResponse{
		Capabilities: []*ControllerServiceCapability{
			{
				Type: &ControllerServiceCapability_Rpc{
					Rpc: &ControllerServiceCapability_RPC{
						Type: ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
			{
				Type: &ControllerServiceCapability_Rpc{
					Rpc: &ControllerServiceCapability_RPC{
						Type: ControllerServiceCapability_RPC_GET_CAPACITY,
					},
				},
			},
		},
	}, nil
}

func (s controllerService) CreateSnapshot(context.Context, *CreateSnapshotRequest) (*CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "CreateSnapshot not implemented")
}

func (s controllerService) DeleteSnapshot(context.Context, *DeleteSnapshotRequest) (*DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "DeleteSnapshot not implemented")
}

func (s controllerService) ListSnapshots(context.Context, *ListSnapshotsRequest) (*ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ListSnapshots not implemented")
}

func (s controllerService) ControllerExpandVolume(context.Context, *ControllerExpandVolumeRequest) (*ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ControllerExpandVolume not implemented")
}
