package driver

import (
	"context"

	"github.com/topolvm/topolvm"
	"github.com/topolvm/topolvm/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	ctrl "sigs.k8s.io/controller-runtime"
)

var idLogger = ctrl.Log.WithName("driver").WithName("identity")

// NewIdentityService returns a new IdentityServer.
//
// ready is a function to check the plugin status.
// It should return non-nil error if the plugin is not healthy.
// If the plugin is not yet ready, it should return (false, nil).
// Otherwise, return (true, nil).
func NewIdentityService(ready func() (bool, error)) csi.IdentityServer {
	return &identityService{ready: ready}
}

type identityService struct {
	csi.UnimplementedIdentityServer

	ready func() (bool, error)
}

func (s identityService) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	idLogger.Info("GetPluginInfo", "req", req.String())
	return &csi.GetPluginInfoResponse{
		Name:          topolvm.GetPluginName(),
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
			{
				Type: &csi.PluginCapability_VolumeExpansion_{
					VolumeExpansion: &csi.PluginCapability_VolumeExpansion{
						Type: csi.PluginCapability_VolumeExpansion_ONLINE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_VolumeExpansion_{
					VolumeExpansion: &csi.PluginCapability_VolumeExpansion{
						Type: csi.PluginCapability_VolumeExpansion_OFFLINE,
					},
				},
			},
		},
	}, nil
}

func (s identityService) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	idLogger.Info("Probe", "req", req.String())
	ok, err := s.ready()
	if err != nil {
		idLogger.Error(err, "probe failed")
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	return &csi.ProbeResponse{
		Ready: &wrapperspb.BoolValue{
			Value: ok,
		},
	}, nil
}
