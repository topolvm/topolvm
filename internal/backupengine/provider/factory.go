package provider

import (
	"fmt"

	v1 "github.com/topolvm/topolvm/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetProvider returns the appropriate snapshot provider based on the storage configuration
func GetProvider(client client.Client, snapStorage *v1.SnapshotBackupStorage) (Provider, error) {
	switch snapStorage.Spec.Engine {
	case v1.EngineRestic:
		return NewResticProvider(client, snapStorage)
	//case v1.EngineKopia:
	//	return NewKopiaProvider(setupOptions)
	default:
		return nil, fmt.Errorf("unsupported snapshot engine: %s", snapStorage.Spec.Engine)
	}
}
