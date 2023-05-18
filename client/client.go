package client

import (
	"context"
	"fmt"

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

func (c *wrappedReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	gvk := obj.GetObjectKind().GroupVersionKind()
	switch o := obj.(type) {
	case *unstructured.Unstructured:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := c.client.Get(ctx, key, o, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
		return c.client.Get(ctx, key, obj, opts...)
	case *metav1.PartialObjectMetadata:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := c.client.Get(ctx, key, o, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
		return c.client.Get(ctx, key, obj, opts...)
	case *topolvmv1.LogicalVolume:
		if topolvm.UseLegacy() {
			u := &unstructured.Unstructured{}
			if err := c.scheme.Convert(obj, u, nil); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			if err := c.client.Get(ctx, key, u, opts...); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return c.scheme.Convert(u, obj, nil)
		}
		return c.client.Get(ctx, key, obj, opts...)
	}
	return c.client.Get(ctx, key, obj, opts...)
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

func (c *wrappedClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return c.reader.Get(ctx, key, obj, opts...)
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

func (c *wrappedClient) Status() client.SubResourceWriter {
	return c.SubResource("status")
}

func (c *wrappedClient) SubResource(subResource string) client.SubResourceClient {
	return &wrappedSubResourceClient{
		client:      c.client,
		subResource: subResource,
	}
}

func (c *wrappedClient) Scheme() *runtime.Scheme {
	return c.client.Scheme()
}

func (c *wrappedClient) RESTMapper() meta.RESTMapper {
	return c.client.RESTMapper()
}

type wrappedSubResourceClient struct {
	client      client.Client
	subResource string
}

var _ client.SubResourceClient = &wrappedSubResourceClient{}

func (c *wrappedSubResourceClient) Get(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceGetOption) error {
	// - This method is currently not used in TopoLVM.
	// - To implement this method, tests are required, but there are no similar tests
	//   implemented in the upstream code, and we will need a lot of effort to study it.
	return fmt.Errorf("wrappedSubResourceClient.Get is not implemented")
}

func (c *wrappedSubResourceClient) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	// - This method is currently not used in TopoLVM.
	// - To implement this method, tests are required, but there are no similar tests
	//   implemented in the upstream code, and we will need a lot of effort to study it.
	return fmt.Errorf("wrappedSubResourceClient.Create is not implemented")
}

func (c *wrappedSubResourceClient) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	sc := c.client.SubResource(c.subResource)
	gvk := obj.GetObjectKind().GroupVersionKind()
	switch o := obj.(type) {
	case *unstructured.Unstructured:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := sc.Update(ctx, o, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
		return sc.Update(ctx, obj, opts...)
	case *metav1.PartialObjectMetadata:
		return sc.Update(ctx, obj, opts...)
	case *topolvmv1.LogicalVolume:
		if topolvm.UseLegacy() {
			u := &unstructured.Unstructured{}
			if err := c.client.Scheme().Convert(obj, u, nil); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			if err := sc.Update(ctx, u, opts...); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return c.client.Scheme().Convert(u, obj, nil)
		}
		return sc.Update(ctx, obj, opts...)
	}
	return sc.Update(ctx, obj, opts...)
}

// wrappedClient assumes that LogicalVolume definitions on topolvm.io and topolvm.cybozu.com are identical.
// Since patch processes resources as Objects, even if the structs are different, if the Spec and Status are the same, there is no problem with patch processing.
// ref: https://github.com/kubernetes-sigs/controller-runtime/blob/v0.12.1/pkg/client/patch.go#L114
func (c *wrappedSubResourceClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	sc := c.client.SubResource(c.subResource)
	gvk := obj.GetObjectKind().GroupVersionKind()
	switch o := obj.(type) {
	case *unstructured.Unstructured:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := sc.Patch(ctx, o, patch, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
		return sc.Patch(ctx, obj, patch, opts...)
	case *metav1.PartialObjectMetadata:
		if gvk.Group == group && gvk.Kind == kind && topolvm.UseLegacy() {
			o.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			err := sc.Patch(ctx, o, patch, opts...)
			o.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return err
		}
		return sc.Patch(ctx, obj, patch, opts...)
	case *topolvmv1.LogicalVolume:
		if topolvm.UseLegacy() {
			u := &unstructured.Unstructured{}
			if err := c.client.Scheme().Convert(obj, u, nil); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmlegacyv1.GroupVersion.WithKind(kind))
			if err := sc.Patch(ctx, u, patch, opts...); err != nil {
				return err
			}
			u.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
			return c.client.Scheme().Convert(u, obj, nil)
		}
		return sc.Patch(ctx, obj, patch, opts...)
	}
	return sc.Patch(ctx, obj, patch, opts...)
}
