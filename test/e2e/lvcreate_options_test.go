package e2e

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	corev1 "k8s.io/api/core/v1"
)

func testLVCreateOptions() {
	testNamespacePrefix := "lvcreate-options-"
	var ns string
	var cc CleanupContext

	BeforeEach(func() {
		cc = commonBeforeEach()

		ns = testNamespacePrefix + randomString()
		createNamespace(ns)
	})

	AfterEach(func() {
		// When a test fails, I want to investigate the cause. So please don't remove the namespace!
		if !CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
			_, err := kubectl("delete", "namespaces", ns)
			Expect(err).ShouldNot(HaveOccurred())
		}

		commonAfterEach(cc)
	})

	DescribeTable(
		"LVM option should be used",
		func(deviceClass string, storageClassName string) {
			By(fmt.Sprintf("creating Pod with PVC using %s device-class", deviceClass))
			claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 1024, storageClassName))
			podYaml := []byte(fmt.Sprintf(podVolumeMountTemplateYAML, "ubuntu", "topo-pvc"))

			_, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())
			_, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			By("finding the corresponding LV")

			var lvName string
			Eventually(func() error {
				var pvc corev1.PersistentVolumeClaim
				err := getObjects(&pvc, "pvc", "-n", ns, "topo-pvc")
				if err != nil {
					return fmt.Errorf("failed to get PVC. err: %w", err)
				}

				var lv topolvmv1.LogicalVolume
				err = getObjects(&lv, "logicalvolumes", pvc.Spec.VolumeName)
				if err != nil {
					return fmt.Errorf("failed to get logicalvolume. err: %w", err)
				}
				lvName = string(lv.UID)
				return nil
			}).Should(Succeed())

			By("checking that the LV is of type raid")
			stdout, err := execAtLocal("sudo", nil,
				"lvs", "-o", "lv_attr", "--noheadings", "--select", fmt.Sprintf("lv_name=%s", lvName))
			Expect(err).ShouldNot(HaveOccurred())
			attribute_bit1 := string(strings.TrimSpace(string(stdout))[0])
			// lv_attr bit 1 represents the volume type, where 'r' is for raid
			Expect(attribute_bit1).To(Equal("r"))
		},
		Entry("when using lvcreate-options", "create-option-raid1", "topolvm-provisioner-create-option-raid1"),
		Entry("when using lvcreateOptionClasses", "option-class-raid1", "topolvm-provisioner-option-class-raid1"),
	)
}
