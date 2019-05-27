package csi

import (
	"context"

	"github.com/cybozu-go/log"
	"github.com/golang/protobuf/ptypes/wrappers"
)

// NewIdentityService returns a new IdentityServer.
func NewIdentityService() IdentityServer {
	return &identityService{}
}

type identityService struct {
}

func (s identityService) GetPluginInfo(ctx context.Context, req *GetPluginInfoRequest) (*GetPluginInfoResponse, error) {
	log.Info("GetPluginInfo", map[string]interface{}{
		"req": req.String(),
	})
	return &GetPluginInfoResponse{
		Name:          "csi-topolvm.cybozu-ne.co",
		VendorVersion: "1.0.0",
	}, nil
}

func (s identityService) GetPluginCapabilities(ctx context.Context, req *GetPluginCapabilitiesRequest) (*GetPluginCapabilitiesResponse, error) {
	log.Info("GetPluginCapabilities", map[string]interface{}{
		"req": req.String(),
	})
	return &GetPluginCapabilitiesResponse{
		Capabilities: []*PluginCapability{
			{
				Type: &PluginCapability_Service_{
					Service: &PluginCapability_Service{
						Type: PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
			{
				Type: &PluginCapability_Service_{
					Service: &PluginCapability_Service{
						Type: PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
					},
				},
			},
		},
	}, nil
}

func (s identityService) Probe(ctx context.Context, req *ProbeRequest) (*ProbeResponse, error) {
	log.Info("Probe", map[string]interface{}{
		"req": req.String(),
	})
	return &ProbeResponse{
		Ready: &wrappers.BoolValue{
			Value: true,
		},
	}, nil
}
