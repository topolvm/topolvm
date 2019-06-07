package microtest

import (
	"github.com/cybozu-go/topolvm/lvmd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("E2E test", func() {
	testNamespace := "e2e-test"

	BeforeEach(func() {
		kubectl("delete", "namespace", testNamespace)
		stdout, stderr, err := kubectl("create", "namespace", testNamespace)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	})

	AfterEach(func() {
		kubectl("delete", "namespace", testNamespace)
	})

	It("should be mounted in specified path", func() {
		By("initialize a loopback device")
		vgName := "myvg"
		devName, err := lvmd.MakeLoopbackVG(vgName)
		Expect(err).ShouldNot(HaveOccurred())
		defer lvmd.CleanLoopbackVG(devName, vgName)

		By("initialize lvmd with loopback device")

		By("initialize LogicalVolume CRD")

		By("initialize topolvm services")

		By("deploying Pod with PVC")

		By("confirming that the specified device exists in the Pod")
	})
})
