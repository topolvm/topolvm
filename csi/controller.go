package csi

import (
	"context"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/topolvm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// NewControllerService returns a new ControllerServer.
func NewControllerService(service LogicalVolumeService) ControllerServer {
	return &controllerService{service: service}
}

type controllerService struct {
	service LogicalVolumeService
}

func (s controllerService) CreateVolume(ctx context.Context, req *CreateVolumeRequest) (*CreateVolumeResponse, error) {
	log.Info("CreateVolume called", map[string]interface{}{
		"name":                       req.GetName(),
		"required":                   req.GetCapacityRange().GetRequiredBytes(),
		"limit":                      req.GetCapacityRange().GetLimitBytes(),
		"parameters":                 req.GetParameters(),
		"num_secrets":                len(req.GetSecrets()),
		"content_source":             req.GetVolumeContentSource(),
		"accessibility_requirements": req.GetAccessibilityRequirements().String(),
	})

	if req.GetVolumeContentSource() != nil {
		return nil, status.Errorf(codes.InvalidArgument, "volume_content_source not supported")
	}

	// check required volume capabilities
	for _, capability := range req.GetVolumeCapabilities() {
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
			return nil, status.Errorf(codes.InvalidArgument, "unknown or empty access_type")
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

	// process topology
	requirements := req.GetAccessibilityRequirements()
	var node string
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

	name := req.GetName()
	if name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid name")
	}

	var sizeGb int64
	switch size := req.GetCapacityRange().GetRequiredBytes(); {
	case size < 0:
		return nil, status.Errorf(codes.InvalidArgument, "required capacity must not be negative")
	case size == 0:
		sizeGb = 1
	default:
		sizeGb = (size-1)>>30 + 1
	}

	switch limit := req.GetCapacityRange().GetLimitBytes(); {
	case limit < 0:
		return nil, status.Errorf(codes.InvalidArgument, "capacity limit must not be negative")
	case limit > 0 && sizeGb<<30 > limit:
		return nil, status.Errorf(codes.InvalidArgument, "capacity limit exceeded")
	}

	volumeID, err := s.service.CreateVolume(ctx, node, name, sizeGb)
	if err != nil {
		s, ok := status.FromError(err)
		if !ok {
			return nil, status.Errorf(codes.Internal, err.Error())
		}
		return nil, s.Err()
	}

	return &CreateVolumeResponse{
		Volume: &Volume{
			CapacityBytes: sizeGb << 30,
			VolumeId:      volumeID,
			AccessibleTopology: []*Topology{
				{
					Segments: map[string]string{topolvm.TopologyNodeKey: node},
				},
			},
		},
	}, nil
}

func (s controllerService) DeleteVolume(ctx context.Context, req *DeleteVolumeRequest) (*DeleteVolumeResponse, error) {
	log.Info("DeleteVolume called", map[string]interface{}{
		"volume_id":   req.GetVolumeId(),
		"num_secrets": len(req.GetSecrets()),
	})

	err := s.service.DeleteVolume(ctx, req.GetVolumeId())
	if err != nil {
		log.Error("DeleteVolume failed", map[string]interface{}{
			"volume_id": req.GetVolumeId(),
			"error":     err.Error(),
		})
		s, ok := status.FromError(err)
		if !ok {
			return nil, status.Errorf(codes.Internal, err.Error())
		}
		return nil, s.Err()
	}

	return &DeleteVolumeResponse{}, nil
}

func (s controllerService) ControllerPublishVolume(context.Context, *ControllerPublishVolumeRequest) (*ControllerPublishVolumeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "ControllerPublishVolume not implemented")
}

func (s controllerService) ControllerUnpublishVolume(context.Context, *ControllerUnpublishVolumeRequest) (*ControllerUnpublishVolumeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "ControllerUnpublishVolume not implemented")
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
		return nil, status.Error(codes.InvalidArgument, "volume capabilities is empty")
	}

	pv, err := s.service.GetPVByVolumeID(ctx, req.GetVolumeId())
	if apierrors.IsNotFound(err) {
		return nil, status.Errorf(codes.NotFound, "%s is not found", req.GetVolumeId())
	} else if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var confirmed *ValidateVolumeCapabilitiesResponse_Confirmed
	if isValidVolumeCapabilities(pv, req.GetVolumeCapabilities()) {
		confirmed = &ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.GetVolumeCapabilities(),
		}
	}
	return &ValidateVolumeCapabilitiesResponse{
		Confirmed: confirmed,
	}, nil
}

func isValidVolumeCapabilities(pv *corev1.PersistentVolume, capabilities []*VolumeCapability) bool {
	for _, capability := range capabilities {
		// we only support single node writer
		if capability.GetAccessMode().Mode != VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
			return false
		}

		if *pv.Spec.VolumeMode == corev1.PersistentVolumeBlock {
			if capability.GetBlock() != nil {
				return false
			}
		}

		if *pv.Spec.VolumeMode == corev1.PersistentVolumeFilesystem {
			if capability.GetMount() != nil {
				return false
			}
		}
		// TODO: check file system type
	}
	return true
}

func (s controllerService) ListVolumes(ctx context.Context, req *ListVolumesRequest) (*ListVolumesResponse, error) {
	log.Info("ListVolumes called", map[string]interface{}{
		"max_entries":    req.GetMaxEntries(),
		"starting_token": req.GetStartingToken(),
	})

	// doListVolumes

	return &ListVolumesResponse{
		Entries: []*ListVolumesResponse_Entry{},
	}, nil
}

func (s controllerService) GetCapacity(ctx context.Context, req *GetCapacityRequest) (*GetCapacityResponse, error) {
	log.Info("GetCapacity called", map[string]interface{}{
		"volume_capabilities": req.GetVolumeCapabilities(),
		"parameters":          req.GetParameters(),
		"accessible_topology": req.GetAccessibleTopology(),
	})

	// doGetCapacity

	return &GetCapacityResponse{
		AvailableCapacity: 0,
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
						Type: ControllerServiceCapability_RPC_LIST_VOLUMES,
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
	return nil, status.Errorf(codes.Unimplemented, "CreateSnapshot not implemented")
}

func (s controllerService) DeleteSnapshot(context.Context, *DeleteSnapshotRequest) (*DeleteSnapshotResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "DeleteSnapshot not implemented")
}

func (s controllerService) ListSnapshots(context.Context, *ListSnapshotsRequest) (*ListSnapshotsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "ListSnapshots not implemented")
}

func (s controllerService) ControllerExpandVolume(context.Context, *ControllerExpandVolumeRequest) (*ControllerExpandVolumeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "ControllerExpandVolume not implemented")
}
