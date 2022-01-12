package getter

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Unlike reconciler, webhook or CSI gRPC are one shot API, so they needs read-after-create consistency.
// This provides us such semantics by reading api-server directly if the object is missing on the cache reader.
type RetryMissingGetter struct {
	cacheReader client.Reader
	apiReader   client.Reader
}

// NewRetryMissingGetter creates a RetryMissingGetter instance.
func NewRetryMissingGetter(cacheReader client.Reader, apiReader client.Reader) *RetryMissingGetter {
	return &RetryMissingGetter{
		cacheReader: cacheReader,
		apiReader:   apiReader,
	}
}

// Get tries cache reader, then if it returns NotFound error, retry direct reader.
func (r *RetryMissingGetter) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	err := r.cacheReader.Get(ctx, key, obj)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	return r.apiReader.Get(ctx, key, obj)
}
