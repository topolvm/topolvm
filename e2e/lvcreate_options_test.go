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

		ns = testNamespacePrefix + randomString(10)
		createNamespace(ns)
	})

	AfterEach(func() {
		// When a test fails, I want to investigate the cause. So please don't remove the namespace!
		if !CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
			kubectl("delete", "namespaces/"+ns)
		}

		commonAfterEach(cc)
	})

	It("should use lvcreate-options when creating LV", func() {
		By("creating Pod with PVC using raid device-class")
		claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 1, "topolvm-provisioner-raid"))
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
				return fmt.Errorf("failed to get PVC. err: %v", err)
			}

			var lv topolvmv1.LogicalVolume
			err = getObjects(&lv, "logicalvolumes", pvc.Spec.VolumeName)
			if err != nil {
				return fmt.Errorf("failed to get logicalvolume. err: %v", err)
			}
			lvName = string(lv.UID)
			return nil
		}).Should(Succeed())

		By("checking that the LV is of type raid")
		stdout, err := execAtLocal("sudo", nil, "lvs", "-o", "lv_attr", "--noheadings", "--select", "lv_name="+lvName)
		Expect(err).ShouldNot(HaveOccurred())
		attribute_bit1 := string(strings.TrimSpace(string(stdout))[0])
		// lv_attr bit 1 represents the volume type, where 'r' is for raid
		Expect(attribute_bit1).To(Equal("r"))
	})

	It("should use lvcreate-option-classes when creating LV", func() {
		By("creating Pod with PVC using raid1 device-class")
		claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc-raid1", "Filesystem", 1, "topolvm-provisioner-raid1"))
		podYaml := []byte(fmt.Sprintf(podVolumeMountTemplateYAML, "ubuntu2", "topo-pvc-raid1"))

		_, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("finding the corresponding LV")

		var lvName string
		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err := getObjects(&pvc, "pvc", "-n", ns, "topo-pvc-raid1")
			if err != nil {
				return fmt.Errorf("failed to get PVC. err: %v", err)
			}

			var lv topolvmv1.LogicalVolume
			err = getObjects(&lv, "logicalvolumes", pvc.Spec.VolumeName)
			if err != nil {
				return fmt.Errorf("failed to get logicalvolume. err: %v", err)
			}
			lvName = string(lv.UID)
			return nil
		}).Should(Succeed())

		By("checking that the LV is of type raid")
		stdout, err := execAtLocal("sudo", nil, "lvs", "-o", "lv_attr", "--noheadings", "--select", "lv_name="+lvName)
		Expect(err).ShouldNot(HaveOccurred())
		attribute_bit1 := string(strings.TrimSpace(string(stdout))[0])
		// lv_attr bit 1 represents the volume type, where 'r' is for raid
		Expect(attribute_bit1).To(Equal("r"))
	})
}
