package csi

import (
	"context"

	"github.com/golang/protobuf/ptypes/wrappers"
)

// NewIdentityService returns a new IdentityServer.
func NewIdentityService() IdentityServer {
	return &identityService{}
}

type identityService struct {
}

func (s identityService) GetPluginInfo(context.Context, *GetPluginInfoRequest) (*GetPluginInfoResponse, error) {
	return &GetPluginInfoResponse{
		Name:          "csi-topolvm.cybozu-ne.co",
		VendorVersion: "1.0.0",
	}, nil
}

func (s identityService) GetPluginCapabilities(context.Context, *GetPluginCapabilitiesRequest) (*GetPluginCapabilitiesResponse, error) {
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

func (s identityService) Probe(context.Context, *ProbeRequest) (*ProbeResponse, error) {
	return &ProbeResponse{
		Ready: &wrappers.BoolValue{
			Value: true,
		},
	}, nil
}
