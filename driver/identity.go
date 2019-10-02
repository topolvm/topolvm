package driver

import (
	"context"

	"github.com/cybozu-go/topolvm"
	"github.com/cybozu-go/topolvm/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var idLogger = logf.Log.WithName("driver").WithName("identity")

// NewIdentityService returns a new IdentityServer.
func NewIdentityService() csi.IdentityServer {
	return &identityService{}
}

type identityService struct {
}

func (s identityService) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	idLogger.Info("GetPluginInfo", "req", req.String())
	return &csi.GetPluginInfoResponse{
		Name:          topolvm.PluginName,
		VendorVersion: topolvm.Version,
	}, nil
}

func (s identityService) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	idLogger.Info("GetPluginCapabilities", "req", req.String())
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
	idLogger.Info("Probe", "req", req.String())
	return &csi.ProbeResponse{
		Ready: &wrappers.BoolValue{
			Value: true,
		},
	}, nil
}
