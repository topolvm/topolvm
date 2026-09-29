package driver

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	internalDriver "github.com/topolvm/topolvm/internal/driver"
	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type NodeServerSettings = internalDriver.NodeServerSettings

var NewNodeServerWithSettings = internalDriver.NewNodeServer

// NewNodeServer keeps the signature that existed before NodeServerSettings was
// introduced, so that it does not break the external consumers of this package.
var NewNodeServer = func(
	nodeName string,
	vgServiceClient proto.VGServiceClient,
	lvServiceClient proto.LVServiceClient,
	mgr manager.Manager,
) (csi.NodeServer, error) {
	return internalDriver.NewNodeServer(
		nodeName,
		vgServiceClient,
		lvServiceClient,
		mgr,
		internalDriver.NodeServerSettings{VolumeHealth: true},
	)
}
