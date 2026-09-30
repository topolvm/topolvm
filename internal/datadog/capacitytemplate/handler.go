package capacitytemplate

import (
	"context"
	"sort"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// HandlerRemoteLVM calculates capacity from NodeGroup
	// spec.storage.remoteVolumes. It is currently the only implemented handler.
	HandlerRemoteLVM = "remote-lvm"
)

// CapacityHandler calculates fresh-node template capacity for one class of
// TopoLVM backing storage. A nil capacity with a nil error means the NodeGroup
// provides no storage for this handler, so no template capacity is published.
type CapacityHandler interface {
	CalculateCapacity(ctx context.Context, ng *unstructured.Unstructured, spareGB int64) (*resource.Quantity, error)
}

// defaultCapacityHandlers is the registry of implemented handlers. Adding a
// handler here is what makes it selectable from the configuration file.
func defaultCapacityHandlers() map[string]CapacityHandler {
	return map[string]CapacityHandler{
		HandlerRemoteLVM: RemoteLVMHandler{},
	}
}

func handlerNames(registry map[string]CapacityHandler) []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
