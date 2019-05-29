package csi

import (
	"context"
	"path"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/topolvm"
	"github.com/cybozu-go/well"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewNodeService returns a new NodeServer.
func NewNodeService(nodeName, vgName string) NodeServer {
	return &nodeService{
		nodeName: nodeName,
		vgName:   vgName,
	}
}

type nodeService struct {
	nodeName string
	vgName   string
}

func (s nodeService) NodeStageVolume(context.Context, *NodeStageVolumeRequest) (*NodeStageVolumeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "NodeStageVolume not implemented")
}

func (s nodeService) NodeUnstageVolume(context.Context, *NodeUnstageVolumeRequest) (*NodeUnstageVolumeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "NodeUnstageVolume not implemented")
}

func (s nodeService) NodePublishVolume(ctx context.Context, req *NodePublishVolumeRequest) (*NodePublishVolumeResponse, error) {
	log.Info("NodePublishVolume called", map[string]interface{}{
		"volume_id":         req.GetVolumeId(),
		"publish_context":   req.GetPublishContext(),
		"target_path":       req.GetTargetPath(),
		"volume_capability": req.GetVolumeCapability(),
		"read_only":         req.GetReadonly(),
		"num_secrets":       len(req.GetSecrets()),
		"volume_context":    req.GetVolumeContext(),
	})

	if req.GetVolumeCapability().GetBlock() != nil {
		// block device
	} else if mountOption := req.GetVolumeCapability().GetMount(); mountOption != nil {
		accessMode := req.GetVolumeCapability().GetAccessMode().GetMode()
		if accessMode != VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
			modeName := VolumeCapability_AccessMode_Mode_name[int32(accessMode)]
			return nil, status.Errorf(codes.FailedPrecondition, "unsupported access mode: %s", modeName)
		}

		device := path.Join("/dev", s.vgName, req.GetVolumeId())
		out, err := well.CommandContext(ctx, "mkfs", "-t", mountOption.FsType, device).CombinedOutput()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "mkfs failed: %s", out)
		}
		out, err = well.CommandContext(ctx, "mount", "-t", mountOption.FsType, device, req.GetTargetPath()).CombinedOutput()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "mount failed: %s", out)
		}
	}

	return &NodePublishVolumeResponse{}, nil
}

func (s nodeService) NodeUnpublishVolume(ctx context.Context, req *NodeUnpublishVolumeRequest) (*NodeUnpublishVolumeResponse, error) {
	log.Info("NodeUnpublishVolume called", map[string]interface{}{
		"volume_id":   req.GetVolumeId(),
		"target_path": req.GetTargetPath(),
	})

	// doNodeUnpublishVolume

	return &NodeUnpublishVolumeResponse{}, nil
}

func (s nodeService) NodeGetVolumeStats(ctx context.Context, req *NodeGetVolumeStatsRequest) (*NodeGetVolumeStatsResponse, error) {
	log.Info("NodeGetVolumeStats called", map[string]interface{}{
		"volume_id":   req.GetVolumeId(),
		"volume_path": req.GetVolumePath(),
	})

	// doNodeGetVolumeStats

	return &NodeGetVolumeStatsResponse{
		Usage: []*VolumeUsage{},
	}, nil
}

func (s nodeService) NodeExpandVolume(context.Context, *NodeExpandVolumeRequest) (*NodeExpandVolumeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "NodeExpandVolume not implemented")
}

func (s nodeService) NodeGetCapabilities(context.Context, *NodeGetCapabilitiesRequest) (*NodeGetCapabilitiesResponse, error) {
	return &NodeGetCapabilitiesResponse{
		Capabilities: []*NodeServiceCapability{
			{
				Type: &NodeServiceCapability_Rpc{
					Rpc: &NodeServiceCapability_RPC{
						Type: NodeServiceCapability_RPC_GET_VOLUME_STATS,
					},
				},
			},
		},
	}, nil
}

func (s nodeService) NodeGetInfo(ctx context.Context, req *NodeGetInfoRequest) (*NodeGetInfoResponse, error) {
	return &NodeGetInfoResponse{
		NodeId: s.nodeName,
		AccessibleTopology: &Topology{
			Segments: map[string]string{
				topolvm.TopologyNodeKey: s.nodeName,
			},
		},
	}, nil
}
