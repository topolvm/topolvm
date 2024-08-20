package e2e

import (
	_ "embed"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var (
	//go:embed testdata/empty_storageclass/manifest.yaml
	emptyStorageClassManifest string
)

const (
	nsEmptyStorageClassTest = "empty-storageclass-test"
)

func testEmptyStorageClass() {
	var cc CleanupContext
	BeforeEach(func() {
		createNamespace(nsEmptyStorageClassTest)
		cc = commonBeforeEach()
	})
	AfterEach(func() {
		_, err := kubectl("delete", "namespaces", nsEmptyStorageClassTest)
		Expect(err).ShouldNot(HaveOccurred())
		commonAfterEach(cc)
	})

	// This test confirms that each webhook of TopoLVM ignores PVCs when the PVC's storageClassName is empty ("")
	// and confirm that such a Pod is possible to be deployed.
	// There are no corresponding PVs in the deployed PVC and the Pod does not work, but that is as intended.
	It("should create a Pod with empty StorageClass PVC", func() {
		By("applying a Pod with empty StorageClass PVC")
		yaml := []byte(fmt.Sprintf(emptyStorageClassManifest,
			nsEmptyStorageClassTest, nsEmptyStorageClassTest))
		_, err := kubectlWithInput(yaml, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		var pod corev1.Pod
		err = getObjects(&pod, "pod", "-n", nsEmptyStorageClassTest, "test-empty-storageclass")
		Expect(err).ShouldNot(HaveOccurred())
	})
}
