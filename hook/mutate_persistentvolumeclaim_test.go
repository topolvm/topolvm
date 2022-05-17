package hook

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
)

const (
	mutatePVCNamespace = "test-mutate-pvc"
	defaultPVCName     = "test-pvc"
)

func setupMutatePVCResources() {
	// Namespace and namespace resources
	ns := &corev1.Namespace{}
	ns.Name = mutatePVCNamespace
	err := k8sClient.Create(testCtx, ns)
	Expect(err).ShouldNot(HaveOccurred())
}

func createPVC(sc string, pvcName string) {
	pvc := &corev1.PersistentVolumeClaim{}
	pvc.Namespace = mutatePVCNamespace
	pvc.Name = pvcName
	pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	if sc != "" {
		pvc.Spec.StorageClassName = strPtr(sc)
	}
	pvc.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(10<<30, resource.DecimalSI),
	}
	err := k8sClient.Create(testCtx, pvc)
	Expect(err).ShouldNot(HaveOccurred())
}

func getPVC(pvcName string) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	name := types.NamespacedName{
		Namespace: mutatePVCNamespace,
		Name:      pvcName,
	}
	err := k8sClient.Get(testCtx, name, pvc)
	return pvc, err
}

func hasTopoLVMFinalizer(pvc *corev1.PersistentVolumeClaim) bool {
	for _, fin := range pvc.Finalizers {
		if fin == topolvm.PVCFinalizer {
			return true
		}
	}
	return false
}

var _ = Describe("pvc mutation webhook", func() {
	It("should not have topolvm.io/pvc finalizer when not specified storageclass", func() {
		pvcName := "empty-storageclass-pvc"
		createPVC("", pvcName)
		pvc, err := getPVC(pvcName)
		Expect(err).ShouldNot(HaveOccurred())
		hasFinalizer := hasTopoLVMFinalizer(pvc)
		Expect(hasFinalizer).Should(Equal(false), "finalizer should not be set for storageclass=%s", hostLocalStorageClassName)
	})

	It("should not have topolvm.io/pvc finalizer when the specified StorageClass does not exist", func() {
		pvcName := "unexists-storageclass-pvc"
		createPVC(missingStorageClassName, pvcName)
		pvc, err := getPVC(pvcName)
		Expect(err).ShouldNot(HaveOccurred())
		hasFinalizer := hasTopoLVMFinalizer(pvc)
		Expect(hasFinalizer).Should(Equal(false), "finalizer should not be set for storageclass=%s", missingStorageClassName)
	})

	It("should not have topolvm.io/pvc finalizer with storageclass host-local", func() {
		pvcName := "host-local-pvc"
		createPVC(hostLocalStorageClassName, pvcName)
		pvc, err := getPVC(pvcName)
		Expect(err).ShouldNot(HaveOccurred())
		hasFinalizer := hasTopoLVMFinalizer(pvc)
		Expect(hasFinalizer).Should(Equal(false), "finalizer should not be set for storageclass=%s", hostLocalStorageClassName)
	})

	It("should have topolvm.io/pvc finalizer with storageclass topolvm-provisioner", func() {
		pvcName := "topolvm-provisioner-pvc"
		createPVC(topolvmProvisionerStorageClassName, pvcName)
		pvc, err := getPVC(pvcName)
		Expect(err).ShouldNot(HaveOccurred())
		hasFinalizer := hasTopoLVMFinalizer(pvc)
		Expect(hasFinalizer).Should(Equal(true), "finalizer should be set for storageclass=%s", topolvmProvisionerStorageClassName)
	})

	It("should have topolvm.io/pvc finalizer with storageclass topolvm-provisioner-immediate", func() {
		pvcName := "topolvm-provisioner-immediate-pvc"
		createPVC(topolvmProvisionerImmediateStorageClassName, pvcName)
		pvc, err := getPVC(pvcName)
		Expect(err).ShouldNot(HaveOccurred())
		hasFinalizer := hasTopoLVMFinalizer(pvc)
		Expect(hasFinalizer).Should(Equal(true), "finalizer should be set for storageclass=%s", topolvmProvisionerImmediateStorageClassName)
	})
})
