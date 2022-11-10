package client

import (
	"context"

	"github.com/topolvm/topolvm"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kind     = "LogicalVolume"
	kindList = "LogicalVolumeList"
)

var (
	group = topolvmv1.GroupVersion.Group
)

type wrappedReader struct {
	client client.Reader
	scheme *runtime.Scheme
}

var _ client.Reader = &wrappedReader{}

func NewWrappedReader(c client.Reader, s *runtime.Scheme) client.Reader {
	return &wrappedReader{
		client: c,
		scheme: s,
	}
}

func (c *wrappedReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	gvk := obj.GetObjectKind().GroupVersionKind()
	switch o := obj.(type) {
	case *unstructured.Unstructured:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := c.client.Get(ctx, key, o)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
		return c.client.Get(ctx, key, obj)
	case *metav1.PartialObjectMetadata:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := c.client.Get(ctx, key, o)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
		return c.client.Get(ctx, key, obj)
	case *topolvmv1.LogicalVolume:
		if topolvm.UseLegacy() {
			u := &unstructured.Unstructured{}
			if err := c.scheme.Convert(obj, u, nil); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			if err := c.client.Get(ctx, key, u); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return c.scheme.Convert(u, obj, nil)
		}
		return c.client.Get(ctx, key, obj)
	}
	return c.client.Get(ctx, key, obj)
}

func (c *wrappedReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	gvk := list.GetObjectKind().GroupVersionKind()
	switch o := list.(type) {
	case *unstructured.UnstructuredList:
		if gvk.Group == group && gvk.Kind == kindList && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kindList))
			err := c.client.List(ctx, list, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kindList))
			return err
		}
		return c.client.List(ctx, list, opts...)
	case *metav1.PartialObjectMetadataList:
		if gvk.Group == group && gvk.Kind == kindList && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kindList))
			err := c.client.List(ctx, list, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kindList))
			return err
		}
		return c.client.List(ctx, list, opts...)
	case *topolvmv1.LogicalVolumeList:
		if topolvm.UseLegacy() {
			l := new(topolvmlegacyv1.LogicalVolumeList)
			if err := c.client.List(ctx, l, opts...); err != nil {
				return err
			}
			u := &unstructured.UnstructuredList{}
			if err := c.scheme.Convert(l, u, nil); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kindList))
			return c.scheme.Convert(u, list, nil)
		}
		return c.client.List(ctx, list, opts...)
	}
	return c.client.List(ctx, list, opts...)
}

type wrappedClient struct {
	reader client.Reader
	client client.Client
}

var _ client.Client = &wrappedClient{}

func NewWrappedClient(c client.Client) client.Client {
	return &wrappedClient{
		reader: NewWrappedReader(c, c.Scheme()),
		client: c,
	}
}

func (c *wrappedClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	return c.reader.Get(ctx, key, obj)
}

func (c *wrappedClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return c.reader.List(ctx, list, opts...)
}

func (c *wrappedClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	gvk := obj.GetObjectKind().GroupVersionKind()
	switch o := obj.(type) {
	case *unstructured.Unstructured:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := c.client.Create(ctx, o, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
		return c.client.Create(ctx, obj, opts...)
	case *metav1.PartialObjectMetadata:
		return c.client.Create(ctx, obj, opts...)
	case *topolvmv1.LogicalVolume:
		if topolvm.UseLegacy() {
			u := &unstructured.Unstructured{}
			if err := c.client.Scheme().Convert(obj, u, nil); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			if err := c.client.Create(ctx, u, opts...); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return c.client.Scheme().Convert(u, obj, nil)
		}
		return c.client.Create(ctx, obj, opts...)
	}
	return c.client.Create(ctx, obj, opts...)
}

func (c *wrappedClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	gvk := obj.GetObjectKind().GroupVersionKind()
	switch o := obj.(type) {
	case *unstructured.Unstructured:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := c.client.Delete(ctx, o, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
		return c.client.Delete(ctx, obj, opts...)
	case *metav1.PartialObjectMetadata:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := c.client.Delete(ctx, o, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
		return c.client.Delete(ctx, obj, opts...)
	case *topolvmv1.LogicalVolume:
		if topolvm.UseLegacy() {
			u := &unstructured.Unstructured{}
			if err := c.client.Scheme().Convert(obj, u, nil); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			return c.client.Delete(ctx, u, opts...)
		}
		return c.client.Delete(ctx, obj, opts...)
	}
	return c.client.Delete(ctx, obj, opts...)
}

func (c *wrappedClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	gvk := obj.GetObjectKind().GroupVersionKind()
	switch o := obj.(type) {
	case *unstructured.Unstructured:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := c.client.Update(ctx, o, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
		return c.client.Update(ctx, obj, opts...)
	case *metav1.PartialObjectMetadata:
		return c.client.Update(ctx, obj, opts...)
	case *topolvmv1.LogicalVolume:
		if topolvm.UseLegacy() {
			u := &unstructured.Unstructured{}
			if err := c.client.Scheme().Convert(obj, u, nil); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			if err := c.client.Update(ctx, u, opts...); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return c.client.Scheme().Convert(u, obj, nil)
		}
		return c.client.Update(ctx, obj, opts...)
	}
	return c.client.Update(ctx, obj, opts...)
}

// wrappedClient assumes that LogicalVolume definitions on topolvm.io and topolvm.cybozu.com are identical.
// Since patch processes resources as Objects, even if the structs are different, if the Spec and Status are the same, there is no problem with patch processing.
// ref: https://github.com/kubernetes-sigs/controller-runtime/blob/v0.12.1/pkg/client/patch.go#L114
func (c *wrappedClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	gvk := obj.GetObjectKind().GroupVersionKind()
	switch o := obj.(type) {
	case *unstructured.Unstructured:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := c.client.Patch(ctx, o, patch, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
		return c.client.Patch(ctx, obj, patch, opts...)
	case *metav1.PartialObjectMetadata:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := c.client.Patch(ctx, o, patch, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
	case *topolvmv1.LogicalVolume:
		if topolvm.UseLegacy() {
			u := &unstructured.Unstructured{}
			if err := c.client.Scheme().Convert(obj, u, nil); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			if err := c.client.Patch(ctx, u, patch, opts...); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return c.client.Scheme().Convert(u, obj, nil)
		}
		return c.client.Patch(ctx, obj, patch, opts...)
	}
	return c.client.Patch(ctx, obj, patch, opts...)
}

func (c *wrappedClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	gvk := obj.GetObjectKind().GroupVersionKind()
	switch o := obj.(type) {
	case *unstructured.Unstructured:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := c.client.DeleteAllOf(ctx, o, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
		return c.client.DeleteAllOf(ctx, obj, opts...)
	case *metav1.PartialObjectMetadata:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := c.client.DeleteAllOf(ctx, o, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
		return c.client.DeleteAllOf(ctx, obj, opts...)
	case *topolvmv1.LogicalVolume:
		if topolvm.UseLegacy() {
			u := &unstructured.Unstructured{}
			if err := c.client.Scheme().Convert(obj, u, nil); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			return c.client.DeleteAllOf(ctx, u, opts...)
		}
		return c.client.DeleteAllOf(ctx, obj, opts...)
	}
	return c.client.DeleteAllOf(ctx, obj, opts...)
}

func (c *wrappedClient) Status() client.StatusWriter {
	return &wrappedStatusWriter{client: c.client}
}

func (c *wrappedClient) Scheme() *runtime.Scheme {
	return c.client.Scheme()
}

func (c *wrappedClient) RESTMapper() meta.RESTMapper {
	return c.client.RESTMapper()
}

type wrappedStatusWriter struct {
	client client.Client
}

var _ client.StatusWriter = &wrappedStatusWriter{}

func (c *wrappedStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	gvk := obj.GetObjectKind().GroupVersionKind()
	switch o := obj.(type) {
	case *unstructured.Unstructured:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := c.client.Status().Update(ctx, o, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
		return c.client.Status().Update(ctx, obj, opts...)
	case *metav1.PartialObjectMetadata:
		return c.client.Status().Update(ctx, obj, opts...)
	case *topolvmv1.LogicalVolume:
		if topolvm.UseLegacy() {
			u := &unstructured.Unstructured{}
			if err := c.client.Scheme().Convert(obj, u, nil); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			if err := c.client.Status().Update(ctx, u, opts...); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return c.client.Scheme().Convert(u, obj, nil)
		}
		return c.client.Status().Update(ctx, obj, opts...)
	}
	return c.client.Status().Update(ctx, obj, opts...)
}

// wrappedClient assumes that LogicalVolume definitions on topolvm.io and topolvm.cybozu.com are identical.
// Since patch processes resources as Objects, even if the structs are different, if the Spec and Status are the same, there is no problem with patch processing.
// ref: https://github.com/kubernetes-sigs/controller-runtime/blob/v0.12.1/pkg/client/patch.go#L114
func (c *wrappedStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	gvk := obj.GetObjectKind().GroupVersionKind()
	switch o := obj.(type) {
	case *unstructured.Unstructured:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := c.client.Status().Patch(ctx, o, patch, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
		return c.client.Status().Patch(ctx, obj, patch, opts...)
	case *metav1.PartialObjectMetadata:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := c.client.Status().Patch(ctx, o, patch, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
		return c.client.Status().Patch(ctx, obj, patch, opts...)
	case *topolvmv1.LogicalVolume:
		if topolvm.UseLegacy() {
			u := &unstructured.Unstructured{}
			if err := c.client.Scheme().Convert(obj, u, nil); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			if err := c.client.Status().Patch(ctx, u, patch, opts...); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return c.client.Scheme().Convert(u, obj, nil)
		}
		return c.client.Status().Patch(ctx, obj, patch, opts...)
	}
	return c.client.Status().Patch(ctx, obj, patch, opts...)
}
