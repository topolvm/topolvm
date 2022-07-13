package controllers

import (
	"fmt"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/topolvm/topolvm"
)

func testSC() *storagev1.StorageClass {
	policy := corev1.PersistentVolumeReclaimDelete
	mode := storagev1.VolumeBindingWaitForFirstConsumer
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sc",
		},
		Parameters: map[string]string{
			"csi.storage.k8s.io/fstype":  "xfs",
			topolvm.LegacyDeviceClassKey: topolvm.DefaultDeviceClassAnnotationName,
		},
		Provisioner:       topolvm.LegacyPluginName,
		ReclaimPolicy:     &policy,
		VolumeBindingMode: &mode,
	}
}

var _ = Describe("test storageclass controller", func() {
	It("should migrate sc", func() {
		By("create a sc")
		sc := testSC()
		err := k8sClient.Create(testCtx, sc)
		Expect(err).ShouldNot(HaveOccurred())

		By("check a sc data")
		expectedProvisioner := topolvm.PluginName
		expectedParameters := map[string]string{
			"csi.storage.k8s.io/fstype": "xfs",
			topolvm.DeviceClassKey:      topolvm.DefaultDeviceClassAnnotationName,
		}
		name := types.NamespacedName{
			Name: sc.Name,
		}
		Eventually(func() error {
			migratedSC := &storagev1.StorageClass{}
			err := k8sClient.Get(testCtx, name, migratedSC)
			if err != nil {
				return fmt.Errorf("can not get target sc: err=%v", err)
			}
			if diff := cmp.Diff(expectedProvisioner, migratedSC.Provisioner); diff != "" {
				return fmt.Errorf("provisioner does not match: (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(expectedParameters, migratedSC.Parameters); diff != "" {
				return fmt.Errorf("parameters does not match: (-want,+got):\n%s", diff)
			}
			return nil
		}).Should(Succeed())
	})
})
