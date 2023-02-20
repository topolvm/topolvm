package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func testController() {
	var cc CleanupContext
	BeforeEach(func() {
		cc = commonBeforeEach()
	})
	AfterEach(func() {
		commonAfterEach(cc)
	})

	It("should expose Prometheus metrics", func() {
		// For CSI sidecar metrics access checking
		for _, port := range []string{"9808", "9809", "9810", "9811"} {
			stdout, stderr, err := kubectl("exec", "-n", "topolvm-system", "deploy/topolvm-controller", "-c=topolvm-controller", "--", "curl", "http://localhost:"+port+"/metrics")
			Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		}
	})
}
