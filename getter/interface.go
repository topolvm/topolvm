package getter

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Interface interface {
	Get(ctx context.Context, key client.ObjectKey, obj client.Object) error
}
