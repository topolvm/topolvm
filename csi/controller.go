package csi

import (
	context "context"

	"github.com/cybozu-go/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewControllerService returns a new ControllerServer.
func NewControllerService() ControllerServer {
	return &controllerService{}
}

type controllerService struct {
}

func (s controllerService) CreateVolume(ctx context.Context, req *CreateVolumeRequest) (*CreateVolumeResponse, error) {
	log.Info("CreateVolume called", map[string]interface{}{
		"name":           req.GetName(),
		"required":       req.GetCapacityRange().GetRequiredBytes(),
		"limit":          req.GetCapacityRange().GetLimitBytes(),
		"parameters":     req.GetParameters(),
		"num_secrets":    len(req.GetSecrets()),
		"content_source": req.GetVolumeContentSource(),
	})

	// check required volume capabilities
	for _, cap := range req.GetVolumeCapabilities() {
		if block := cap.GetBlock(); block != nil {
			log.Info("CreateVolume specifies volume capability", map[string]interface{}{
				"access_type": "block",
			})
		} else if mount := cap.GetMount(); mount != nil {
			log.Info("CreateVolume specifies volume capability", map[string]interface{}{
				"access_type": "mount",
				"fs_type":     mount.GetFsType(),
				"flags":       mount.GetMountFlags(),
			})
		} else if mode := cap.GetAccessMode(); mode != nil {
			log.Info("CreateVolume specifies volume capability", map[string]interface{}{
				"access_mode": VolumeCapability_AccessMode_Mode_name[int32(mode.GetMode())],
			})
		}
	}

	// process topology

	// doCreateVolume

	return &CreateVolumeResponse{
		Volume: &Volume{
			CapacityBytes:      req.GetCapacityRange().GetRequiredBytes(),
			VolumeId:           "foo",
			AccessibleTopology: []*Topology{},
		},
	}, nil
}

func (s controllerService) DeleteVolume(ctx context.Context, req *DeleteVolumeRequest) (*DeleteVolumeResponse, error) {
	log.Info("DeleteVolume called", map[string]interface{}{
		"volume_id":   req.GetVolumeId(),
		"num_secrets": len(req.GetSecrets()),
	})

	// doDeleteVolume

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

	// doValidateVolumeCapabilities

	return &ValidateVolumeCapabilitiesResponse{
		Confirmed: &ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: []*VolumeCapability{},
		},
	}, nil
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
