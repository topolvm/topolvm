package driver

import (
	"context"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/topolvm"
	"github.com/cybozu-go/topolvm/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
)

// NewIdentityService returns a new IdentityServer.
func NewIdentityService() csi.IdentityServer {
	return &identityService{}
}

type identityService struct {
}

func (s identityService) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	log.Info("GetPluginInfo", map[string]interface{}{
		"req": req.String(),
	})
	return &csi.GetPluginInfoResponse{
		Name:          topolvm.PluginName,
		VendorVersion: topolvm.Version,
	}, nil
}

func (s identityService) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	log.Info("GetPluginCapabilities", map[string]interface{}{
		"req": req.String(),
	})
	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
					},
				},
			},
		},
	}, nil
}

func (s identityService) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	log.Info("Probe", map[string]interface{}{
		"req": req.String(),
	})
	return &csi.ProbeResponse{
		Ready: &wrappers.BoolValue{
			Value: true,
		},
	}, nil
}
