package client

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"google.golang.org/grpc/codes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type fakeListOptions struct {
	f func(opts *client.ListOptions)
}

func (o *fakeListOptions) ApplyToList(opts *client.ListOptions) {
	o.f(opts)
}

const (
	configmapName      = "test"
	configmapNamespace = "default"
)

var (
	configMapGVK     = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}
	configMapListGVK = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMapList"}
	configmapKey     = types.NamespacedName{
		Name:      configmapName,
		Namespace: configmapNamespace,
	}
)

var _ = Describe("client", func() {
	Context("current env", func() {
		BeforeEach(func() {
			os.Setenv("USE_LEGACY", "")
			err := k8sDelegatedClient.DeleteAllOf(testCtx, &topolvmv1.LogicalVolume{})
			Expect(err).ShouldNot(HaveOccurred())
			err = k8sDelegatedClient.DeleteAllOf(testCtx, &topolvmlegacyv1.LogicalVolume{})
			Expect(err).ShouldNot(HaveOccurred())
			cm := &corev1.ConfigMap{}
			cm.Name = configmapName
			cm.Namespace = configmapNamespace
			k8sDelegatedClient.Delete(testCtx, cm)
		})

		Context("wrappedReader", func() {
			Context("Get", func() {
				It("standard case", func() {
					i := 0
					lv := currentLV(i)
					err := k8sDelegatedClient.Create(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					setCurrentLVStatus(lv, i)
					err = k8sDelegatedClient.Status().Update(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					checklv := new(topolvmv1.LogicalVolume)
					c := NewWrappedReader(k8sAPIReader, scheme)
					err = c.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(checklv.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
					Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
					Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
					Expect(checklv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
					Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
					Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
					Expect(checklv.Spec.AccessType).Should(Equal("rw"))
					Expect(checklv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
					Expect(checklv.Status.Code).Should(Equal(codes.Unknown))
					Expect(checklv.Status.Message).Should(Equal(codes.Unknown.String()))
					Expect(checklv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
				})
			})

			Context("List", func() {
				It("standard case", func() {
					for i := 0; i < 2; i++ {
						lv := currentLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						setCurrentLVStatus(lv, i)
						err = k8sDelegatedClient.Status().Update(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())
					}

					lvlist := new(topolvmv1.LogicalVolumeList)
					c := NewWrappedReader(k8sAPIReader, scheme)
					err := c.List(testCtx, lvlist)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(len(lvlist.Items)).Should(Equal(2))
					for i, lv := range lvlist.Items {
						Expect(lv.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						Expect(lv.Spec.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						Expect(lv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
						Expect(lv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
						Expect(lv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						Expect(lv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
						Expect(lv.Spec.AccessType).Should(Equal("rw"))
						Expect(lv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
						Expect(lv.Status.Code).Should(Equal(codes.Unknown))
						Expect(lv.Status.Message).Should(Equal(codes.Unknown.String()))
						Expect(lv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
					}
				})
			})
		})

		Context("wrappedClient", func() {
			Context("Get", func() {
				It("standard case", func() {
					i := 0
					lv := currentLV(i)
					err := k8sDelegatedClient.Create(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					setCurrentLVStatus(lv, i)
					err = k8sDelegatedClient.Status().Update(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					Eventually(func(g Gomega) {
						checklv := new(topolvmv1.LogicalVolume)
						c := NewWrappedClient(k8sDelegatedClient)
						err = c.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
						g.Expect(err).ShouldNot(HaveOccurred())
						g.Expect(checklv.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						g.Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						g.Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
						g.Expect(checklv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
						g.Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						g.Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
						g.Expect(checklv.Spec.AccessType).Should(Equal("rw"))
						g.Expect(checklv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
						g.Expect(checklv.Status.Code).Should(Equal(codes.Unknown))
						g.Expect(checklv.Status.Message).Should(Equal(codes.Unknown.String()))
						g.Expect(checklv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
					}).Should(Succeed())
				})
			})

			Context("List", func() {
				It("standard case", func() {
					for i := 0; i < 2; i++ {
						lv := currentLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						setCurrentLVStatus(lv, i)
						err = k8sDelegatedClient.Status().Update(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())
					}

					Eventually(func(g Gomega) {
						lvlist := new(topolvmv1.LogicalVolumeList)
						c := NewWrappedClient(k8sDelegatedClient)
						err := c.List(testCtx, lvlist)
						g.Expect(err).ShouldNot(HaveOccurred())
						g.Expect(len(lvlist.Items)).Should(Equal(2))
						for i, lv := range lvlist.Items {
							g.Expect(lv.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
							g.Expect(lv.Spec.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
							g.Expect(lv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
							g.Expect(lv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
							g.Expect(lv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
							g.Expect(lv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
							g.Expect(lv.Spec.AccessType).Should(Equal("rw"))
							g.Expect(lv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
							g.Expect(lv.Status.Code).Should(Equal(codes.Unknown))
							g.Expect(lv.Status.Message).Should(Equal(codes.Unknown.String()))
							g.Expect(lv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						}
					}).Should(Succeed())
				})
			})

			Context("Create", func() {
				It("standard case", func() {
					i := 0
					lv := currentLV(i)
					c := NewWrappedClient(k8sDelegatedClient)
					err := c.Create(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					checklv := new(topolvmv1.LogicalVolume)
					err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(checklv.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
					Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
					Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
					Expect(checklv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
					Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
					Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
					Expect(checklv.Spec.AccessType).Should(Equal("rw"))
				})
			})

			Context("Delete", func() {
				It("standard case", func() {
					i := 0
					lv := currentLV(i)
					err := k8sDelegatedClient.Create(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					c := NewWrappedClient(k8sDelegatedClient)
					err = c.Delete(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					checklv := new(topolvmv1.LogicalVolume)
					err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
					Expect(err).Should(HaveOccurred())
					Expect(apierrs.IsNotFound(err)).Should(BeTrue())
				})
			})

			Context("Update", func() {
				It("standard case", func() {
					i := 0
					lv := currentLV(i)
					err := k8sDelegatedClient.Create(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					ann := map[string]string{"foo": "bar"}
					lv.Annotations = ann
					lv.Spec.Name = fmt.Sprintf("updated-current-%d", i)
					lv.Spec.NodeName = fmt.Sprintf("updated-node-%d", i)
					c := NewWrappedClient(k8sDelegatedClient)
					err = c.Update(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					checklv := new(topolvmv1.LogicalVolume)
					err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(checklv.Annotations).Should(Equal(ann))
					Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("updated-current-%d", i)))
					Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("updated-node-%d", i)))
					Expect(checklv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
					Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
					Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
					Expect(checklv.Spec.AccessType).Should(Equal("rw"))
				})
			})

			Context("Patch", func() {
				It("standard case", func() {
					i := 0
					lv := currentLV(i)
					err := k8sDelegatedClient.Create(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					ann := map[string]string{"foo": "bar"}
					lv2 := lv.DeepCopy()
					lv2.Annotations = ann
					lv2.Spec.Name = fmt.Sprintf("updated-current-%d", i)
					lv2.Spec.NodeName = fmt.Sprintf("updated-node-%d", i)
					c := NewWrappedClient(k8sDelegatedClient)
					patch := client.MergeFrom(lv)
					err = c.Patch(testCtx, lv2, patch)
					Expect(err).ShouldNot(HaveOccurred())

					checklv := new(topolvmv1.LogicalVolume)
					err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(checklv.Annotations).Should(Equal(ann))
					Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("updated-current-%d", i)))
					Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("updated-node-%d", i)))
					Expect(checklv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
					Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
					Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
					Expect(checklv.Spec.AccessType).Should(Equal("rw"))
				})
			})

			Context("DeleteAllOf", func() {
				It("standard case", func() {
					for i := 0; i < 2; i++ {
						lv := currentLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())
					}

					checklvlist := new(topolvmv1.LogicalVolumeList)
					err := k8sAPIReader.List(testCtx, checklvlist)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(len(checklvlist.Items)).Should(Equal(2))

					lv := new(topolvmv1.LogicalVolume)
					c := NewWrappedClient(k8sDelegatedClient)
					err = c.DeleteAllOf(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					checklvlist = new(topolvmv1.LogicalVolumeList)
					err = k8sAPIReader.List(testCtx, checklvlist)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(len(checklvlist.Items)).Should(Equal(0))
				})
			})

			Context("SubResourceClient", func() {
				Context("Get", func() {
					It("should not be implemented", func() {
						i := 0
						lv := currentLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						c := NewWrappedClient(k8sDelegatedClient)
						err = c.SubResource("status").Get(testCtx, lv, nil)
						Expect(err).Should(HaveOccurred())
					})
				})

				Context("Create", func() {
					It("should not be implemented", func() {
						i := 0
						lv := currentLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						c := NewWrappedClient(k8sDelegatedClient)
						err = c.SubResource("status").Create(testCtx, lv, nil)
						Expect(err).Should(HaveOccurred())
					})
				})
			})

			Context("SubResourceWriter", func() {
				Context("Update", func() {
					It("standard case", func() {
						i := 0
						lv := currentLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						setCurrentLVStatus(lv, i)
						c := NewWrappedClient(k8sDelegatedClient)
						err = c.Status().Update(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						checklv := new(topolvmv1.LogicalVolume)
						err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(checklv.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
						Expect(checklv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
						Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
						Expect(checklv.Spec.AccessType).Should(Equal("rw"))
						Expect(checklv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
						Expect(checklv.Status.Code).Should(Equal(codes.Unknown))
						Expect(checklv.Status.Message).Should(Equal(codes.Unknown.String()))
						Expect(checklv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
					})
				})

				Context("Patch", func() {
					It("standard case", func() {
						i := 0
						lv := currentLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						lv2 := lv.DeepCopy()
						setCurrentLVStatus(lv2, i)
						patch := client.MergeFrom(lv)
						c := NewWrappedClient(k8sDelegatedClient)
						err = c.Status().Patch(testCtx, lv2, patch)
						Expect(err).ShouldNot(HaveOccurred())

						checklv := new(topolvmv1.LogicalVolume)
						err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(checklv.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
						Expect(checklv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
						Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
						Expect(checklv.Spec.AccessType).Should(Equal("rw"))
						Expect(checklv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
						Expect(checklv.Status.Code).Should(Equal(codes.Unknown))
						Expect(checklv.Status.Message).Should(Equal(codes.Unknown.String()))
						Expect(checklv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
					})
				})
			})
		})
	})

	Context("legacy env", func() {
		BeforeEach(func() {
			os.Setenv("USE_LEGACY", "true")
			err := k8sDelegatedClient.DeleteAllOf(testCtx, &topolvmv1.LogicalVolume{})
			Expect(err).ShouldNot(HaveOccurred())
			err = k8sDelegatedClient.DeleteAllOf(testCtx, &topolvmlegacyv1.LogicalVolume{})
			Expect(err).ShouldNot(HaveOccurred())
			cm := &corev1.ConfigMap{}
			cm.Name = configmapName
			cm.Namespace = configmapNamespace
			k8sDelegatedClient.Delete(testCtx, cm)
		})

		Context("wrappedReader", func() {
			Context("Get", func() {
				get := func(doCheck bool, f func(*wrappedReader, *topolvmv1.LogicalVolume, string)) {
					i := 0
					lv := legacyLV(i)
					err := k8sDelegatedClient.Create(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					setLegacyLVStatus(lv, i)
					err = k8sDelegatedClient.Status().Update(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					checklv := new(topolvmv1.LogicalVolume)
					c := NewWrappedReader(k8sAPIReader, scheme)
					r := c.(*wrappedReader)
					f(r, checklv, lv.Name)

					if doCheck {
						Expect(checklv.Name).Should(Equal(fmt.Sprintf("legacy-%d", i)))
						Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("legacy-%d", i)))
						Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
						Expect(checklv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
						Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
						Expect(checklv.Spec.AccessType).Should(Equal("rw"))
						Expect(checklv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
						Expect(checklv.Status.Code).Should(Equal(codes.Unknown))
						Expect(checklv.Status.Message).Should(Equal(codes.Unknown.String()))
						Expect(checklv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
					}
				}

				It("typedClient", func() {
					get(true, func(c *wrappedReader, lv *topolvmv1.LogicalVolume, name string) {
						err := c.Get(testCtx, types.NamespacedName{Name: name}, lv)
						Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("unstructured.Unstructured", func() {
					get(true, func(c *wrappedReader, lv *topolvmv1.LogicalVolume, name string) {
						u := &unstructured.Unstructured{}
						Expect(c.scheme.Convert(lv, u, nil)).ShouldNot(HaveOccurred())
						err := c.Get(testCtx, types.NamespacedName{Name: name}, u)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(c.scheme.Convert(u, lv, nil)).ShouldNot(HaveOccurred())
					})
				})

				It("metav1.PartialObjectMetadata", func() {
					get(false, func(c *wrappedReader, lv *topolvmv1.LogicalVolume, name string) {
						p := &metav1.PartialObjectMetadata{}
						p.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
						lv.ObjectMeta.DeepCopyInto(&p.ObjectMeta)
						err := c.Get(testCtx, types.NamespacedName{Name: name}, p)
						Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("non LogicalVolume object", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())
					checkcm := new(corev1.ConfigMap)
					c := NewWrappedReader(k8sAPIReader, scheme)
					err = c.Get(testCtx, configmapKey, checkcm)
					Expect(err).ShouldNot(HaveOccurred())
				})

				It("non LogicalVolume unstructured.Unstructured", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())
					u := &unstructured.Unstructured{}
					u.SetGroupVersionKind(configMapGVK)
					c := NewWrappedReader(k8sAPIReader, scheme)
					err = c.Get(testCtx, configmapKey, u)
					Expect(err).ShouldNot(HaveOccurred())
				})

				It("non LogicalVolume metav1.PartialObjectMetadata", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())
					p := &metav1.PartialObjectMetadata{}
					p.SetGroupVersionKind(configMapGVK)
					c := NewWrappedReader(k8sAPIReader, scheme)
					err = c.Get(testCtx, configmapKey, p)
					Expect(err).ShouldNot(HaveOccurred())
				})
			})

			Context("List", func() {
				list := func(doCheck bool, f func(*wrappedReader, *topolvmv1.LogicalVolumeList)) {
					for i := 0; i < 2; i++ {
						lv := legacyLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						setLegacyLVStatus(lv, i)
						err = k8sDelegatedClient.Status().Update(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())
					}

					lvlist := new(topolvmv1.LogicalVolumeList)
					c := NewWrappedReader(k8sAPIReader, scheme)
					r := c.(*wrappedReader)
					f(r, lvlist)

					if doCheck {
						Expect(lvlist.Items).ShouldNot(HaveLen(0))
						for i, lv := range lvlist.Items {
							Expect(lv.Name).Should(Equal(fmt.Sprintf("legacy-%d", i)))
							Expect(lv.Spec.Name).Should(Equal(fmt.Sprintf("legacy-%d", i)))
							Expect(lv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
							Expect(lv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
							Expect(lv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
							Expect(lv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
							Expect(lv.Spec.AccessType).Should(Equal("rw"))
							Expect(lv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
							Expect(lv.Status.Code).Should(Equal(codes.Unknown))
							Expect(lv.Status.Message).Should(Equal(codes.Unknown.String()))
							Expect(lv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						}
					}
				}

				It("typedClient", func() {
					list(true, func(c *wrappedReader, lvlist *topolvmv1.LogicalVolumeList) {
						err := c.List(testCtx, lvlist)
						Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("unstructured.Unstructured", func() {
					list(true, func(c *wrappedReader, lvlist *topolvmv1.LogicalVolumeList) {
						u := &unstructured.UnstructuredList{}
						Expect(c.scheme.Convert(lvlist, u, nil)).ShouldNot(HaveOccurred())
						err := c.List(testCtx, u)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(c.scheme.Convert(u, lvlist, nil)).ShouldNot(HaveOccurred())
					})
				})

				It("metav1.PartialObjectMetadata", func() {
					list(false, func(c *wrappedReader, lvlist *topolvmv1.LogicalVolumeList) {
						p := &metav1.PartialObjectMetadataList{}
						p.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
						err := c.List(testCtx, p)
						Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("option test", func() {
					var opt1called bool
					opt1 := &fakeListOptions{
						f: func(opts *client.ListOptions) {
							opt1called = true
						},
					}
					var opt2called bool
					opt2 := &fakeListOptions{
						f: func(opts *client.ListOptions) {
							opt2called = true
						},
					}

					c := NewWrappedReader(k8sAPIReader, scheme)
					err := c.List(testCtx, new(topolvmv1.LogicalVolumeList), opt1, opt2)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(opt1called).Should(BeTrue())
					Expect(opt2called).Should(BeTrue())
				})

				It("non LogicalVolumeList object", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())

					list := new(corev1.ConfigMapList)
					c := NewWrappedReader(k8sAPIReader, scheme)
					err = c.List(testCtx, list)
					Expect(err).ShouldNot(HaveOccurred())
					list2 := new(corev1.ConfigMapList)
					err = k8sAPIReader.List(testCtx, list2)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(len(list.Items)).Should(Equal(len(list2.Items)))
				})

				It("non LogicalVolumeList unstructured.Unstructured", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())

					u := &unstructured.UnstructuredList{}
					u.SetGroupVersionKind(configMapListGVK)
					c := NewWrappedReader(k8sAPIReader, scheme)
					err = c.List(testCtx, u)
					Expect(err).ShouldNot(HaveOccurred())
					list := new(corev1.ConfigMapList)
					Expect(scheme.Convert(u, list, nil)).ShouldNot(HaveOccurred())
					list2 := new(corev1.ConfigMapList)
					err = k8sAPIReader.List(testCtx, list2)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(len(list.Items)).Should(Equal(len(list2.Items)))
				})

				It("non LogicalVolumeList metav1.PartialObjectMetadata", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())

					p := &metav1.PartialObjectMetadataList{}
					p.SetGroupVersionKind(configMapListGVK)
					c := NewWrappedReader(k8sAPIReader, scheme)
					err = c.List(testCtx, p)
					Expect(err).ShouldNot(HaveOccurred())
					list2 := new(corev1.ConfigMapList)
					err = k8sAPIReader.List(testCtx, list2)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(len(p.Items)).Should(Equal(len(list2.Items)))
				})
			})
		})

		Context("wrappedClient", func() {
			Context("Get", func() {
				get := func(doCheck bool, f func(Gomega, client.Client, *topolvmv1.LogicalVolume, string)) {
					i := 0
					lv := legacyLV(i)
					err := k8sDelegatedClient.Create(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					setLegacyLVStatus(lv, i)
					err = k8sDelegatedClient.Status().Update(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					Eventually(func(g Gomega) {
						checklv := new(topolvmv1.LogicalVolume)
						c := NewWrappedClient(k8sDelegatedClient)
						f(g, c, checklv, lv.Name)

						if doCheck {
							g.Expect(checklv.Name).Should(Equal(fmt.Sprintf("legacy-%d", i)))
							g.Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("legacy-%d", i)))
							g.Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
							g.Expect(checklv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
							g.Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
							g.Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
							g.Expect(checklv.Spec.AccessType).Should(Equal("rw"))
							g.Expect(checklv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
							g.Expect(checklv.Status.Code).Should(Equal(codes.Unknown))
							g.Expect(checklv.Status.Message).Should(Equal(codes.Unknown.String()))
							g.Expect(checklv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						}
					}).Should(Succeed())

				}

				It("typedClient", func() {
					get(true, func(g Gomega, c client.Client, lv *topolvmv1.LogicalVolume, name string) {
						err := c.Get(testCtx, types.NamespacedName{Name: name}, lv)
						g.Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("unstructured.Unstructured", func() {
					get(true, func(g Gomega, c client.Client, lv *topolvmv1.LogicalVolume, name string) {
						u := &unstructured.Unstructured{}
						g.Expect(c.Scheme().Convert(lv, u, nil)).ShouldNot(HaveOccurred())
						err := c.Get(testCtx, types.NamespacedName{Name: name}, u)
						g.Expect(err).ShouldNot(HaveOccurred())
						g.Expect(c.Scheme().Convert(u, lv, nil)).ShouldNot(HaveOccurred())
					})
				})

				It("metav1.PartialObjectMetadata", func() {
					get(false, func(g Gomega, c client.Client, lv *topolvmv1.LogicalVolume, name string) {
						p := &metav1.PartialObjectMetadata{}
						p.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
						lv.ObjectMeta.DeepCopyInto(&p.ObjectMeta)
						err := c.Get(testCtx, types.NamespacedName{Name: name}, p)
						g.Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("non LogicalVolume object", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())
					checkcm := new(corev1.ConfigMap)
					Eventually(func(g Gomega) {
						c := NewWrappedClient(k8sDelegatedClient)
						err = c.Get(testCtx, configmapKey, checkcm)
						g.Expect(err).ShouldNot(HaveOccurred())
					}).Should(Succeed())
				})

				It("non LogicalVolume unstructured.Unstructured", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())
					u := &unstructured.Unstructured{}
					u.SetGroupVersionKind(configMapGVK)
					Eventually(func(g Gomega) {
						c := NewWrappedClient(k8sDelegatedClient)
						err = c.Get(testCtx, configmapKey, u)
						g.Expect(err).ShouldNot(HaveOccurred())
					}).Should(Succeed())
				})

				It("non LogicalVolume metav1.PartialObjectMetadata", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())
					p := &metav1.PartialObjectMetadata{}
					p.SetGroupVersionKind(configMapGVK)
					Eventually(func(g Gomega) {
						c := NewWrappedClient(k8sDelegatedClient)
						err = c.Get(testCtx, configmapKey, p)
						g.Expect(err).ShouldNot(HaveOccurred())
					}).Should(Succeed())
				})
			})

			Context("List", func() {
				list := func(doCheck bool, f func(Gomega, *wrappedReader, *topolvmv1.LogicalVolumeList)) {
					for i := 0; i < 2; i++ {
						lv := legacyLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						setLegacyLVStatus(lv, i)
						err = k8sDelegatedClient.Status().Update(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())
					}

					Eventually(func(g Gomega) {
						lvlist := new(topolvmv1.LogicalVolumeList)
						c := NewWrappedReader(k8sAPIReader, scheme)
						r := c.(*wrappedReader)
						f(g, r, lvlist)

						if doCheck {
							for i, lv := range lvlist.Items {
								Expect(lv.Name).Should(Equal(fmt.Sprintf("legacy-%d", i)))
								Expect(lv.Spec.Name).Should(Equal(fmt.Sprintf("legacy-%d", i)))
								Expect(lv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
								Expect(lv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
								Expect(lv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
								Expect(lv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
								Expect(lv.Spec.AccessType).Should(Equal("rw"))
								Expect(lv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
								Expect(lv.Status.Code).Should(Equal(codes.Unknown))
								Expect(lv.Status.Message).Should(Equal(codes.Unknown.String()))
								Expect(lv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
							}
						}
					}).Should(Succeed())

				}

				It("typedClient", func() {
					list(true, func(g Gomega, c *wrappedReader, lvlist *topolvmv1.LogicalVolumeList) {
						err := c.List(testCtx, lvlist)
						g.Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("unstructured.Unstructured", func() {
					list(true, func(g Gomega, c *wrappedReader, lvlist *topolvmv1.LogicalVolumeList) {
						u := &unstructured.UnstructuredList{}
						g.Expect(c.scheme.Convert(lvlist, u, nil)).ShouldNot(HaveOccurred())
						err := c.List(testCtx, u)
						g.Expect(err).ShouldNot(HaveOccurred())
						g.Expect(c.scheme.Convert(u, lvlist, nil)).ShouldNot(HaveOccurred())
					})
				})

				It("metav1.PartialObjectMetadata", func() {
					list(false, func(g Gomega, c *wrappedReader, lvlist *topolvmv1.LogicalVolumeList) {
						p := &metav1.PartialObjectMetadataList{}
						p.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
						err := c.List(testCtx, p)
						g.Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("option test", func() {
					var opt1called bool
					opt1 := &fakeListOptions{
						f: func(opts *client.ListOptions) {
							opt1called = true
						},
					}
					var opt2called bool
					opt2 := &fakeListOptions{
						f: func(opts *client.ListOptions) {
							opt2called = true
						},
					}

					c := NewWrappedClient(k8sDelegatedClient)
					err := c.List(testCtx, new(topolvmv1.LogicalVolumeList), opt1, opt2)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(opt1called).Should(BeTrue())
					Expect(opt2called).Should(BeTrue())
				})

				It("non LogicalVolumeList object", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())

					Eventually(func(g Gomega) {
						list := new(corev1.ConfigMapList)
						c := NewWrappedClient(k8sDelegatedClient)
						err = c.List(testCtx, list)
						g.Expect(err).ShouldNot(HaveOccurred())
						list2 := new(corev1.ConfigMapList)
						err = k8sAPIReader.List(testCtx, list2)
						g.Expect(err).ShouldNot(HaveOccurred())
						g.Expect(len(list.Items)).Should(Equal(len(list2.Items)))
					}).Should(Succeed())
				})

				It("non LogicalVolumeList unstructured.Unstructured", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())

					Eventually(func(g Gomega) {
						u := &unstructured.UnstructuredList{}
						u.SetGroupVersionKind(configMapGVK)
						c := NewWrappedClient(k8sDelegatedClient)
						err = c.List(testCtx, u)
						g.Expect(err).ShouldNot(HaveOccurred())
						list2 := new(corev1.ConfigMapList)
						err = k8sAPIReader.List(testCtx, list2)
						g.Expect(err).ShouldNot(HaveOccurred())
						g.Expect(len(u.Items)).Should(Equal(len(list2.Items)))
					}).Should(Succeed())
				})

				It("non LogicalVolumeList metav1.PartialObjectMetadata", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())

					Eventually(func(g Gomega) {
						p := &metav1.PartialObjectMetadataList{}
						p.SetGroupVersionKind(configMapGVK)
						c := NewWrappedClient(k8sDelegatedClient)
						err = c.List(testCtx, p)
						g.Expect(err).ShouldNot(HaveOccurred())
						list2 := new(corev1.ConfigMapList)
						err = k8sAPIReader.List(testCtx, list2)
						g.Expect(err).ShouldNot(HaveOccurred())
						g.Expect(len(p.Items)).Should(Equal(len(list2.Items)))
					}).Should(Succeed())
				})
			})

			Context("Create", func() {
				create := func(doCheck bool, f func(client.Client, *topolvmv1.LogicalVolume)) {
					i := 0
					lv := currentLV(i)
					c := NewWrappedClient(k8sDelegatedClient)
					f(c, lv)

					if doCheck {
						checklv := new(topolvmlegacyv1.LogicalVolume)
						err := k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(checklv.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("current-%d", i)))
						Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
						Expect(checklv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
						Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
						Expect(checklv.Spec.AccessType).Should(Equal("rw"))
					}
				}

				It("typedClient", func() {
					create(true, func(c client.Client, lv *topolvmv1.LogicalVolume) {
						err := c.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("unstructured.Unstructured", func() {
					create(true, func(c client.Client, lv *topolvmv1.LogicalVolume) {
						u := &unstructured.Unstructured{}
						Expect(c.Scheme().Convert(lv, u, nil)).ShouldNot(HaveOccurred())
						err := c.Create(testCtx, u)
						Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("metav1.PartialObjectMetadata", func() {
					create(false, func(c client.Client, lv *topolvmv1.LogicalVolume) {
						p := &metav1.PartialObjectMetadata{}
						p.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
						lv.ObjectMeta.DeepCopyInto(&p.ObjectMeta)
						err := c.Create(testCtx, p)
						Expect(err).Should(HaveOccurred())
					})
				})

				It("non LogicalVolume object", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					c := NewWrappedClient(k8sDelegatedClient)
					err := c.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())
					err = k8sAPIReader.Get(testCtx, configmapKey, cm)
					Expect(err).ShouldNot(HaveOccurred())
				})

				It("non LogicalVolume unstructured.Unstructured", func() {
					u := &unstructured.Unstructured{}
					u.SetGroupVersionKind(configMapGVK)
					u.SetName(configmapName)
					u.SetNamespace(configmapNamespace)
					c := NewWrappedClient(k8sDelegatedClient)
					err := c.Create(testCtx, u)
					Expect(err).ShouldNot(HaveOccurred())
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err = k8sAPIReader.Get(testCtx, configmapKey, cm)
					Expect(err).ShouldNot(HaveOccurred())
				})
			})

			Context("Delete", func() {
				delete := func(f func(client.Client, *topolvmv1.LogicalVolume)) {
					i := 0
					lv := legacyLV(i)
					err := k8sDelegatedClient.Create(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					c := NewWrappedClient(k8sDelegatedClient)
					f(c, convertToCurrent(lv))

					checklv := new(topolvmlegacyv1.LogicalVolume)
					err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
					Expect(err).Should(HaveOccurred())
					Expect(apierrs.IsNotFound(err)).Should(BeTrue())
				}

				It("typedClient", func() {
					delete(func(c client.Client, lv *topolvmv1.LogicalVolume) {
						err := c.Delete(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("unstructured.Unstructured", func() {
					delete(func(c client.Client, lv *topolvmv1.LogicalVolume) {
						u := &unstructured.Unstructured{}
						Expect(c.Scheme().Convert(lv, u, nil)).ShouldNot(HaveOccurred())
						err := c.Delete(testCtx, u)
						Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("metav1.PartialObjectMetadata", func() {
					delete(func(c client.Client, lv *topolvmv1.LogicalVolume) {
						p := &metav1.PartialObjectMetadata{}
						p.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
						lv.ObjectMeta.DeepCopyInto(&p.ObjectMeta)
						err := c.Delete(testCtx, p)
						Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("non LogicalVolume object", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())

					c := NewWrappedClient(k8sDelegatedClient)
					err = c.Delete(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())

					err = k8sAPIReader.Get(testCtx, configmapKey, cm)
					Expect(err).Should(HaveOccurred())
					Expect(apierrs.IsNotFound(err)).Should(BeTrue())
				})

				It("non LogicalVolume unstructured.Unstructured", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())

					u := &unstructured.Unstructured{}
					u.SetGroupVersionKind(configMapGVK)
					u.SetName(configmapName)
					u.SetNamespace(configmapNamespace)
					c := NewWrappedClient(k8sDelegatedClient)
					err = c.Delete(testCtx, u)
					Expect(err).ShouldNot(HaveOccurred())

					err = k8sAPIReader.Get(testCtx, configmapKey, cm)
					Expect(err).Should(HaveOccurred())
					Expect(apierrs.IsNotFound(err)).Should(BeTrue())
				})

				It("non LogicalVolume metav1.PartialObjectMetadata", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())

					p := &metav1.PartialObjectMetadata{}
					p.SetGroupVersionKind(configMapGVK)
					p.SetName(configmapName)
					p.SetNamespace(configmapNamespace)
					c := NewWrappedClient(k8sDelegatedClient)
					err = c.Delete(testCtx, p)
					Expect(err).ShouldNot(HaveOccurred())

					err = k8sAPIReader.Get(testCtx, configmapKey, cm)
					Expect(err).Should(HaveOccurred())
					Expect(apierrs.IsNotFound(err)).Should(BeTrue())
				})
			})

			Context("Update", func() {
				update := func(doCheck bool, f func(client.Client, *topolvmv1.LogicalVolume)) {
					i := 0
					lv := legacyLV(i)
					err := k8sDelegatedClient.Create(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					ann := map[string]string{"foo": "bar"}
					lv2 := convertToCurrent(lv)
					lv2.Annotations = ann
					lv2.Spec.Name = fmt.Sprintf("updated-legacy-%d", i)
					lv2.Spec.NodeName = fmt.Sprintf("updated-node-%d", i)
					c := NewWrappedClient(k8sDelegatedClient)
					f(c, lv2)

					if doCheck {
						checklv := new(topolvmlegacyv1.LogicalVolume)
						err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(checklv.Annotations).Should(Equal(ann))
						Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("updated-legacy-%d", i)))
						Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("updated-node-%d", i)))
						Expect(checklv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
						Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
						Expect(checklv.Spec.AccessType).Should(Equal("rw"))
					}
				}

				It("typedClient", func() {
					update(true, func(c client.Client, lv *topolvmv1.LogicalVolume) {
						err := c.Update(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("unstructured.Unstructured", func() {
					update(true, func(c client.Client, lv *topolvmv1.LogicalVolume) {
						u := &unstructured.Unstructured{}
						Expect(c.Scheme().Convert(lv, u, nil)).ShouldNot(HaveOccurred())
						err := c.Update(testCtx, u)
						Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("metav1.PartialObjectMetadata", func() {
					update(false, func(c client.Client, lv *topolvmv1.LogicalVolume) {
						p := &metav1.PartialObjectMetadata{}
						p.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
						lv.ObjectMeta.DeepCopyInto(&p.ObjectMeta)
						err := c.Update(testCtx, p)
						Expect(err).Should(HaveOccurred())
					})
				})

				It("non LogicalVolume object", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())

					ann := map[string]string{"foo": "bar"}
					cm2 := cm.DeepCopy()
					cm2.Annotations = ann
					c := NewWrappedClient(k8sDelegatedClient)
					err = c.Update(testCtx, cm2)
					Expect(err).ShouldNot(HaveOccurred())

					cm3 := new(corev1.ConfigMap)
					err = k8sAPIReader.Get(testCtx, configmapKey, cm3)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(cm3.Annotations).Should(Equal(ann))
				})

				It("non LogicalVolume unstructured.Unstructured", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())

					ann := map[string]string{"foo": "bar"}
					cm2 := cm.DeepCopy()
					cm2.Annotations = ann
					u := &unstructured.Unstructured{}
					Expect(scheme.Convert(cm2, u, nil)).ShouldNot(HaveOccurred())
					c := NewWrappedClient(k8sDelegatedClient)
					err = c.Update(testCtx, u)
					Expect(err).ShouldNot(HaveOccurred())

					cm3 := new(corev1.ConfigMap)
					err = k8sAPIReader.Get(testCtx, configmapKey, cm3)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(cm3.Annotations).Should(Equal(ann))
				})
			})

			Context("Patch", func() {
				patch := func(doCheck bool, f func(client.Client, *topolvmv1.LogicalVolume, client.Patch)) {
					i := 0
					lv := legacyLV(i)
					err := k8sDelegatedClient.Create(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					ann := map[string]string{"foo": "bar"}
					lv2 := convertToCurrent(lv)
					lv2.TypeMeta.APIVersion = topolvmlegacyv1.GroupVersion.String()
					lv3 := lv2.DeepCopy()
					lv3.Annotations = ann
					lv3.Spec.Name = fmt.Sprintf("updated-legacy-%d", i)
					lv3.Spec.NodeName = fmt.Sprintf("updated-node-%d", i)
					patch := client.MergeFrom(lv2)
					c := NewWrappedClient(k8sDelegatedClient)
					f(c, lv3, patch)

					checklv := new(topolvmlegacyv1.LogicalVolume)
					err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(checklv.Annotations).Should(Equal(ann))
					if doCheck {
						Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("updated-legacy-%d", i)))
						Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("updated-node-%d", i)))
						Expect(checklv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
						Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
						Expect(checklv.Spec.AccessType).Should(Equal("rw"))
					}
				}

				It("typedClient", func() {
					patch(true, func(c client.Client, lv *topolvmv1.LogicalVolume, patch client.Patch) {
						err := c.Patch(testCtx, lv, patch)
						Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("unstructured.Unstructured", func() {
					patch(true, func(c client.Client, lv *topolvmv1.LogicalVolume, patch client.Patch) {
						u := &unstructured.Unstructured{}
						Expect(c.Scheme().Convert(lv, u, nil)).ShouldNot(HaveOccurred())
						err := c.Patch(testCtx, u, patch)
						Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("metav1.PartialObjectMetadata", func() {
					patch(false, func(c client.Client, lv *topolvmv1.LogicalVolume, patch client.Patch) {
						p := &metav1.PartialObjectMetadata{}
						p.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
						lv.ObjectMeta.DeepCopyInto(&p.ObjectMeta)
						err := c.Patch(testCtx, p, patch)
						Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("non LogicalVolume object", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())

					ann := map[string]string{"foo": "bar"}
					cm2 := cm.DeepCopy()
					cm3 := cm2.DeepCopy()
					cm3.Annotations = ann
					patch := client.MergeFrom(cm2)
					c := NewWrappedClient(k8sDelegatedClient)
					err = c.Patch(testCtx, cm3, patch)
					Expect(err).ShouldNot(HaveOccurred())

					cm4 := new(corev1.ConfigMap)
					err = k8sAPIReader.Get(testCtx, configmapKey, cm4)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(cm4.Annotations).Should(Equal(ann))
				})

				It("non LogicalVolume unstructured.Unstructured", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())

					ann := map[string]string{"foo": "bar"}
					cm2 := cm.DeepCopy()
					cm3 := cm2.DeepCopy()
					cm3.Annotations = ann
					u := &unstructured.Unstructured{}
					Expect(scheme.Convert(cm3, u, nil)).ShouldNot(HaveOccurred())
					patch := client.MergeFrom(cm2)
					c := NewWrappedClient(k8sDelegatedClient)
					err = c.Patch(testCtx, u, patch)
					Expect(err).ShouldNot(HaveOccurred())

					cm4 := new(corev1.ConfigMap)
					err = k8sAPIReader.Get(testCtx, configmapKey, cm4)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(cm4.Annotations).Should(Equal(ann))
				})

				It("non LogicalVolume metav1.PartialObjectMetadata", func() {
					cm := new(corev1.ConfigMap)
					cm.Name = configmapName
					cm.Namespace = configmapNamespace
					err := k8sDelegatedClient.Create(testCtx, cm)
					Expect(err).ShouldNot(HaveOccurred())

					ann := map[string]string{"foo": "bar"}
					p := &metav1.PartialObjectMetadata{}
					p.SetGroupVersionKind(configMapGVK)
					p.SetName(configmapName)
					p.SetNamespace(configmapNamespace)
					p.SetAnnotations(ann)
					patch := client.MergeFrom(cm)
					p.SetAnnotations(ann)
					c := NewWrappedClient(k8sDelegatedClient)
					err = c.Patch(testCtx, p, patch)
					Expect(err).ShouldNot(HaveOccurred())

					cm2 := new(corev1.ConfigMap)
					err = k8sAPIReader.Get(testCtx, configmapKey, cm2)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(cm2.Annotations).Should(Equal(ann))
				})
			})

			Context("DeleteAllOf", func() {
				deleteAllOf := func(f func(client.Client, *topolvmv1.LogicalVolume)) {
					for i := 0; i < 2; i++ {
						lv := legacyLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())
					}

					checklvlist := new(topolvmlegacyv1.LogicalVolumeList)
					err := k8sAPIReader.List(testCtx, checklvlist)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(len(checklvlist.Items)).Should(Equal(2))

					lv := new(topolvmv1.LogicalVolume)
					c := NewWrappedClient(k8sDelegatedClient)
					f(c, lv)

					checklvlist = new(topolvmlegacyv1.LogicalVolumeList)
					err = k8sAPIReader.List(testCtx, checklvlist)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(len(checklvlist.Items)).Should(Equal(0))
				}

				It("typedClient", func() {
					deleteAllOf(func(c client.Client, lv *topolvmv1.LogicalVolume) {
						err := c.DeleteAllOf(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("unstructured.Unstructured", func() {
					deleteAllOf(func(c client.Client, lv *topolvmv1.LogicalVolume) {
						u := &unstructured.Unstructured{}
						Expect(c.Scheme().Convert(lv, u, nil)).ShouldNot(HaveOccurred())
						err := c.DeleteAllOf(testCtx, u)
						Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("metav1.PartialObjectMetadata", func() {
					deleteAllOf(func(c client.Client, lv *topolvmv1.LogicalVolume) {
						p := &metav1.PartialObjectMetadata{}
						p.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
						lv.ObjectMeta.DeepCopyInto(&p.ObjectMeta)
						err := c.DeleteAllOf(testCtx, p)
						Expect(err).ShouldNot(HaveOccurred())
					})
				})

				It("typedClient", func() {
					for i := 0; i < 2; i++ {
						lv := legacyLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())
					}

					checklvlist := new(topolvmlegacyv1.LogicalVolumeList)
					err := k8sAPIReader.List(testCtx, checklvlist)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(len(checklvlist.Items)).Should(Equal(2))

					lv := new(topolvmv1.LogicalVolume)
					c := NewWrappedClient(k8sDelegatedClient)
					err = c.DeleteAllOf(testCtx, lv)
					Expect(err).ShouldNot(HaveOccurred())

					checklvlist = new(topolvmlegacyv1.LogicalVolumeList)
					err = k8sAPIReader.List(testCtx, checklvlist)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(len(checklvlist.Items)).Should(Equal(0))
				})

				It("non LogicalVolume object", func() {
					Skip("If you found resources other than LogicalVolume resource what DeleteAllOf can be executed in the envtest environment, please write test cases for these resources.")
				})

				It("non LogicalVolume unstructured.Unstructured", func() {
					Skip("If you found resources other than LogicalVolume resource what DeleteAllOf can be executed in the envtest environment, please write test cases for these resources.")
				})

				It("non LogicalVolume metav1.PartialObjectMetadata", func() {
					Skip("If you found resources other than LogicalVolume resource what DeleteAllOf can be executed in the envtest environment, please write test cases for these resources.")
				})
			})

			Context("SubResourceClient", func() {
				Context("Get", func() {
					It("should not be implemented", func() {
						i := 0
						lv := currentLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						c := NewWrappedClient(k8sDelegatedClient)
						err = c.SubResource("status").Get(testCtx, lv, nil)
						Expect(err).Should(HaveOccurred())
					})
				})

				Context("Create", func() {
					It("should not be implemented", func() {
						i := 0
						lv := currentLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						c := NewWrappedClient(k8sDelegatedClient)
						err = c.SubResource("status").Create(testCtx, lv, nil)
						Expect(err).Should(HaveOccurred())
					})
				})
			})

			Context("SubResourceWriter", func() {
				BeforeEach(func() {
					svc := &corev1.Service{}
					svc.Name = configmapName
					svc.Namespace = configmapNamespace
					k8sDelegatedClient.Delete(testCtx, svc)
				})

				Context("Update", func() {
					update := func(doCheck bool, f func(client.Client, *topolvmv1.LogicalVolume)) {
						i := 0
						lv := legacyLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						lv2 := convertToCurrent(lv)
						setCurrentLVStatus(lv2, i)
						c := NewWrappedClient(k8sDelegatedClient)
						f(c, lv2)

						if doCheck {
							checklv := new(topolvmlegacyv1.LogicalVolume)
							err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
							Expect(err).ShouldNot(HaveOccurred())
							Expect(checklv.Name).Should(Equal(fmt.Sprintf("legacy-%d", i)))
							Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("legacy-%d", i)))
							Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
							Expect(checklv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
							Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
							Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
							Expect(checklv.Spec.AccessType).Should(Equal("rw"))
							Expect(checklv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
							Expect(checklv.Status.Code).Should(Equal(codes.Unknown))
							Expect(checklv.Status.Message).Should(Equal(codes.Unknown.String()))
							Expect(checklv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						}
					}

					It("typedClient", func() {
						update(true, func(c client.Client, lv *topolvmv1.LogicalVolume) {
							err := c.Status().Update(testCtx, lv)
							Expect(err).ShouldNot(HaveOccurred())
						})
					})

					It("unstructured.Unstructured", func() {
						update(true, func(c client.Client, lv *topolvmv1.LogicalVolume) {
							u := &unstructured.Unstructured{}
							Expect(c.Scheme().Convert(lv, u, nil)).ShouldNot(HaveOccurred())
							err := c.Status().Update(testCtx, u)
							Expect(err).ShouldNot(HaveOccurred())
						})
					})

					It("metav1.PartialObjectMetadata", func() {
						update(false, func(c client.Client, lv *topolvmv1.LogicalVolume) {
							p := &metav1.PartialObjectMetadata{}
							p.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
							lv.ObjectMeta.DeepCopyInto(&p.ObjectMeta)
							err := c.Status().Update(testCtx, p)
							Expect(err).Should(HaveOccurred())
						})
					})

					It("non LogicalVolume object", func() {
						svc := new(corev1.Service)
						svc.Name = configmapName
						svc.Namespace = configmapNamespace
						svc.Spec = corev1.ServiceSpec{
							Selector: map[string]string{"app": "MyApp"},
							Ports: []corev1.ServicePort{
								{
									Protocol: corev1.ProtocolTCP,
									Port:     80,
								},
							},
						}
						err := k8sDelegatedClient.Create(testCtx, svc)
						Expect(err).ShouldNot(HaveOccurred())
						defer func() {
							err := k8sDelegatedClient.Delete(testCtx, svc)
							Expect(err).ShouldNot(HaveOccurred())
						}()

						svc2 := svc.DeepCopy()
						lbstatus := corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{
									Hostname: "example.com",
								},
							},
						}
						svc2.Status.LoadBalancer = lbstatus
						c := NewWrappedClient(k8sDelegatedClient)
						err = c.Status().Update(testCtx, svc2)
						Expect(err).ShouldNot(HaveOccurred())

						svc3 := new(corev1.Service)
						err = k8sAPIReader.Get(testCtx, configmapKey, svc3)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(svc3.Status.LoadBalancer).Should(Equal(lbstatus))
					})

					It("non LogicalVolume unstructured.Unstructured", func() {
						svc := new(corev1.Service)
						svc.Name = configmapName
						svc.Namespace = configmapNamespace
						svc.Spec = corev1.ServiceSpec{
							Selector: map[string]string{"app": "MyApp"},
							Ports: []corev1.ServicePort{
								{
									Protocol: corev1.ProtocolTCP,
									Port:     80,
								},
							},
						}
						err := k8sDelegatedClient.Create(testCtx, svc)
						Expect(err).ShouldNot(HaveOccurred())
						defer func() {
							err := k8sDelegatedClient.Delete(testCtx, svc)
							Expect(err).ShouldNot(HaveOccurred())
						}()

						svc2 := svc.DeepCopy()
						lbstatus := corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{
									Hostname: "example.com",
								},
							},
						}
						svc2.Status.LoadBalancer = lbstatus
						u := &unstructured.Unstructured{}
						u.SetGroupVersionKind(configMapGVK)
						Expect(scheme.Convert(svc2, u, nil)).ShouldNot(HaveOccurred())

						c := NewWrappedClient(k8sDelegatedClient)
						err = c.Status().Update(testCtx, u)
						Expect(err).ShouldNot(HaveOccurred())

						svc3 := new(corev1.Service)
						err = k8sAPIReader.Get(testCtx, configmapKey, svc3)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(svc3.Status.LoadBalancer).Should(Equal(lbstatus))
					})
				})

				Context("Patch", func() {
					patch := func(doCheck bool, f func(client.Client, *topolvmv1.LogicalVolume, client.Patch)) {
						i := 0
						lv := legacyLV(i)
						err := k8sDelegatedClient.Create(testCtx, lv)
						Expect(err).ShouldNot(HaveOccurred())

						lv2 := convertToCurrent(lv)
						lv2.TypeMeta.APIVersion = topolvmlegacyv1.GroupVersion.String()
						lv3 := lv2.DeepCopy()
						setCurrentLVStatus(lv3, i)
						patch := client.MergeFrom(lv2)
						c := NewWrappedClient(k8sDelegatedClient)
						f(c, lv3, patch)

						checklv := new(topolvmlegacyv1.LogicalVolume)
						err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
						Expect(err).ShouldNot(HaveOccurred())
						if doCheck {
							Expect(checklv.Name).Should(Equal(fmt.Sprintf("legacy-%d", i)))
							Expect(checklv.Spec.Name).Should(Equal(fmt.Sprintf("legacy-%d", i)))
							Expect(checklv.Spec.NodeName).Should(Equal(fmt.Sprintf("node-%d", i)))
							Expect(checklv.Spec.DeviceClass).Should(Equal(topolvm.DefaultDeviceClassName))
							Expect(checklv.Spec.Size.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
							Expect(checklv.Spec.Source).Should(Equal(fmt.Sprintf("source-%d", i)))
							Expect(checklv.Spec.AccessType).Should(Equal("rw"))
							Expect(checklv.Status.VolumeID).Should(Equal(fmt.Sprintf("volume-%d", i)))
							Expect(checklv.Status.Code).Should(Equal(codes.Unknown))
							Expect(checklv.Status.Message).Should(Equal(codes.Unknown.String()))
							Expect(checklv.Status.CurrentSize.Value()).Should(Equal(resource.NewQuantity(1<<30, resource.BinarySI).Value()))
						}
					}

					It("typedClient", func() {
						patch(true, func(c client.Client, lv *topolvmv1.LogicalVolume, patch client.Patch) {
							err := c.Status().Patch(testCtx, lv, patch)
							Expect(err).ShouldNot(HaveOccurred())
						})
					})

					It("unstructured.Unstructured", func() {
						patch(true, func(c client.Client, lv *topolvmv1.LogicalVolume, patch client.Patch) {
							u := &unstructured.Unstructured{}
							Expect(c.Scheme().Convert(lv, u, nil)).ShouldNot(HaveOccurred())
							err := c.Status().Patch(testCtx, u, patch)
							Expect(err).ShouldNot(HaveOccurred())
						})
					})

					It("metav1.PartialObjectMetadata", func() {
						patch(false, func(c client.Client, lv *topolvmv1.LogicalVolume, patch client.Patch) {
							p := &metav1.PartialObjectMetadata{}
							p.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind(kind))
							lv.ObjectMeta.DeepCopyInto(&p.ObjectMeta)
							err := c.Status().Patch(testCtx, p, patch)
							Expect(err).ShouldNot(HaveOccurred())
						})
					})

					It("non LogicalVolume object", func() {
						svc := new(corev1.Service)
						svc.Name = configmapName
						svc.Namespace = configmapNamespace
						svc.Spec = corev1.ServiceSpec{
							Selector: map[string]string{"app": "MyApp"},
							Ports: []corev1.ServicePort{
								{
									Protocol: corev1.ProtocolTCP,
									Port:     80,
								},
							},
						}
						err := k8sDelegatedClient.Create(testCtx, svc)
						Expect(err).ShouldNot(HaveOccurred())

						svc2 := svc.DeepCopy()
						svc3 := svc2.DeepCopy()
						lbstatus := corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{
									Hostname: "example.com",
								},
							},
						}
						svc3.Status.LoadBalancer = lbstatus
						patch := client.MergeFrom(svc2)
						c := NewWrappedClient(k8sDelegatedClient)
						err = c.Status().Patch(testCtx, svc3, patch)
						Expect(err).ShouldNot(HaveOccurred())

						svc4 := new(corev1.Service)
						err = k8sAPIReader.Get(testCtx, configmapKey, svc4)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(svc4.Status.LoadBalancer).Should(Equal(lbstatus))
					})

					It("non LogicalVolume unstructured.Unstructured", func() {
						svc := new(corev1.Service)
						svc.Name = configmapName
						svc.Namespace = configmapNamespace
						svc.Spec = corev1.ServiceSpec{
							Selector: map[string]string{"app": "MyApp"},
							Ports: []corev1.ServicePort{
								{
									Protocol: corev1.ProtocolTCP,
									Port:     80,
								},
							},
						}
						err := k8sDelegatedClient.Create(testCtx, svc)
						Expect(err).ShouldNot(HaveOccurred())

						svc2 := svc.DeepCopy()
						svc3 := svc2.DeepCopy()
						lbstatus := corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{
									Hostname: "example.com",
								},
							},
						}
						svc3.Status.LoadBalancer = lbstatus
						patch := client.MergeFrom(svc2)
						u := &unstructured.Unstructured{}
						Expect(scheme.Convert(svc3, u, nil)).ShouldNot(HaveOccurred())
						c := NewWrappedClient(k8sDelegatedClient)
						err = c.Status().Patch(testCtx, u, patch)
						Expect(err).ShouldNot(HaveOccurred())

						svc4 := new(corev1.Service)
						err = k8sAPIReader.Get(testCtx, configmapKey, svc4)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(svc4.Status.LoadBalancer).Should(Equal(lbstatus))
					})

					It("non LogicalVolume metav1.PartialObjectMetadata", func() {
						svc := new(corev1.Service)
						svc.Name = configmapName
						svc.Namespace = configmapNamespace
						svc.Spec = corev1.ServiceSpec{
							Selector: map[string]string{"app": "MyApp"},
							Ports: []corev1.ServicePort{
								{
									Protocol: corev1.ProtocolTCP,
									Port:     80,
								},
							},
						}
						err := k8sDelegatedClient.Create(testCtx, svc)
						Expect(err).ShouldNot(HaveOccurred())

						ann := map[string]string{"foo": "bar"}
						svc2 := svc.DeepCopy()
						svc2.Spec = corev1.ServiceSpec{}
						svc2.Status = corev1.ServiceStatus{}
						patch := client.MergeFrom(svc2)
						p := &metav1.PartialObjectMetadata{}
						p.SetGroupVersionKind(svc2.GroupVersionKind().GroupVersion().WithKind("Service"))
						p.SetName(configmapName)
						p.SetNamespace(configmapNamespace)
						p.SetAnnotations(ann)
						c := NewWrappedClient(k8sDelegatedClient)
						err = c.Status().Patch(testCtx, p, patch)
						Expect(err).ShouldNot(HaveOccurred())

						svc4 := new(corev1.Service)
						err = k8sAPIReader.Get(testCtx, configmapKey, svc4)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(svc4.Annotations).Should(Equal(ann))
					})
				})
			})
		})
	})
})
