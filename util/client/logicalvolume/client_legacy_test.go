package logicalvolume

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/util/conversion"
	"google.golang.org/grpc/codes"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("wrapped logicalvolume client(current)", func() {
	BeforeEach(func() {
		os.Setenv("USE_LEGACY_PLUGIN_NAME", "true")
		err := k8sDelegatedClient.DeleteAllOf(testCtx, &topolvmv1.LogicalVolume{})
		Expect(err).ShouldNot(HaveOccurred())
		err = k8sDelegatedClient.DeleteAllOf(testCtx, &topolvmlegacyv1.LogicalVolume{})
		Expect(err).ShouldNot(HaveOccurred())
	})

	Context("List", func() {
		It("standard case", func() {
			for i := 0; i < 2; i++ {
				lv := legacyLV(i)
				err := k8sDelegatedClient.Create(testCtx, lv)
				Expect(err).ShouldNot(HaveOccurred())

				setLegacyLVStatus(lv, i)
				err = k8sDelegatedClient.Status().Update(testCtx, lv)
				Expect(err).ShouldNot(HaveOccurred())
			}

			lvlist := new(topolvmv1.LogicalVolumeList)
			err := List(testCtx, k8sAPIReader, lvlist)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(lvlist.Items)).Should(Equal(2))
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
		})
	})

	Context("Get", func() {
		It("standard case", func() {
			i := 0
			lv := legacyLV(i)
			err := k8sDelegatedClient.Create(testCtx, lv)
			Expect(err).ShouldNot(HaveOccurred())

			setLegacyLVStatus(lv, i)
			err = k8sDelegatedClient.Status().Update(testCtx, lv)
			Expect(err).ShouldNot(HaveOccurred())

			checklv := new(topolvmv1.LogicalVolume)
			err = Get(testCtx, k8sAPIReader, types.NamespacedName{Name: lv.Name}, checklv)
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
		})
	})

	Context("Create", func() {
		It("standard case", func() {
			i := 0
			lv := currentLV(i)
			err := Create(testCtx, k8sDelegatedClient, lv)
			Expect(err).ShouldNot(HaveOccurred())

			checklv := new(topolvmlegacyv1.LogicalVolume)
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

	Context("Update", func() {
		It("standard case", func() {
			i := 0
			lv := legacyLV(i)
			err := k8sDelegatedClient.Create(testCtx, lv)
			Expect(err).ShouldNot(HaveOccurred())

			ann := map[string]string{"foo": "bar"}
			lv2 := conversion.ConvertToCurrent(lv)
			lv2.Annotations = ann
			lv2.Spec.Name = fmt.Sprintf("updated-legacy-%d", i)
			lv2.Spec.NodeName = fmt.Sprintf("updated-node-%d", i)
			err = Update(testCtx, k8sDelegatedClient, lv2)
			Expect(err).ShouldNot(HaveOccurred())

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
		})
	})

	Context("Patch", func() {
		It("standard case", func() {
			i := 0
			lv := legacyLV(i)
			err := k8sDelegatedClient.Create(testCtx, lv)
			Expect(err).ShouldNot(HaveOccurred())

			ann := map[string]string{"foo": "bar"}
			lv2 := conversion.ConvertToCurrent(lv)
			lv3 := lv2.DeepCopy()
			lv3.Annotations = ann
			lv3.Spec.Name = fmt.Sprintf("updated-legacy-%d", i)
			lv3.Spec.NodeName = fmt.Sprintf("updated-node-%d", i)
			patchFn := func(obj client.Object) (client.Patch, error) {
				return client.MergeFrom(obj), nil
			}
			err = Patch(testCtx, k8sDelegatedClient, lv3, lv2, patchFn)
			Expect(err).ShouldNot(HaveOccurred())

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
		})
	})

	Context("Delete", func() {
		It("standard case", func() {
			i := 0
			lv := legacyLV(i)
			err := k8sDelegatedClient.Create(testCtx, lv)
			Expect(err).ShouldNot(HaveOccurred())

			err = Delete(testCtx, k8sDelegatedClient, conversion.ConvertToCurrent(lv))
			Expect(err).ShouldNot(HaveOccurred())

			checklv := new(topolvmlegacyv1.LogicalVolume)
			err = k8sAPIReader.Get(testCtx, types.NamespacedName{Name: lv.Name}, checklv)
			Expect(err).Should(HaveOccurred())
			Expect(apierrs.IsNotFound(err)).Should(BeTrue())
		})
	})

	Context("StatusUpdate", func() {
		It("standard case", func() {
			i := 0
			lv := legacyLV(i)
			err := k8sDelegatedClient.Create(testCtx, lv)
			Expect(err).ShouldNot(HaveOccurred())

			lv2 := conversion.ConvertToCurrent(lv)
			setCurrentLVStatus(lv2, i)
			err = StatusUpdate(testCtx, k8sDelegatedClient, lv2)
			Expect(err).ShouldNot(HaveOccurred())

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
		})
	})
})
