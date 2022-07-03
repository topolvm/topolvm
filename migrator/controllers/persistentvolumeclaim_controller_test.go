package controllers

import (
	"fmt"
	"sort"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/topolvm/topolvm"
)

var testPVCNamespace = "test-pvc"

func testPVC() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc",
			Namespace: testPVCNamespace,
			Finalizers: []string{
				topolvm.LegacyPVCFinalizer,
				"aaa/bbb",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: *resource.NewQuantity(1<<30, resource.BinarySI),
				},
			},
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: strPtr("topolvm-storageclass"),
			VolumeMode:       volumeModePtr(corev1.PersistentVolumeFilesystem),
		},
	}
}

var _ = Describe("test persistentvolumeclaim controller", func() {
	BeforeEach(func() {
		ns := &corev1.Namespace{}
		ns.Name = testPVCNamespace
		err := k8sClient.Create(testCtx, ns)
		Expect(err).ShouldNot(HaveOccurred())
	})
	AfterEach(func() {
		ns := &corev1.Namespace{}
		ns.Name = testPVCNamespace
		err := k8sClient.Delete(testCtx, ns)
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("should migrate pvc finalizer", func() {
		By("create a pvc")
		pvc := testPVC()
		err := k8sClient.Create(testCtx, pvc)
		Expect(err).ShouldNot(HaveOccurred())

		By("check a pvc data")
		expetedFinalizers := []string{
			// Unrelated finalizers does not remove
			"aaa/bbb",
			PVCProtectionFinalizer, // this finalizer added from api-server automatically

			// Replaces the legacy TopoLVM finalizer
			topolvm.PVCFinalizer,
		}
		sort.Strings(expetedFinalizers)
		name := types.NamespacedName{
			Name:      pvc.Name,
			Namespace: pvc.Namespace,
		}
		Eventually(func() error {
			c := &corev1.PersistentVolumeClaim{}
			err := k8sClient.Get(testCtx, name, c)
			if err != nil {
				return fmt.Errorf("can not get target pvc: err=%v", err)
			}
			sort.Strings(c.Finalizers)
			if diff := cmp.Diff(expetedFinalizers, c.Finalizers); diff != "" {
				return fmt.Errorf("pvc finalizers does not match: (-want,+got):\n%s", diff)
			}
			return nil
		}).Should(Succeed())
	})
})
