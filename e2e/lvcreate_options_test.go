package e2e

import (
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func testLVCreateOptions() {
	testNamespacePrefix := "lvcreate-options-"
	var ns string
	var cc CleanupContext

	BeforeEach(func() {
		cc = commonBeforeEach()

		ns = testNamespacePrefix + randomString(10)
		createNamespace(ns)
	})

	AfterEach(func() {
		// When a test fails, I want to investigate the cause. So please don't remove the namespace!
		if !CurrentGinkgoTestDescription().Failed {
			kubectl("delete", "namespaces/"+ns)
		}

		commonAfterEach(cc)
	})

	It("should use lvcreate-options when creating LV", func() {
		By("creating Pod with PVC using raid device-class")
		claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 1, "topolvm-provisioner-raid"))
		podYaml := []byte(fmt.Sprintf(podVolumeMountTemplateYAML, "ubuntu", "topo-pvc"))

		stdout, stderr, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("finding the corresponding LV")

		var lvName string
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pvc", "-n", ns, "topo-pvc", "-o=template", "--template={{.spec.volumeName}}")
			if err != nil {
				return fmt.Errorf("failed to get PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			volumeName := strings.TrimSpace(string(stdout))

			stdout, stderr, err = kubectl("get", "logicalvolumes", "-n", "topolvm-system", volumeName, "-o=template", "--template={{.metadata.uid}}")
			if err != nil {
				return fmt.Errorf("failed to get logicalvolume. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			lvName = strings.TrimSpace(string(stdout))
			return nil
		}).Should(Succeed())

		By("checking that the LV is of type raid")
		stdout, err = exec.Command("sudo", "lvs", "-o", "lv_attr", "--noheadings", "--select", "lv_name="+lvName).Output()
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		attribute_bit1 := string(strings.TrimSpace(string(stdout))[0])
		// lv_attr bit 1 represents the volume type, where 'r' is for raid
		Expect(attribute_bit1).To(Equal("r"))
	})
}
