package csi

import (
	"context"
	"github.com/cybozu-go/topolvm"

	"github.com/cybozu-go/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewNodeService returns a new NodeServer.
func NewNodeService(nodeName string) NodeServer {
	return &nodeService{nodeName: nodeName}
}

type nodeService struct {
	nodeName string
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

	// doNodePublishVolume

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
