package e2e

import (
	_ "embed"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var (
	//go:embed testdata/cloning/clone-pod-template.yaml
	thinPodCloneTemplateYAML string

	//go:embed testdata/cloning/clone-pvc-template.yaml
	thinPvcCloneTemplateYAML string
)

const (
	thinClonePVCName = "thin-clone"
	thinClonePodName = "thin-clone-pod"
)

func testPVCClone() {
	const writePath = "/test1/bootstrap.log"

	var nsCloneTest string

	BeforeEach(func() {
		nsCloneTest = "clone-test-" + randomString()
		createNamespace(nsCloneTest)
	})
	AfterEach(func() {
		if !CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
			_, err := kubectl("delete", "namespaces", nsCloneTest)
			Expect(err).ShouldNot(HaveOccurred())
		}
	})

	It("should create a PVC Clone", func() {
		By("deploying Pod with PVC")

		thinPvcYAML := []byte(fmt.Sprintf(thinPVCTemplateYAML, volName, pvcSizeBytes))
		_, err := kubectlWithInput(thinPvcYAML, "apply", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		thinPodYAML := []byte(fmt.Sprintf(thinPodTemplateYAML, "thinpod", volName))
		_, err = kubectlWithInput(thinPodYAML, "apply", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		By("confirming if the source PVC is created")
		var pvc corev1.PersistentVolumeClaim
		Eventually(func() error {
			err = getObjects(&pvc, "pvc", "-n", nsCloneTest, volName)
			if err != nil {
				return fmt.Errorf("failed to get PVC. err: %w", err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", volName)
			}
			return nil
		}).Should(Succeed())

		By("writing a file on mountpath")
		Eventually(func() error {
			_, err := kubectl("exec", "-n", nsCloneTest, "thinpod", "--", "cp", "/var/log/bootstrap.log", writePath)
			return err
		}).Should(Succeed())

		_, err = kubectl("exec", "-n", nsCloneTest, "thinpod", "--", "sync")
		Expect(err).ShouldNot(HaveOccurred())
		stdout, err := kubectl("exec", "-n", nsCloneTest, "thinpod", "--", "cat", writePath)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(strings.TrimSpace(string(stdout))).ShouldNot(BeEmpty())

		pvcStatusSize := pvc.Status.Capacity.Storage().Value()
		thinPVCCloneYAML := []byte(fmt.Sprintf(thinPvcCloneTemplateYAML, thinClonePVCName, volName, pvcStatusSize))
		_, err = kubectlWithInput(thinPVCCloneYAML, "apply", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		thinPodCloneYAML := []byte(fmt.Sprintf(thinPodCloneTemplateYAML, thinClonePodName, thinClonePVCName))
		_, err = kubectlWithInput(thinPodCloneYAML, "apply", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming that the lv for cloned volume was created in the thin volume group and pool")
		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err := getObjects(&pvc, "pvc", "-n", nsCloneTest, thinClonePVCName)
			if err != nil {
				return fmt.Errorf("failed to get PVC. err: %w", err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", thinClonePVCName)
			}
			return nil
		}).Should(Succeed())
		var lvName string
		Eventually(func() error {
			lvName, err = getLVNameOfPVC(thinClonePVCName, nsCloneTest)
			return err
		}).Should(Succeed())

		var lv *lvinfo
		Eventually(func() error {
			lv, err = getLVInfo(lvName)
			return err
		}).Should(Succeed())

		Expect(lv.vgName).Should(Equal("node1-thin1"))
		Expect(lv.poolName).Should(Equal("pool0"))

		By("confirming that the file exists in the cloned volume")
		Eventually(func() error {
			stdout, err := kubectl("exec", "-n", nsCloneTest, thinClonePodName, "--", "cat", writePath)
			if err != nil {
				return fmt.Errorf("failed to cat. err: %w", err)
			}
			if len(strings.TrimSpace(string(stdout))) == 0 {
				return fmt.Errorf("%s is empty", writePath)
			}
			return nil
		}).Should(Succeed())

	})

	It("validate if the cloned PVC is standalone", func() {
		By("creating a PVC")
		thinPvcYAML := []byte(fmt.Sprintf(thinPVCTemplateYAML, volName, pvcSizeBytes))
		_, err := kubectlWithInput(thinPvcYAML, "apply", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		thinPodYAML := []byte(fmt.Sprintf(thinPodTemplateYAML, "thinpod", volName))
		_, err = kubectlWithInput(thinPodYAML, "apply", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		var pvc corev1.PersistentVolumeClaim
		Eventually(func() error {
			err := getObjects(&pvc, "pvc", "-n", nsCloneTest, volName)
			if err != nil {
				return fmt.Errorf("failed to get PVC. err: %w", err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", volName)
			}
			return nil
		}).Should(Succeed())

		By("creating clone of the PVC")
		pvcStatusSize := pvc.Status.Capacity.Storage().Value()
		thinPVCCloneYAML := []byte(fmt.Sprintf(thinPvcCloneTemplateYAML, thinClonePVCName, volName, pvcStatusSize))
		_, err = kubectlWithInput(thinPVCCloneYAML, "apply", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		thinPodCloneYAML := []byte(fmt.Sprintf(thinPodCloneTemplateYAML, thinClonePodName, thinClonePVCName))
		_, err = kubectlWithInput(thinPodCloneYAML, "apply", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("validating that the cloned volume is present")
		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err := getObjects(&pvc, "pvc", "-n", nsCloneTest, thinClonePVCName)
			if err != nil {
				return fmt.Errorf("failed to get PVC. err: %w", err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", thinClonePVCName)
			}
			return nil
		}).Should(Succeed())

		var lvName string
		Eventually(func() error {
			lvName, err = getLVNameOfPVC(thinClonePVCName, nsCloneTest)
			return err
		}).Should(Succeed())

		var lv *lvinfo
		Eventually(func() error {
			lv, err = getLVInfo(lvName)
			return err
		}).Should(Succeed())

		Expect(lv.vgName).Should(Equal("node1-thin1"))
		Expect(lv.poolName).Should(Equal("pool0"))

		By("deleting the source volume and application")
		_, err = kubectlWithInput(thinPodYAML, "delete", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		_, err = kubectlWithInput(thinPvcYAML, "delete", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("validating if the cloned volume is present and is not deleted")
		lvName, err = getLVNameOfPVC(thinClonePVCName, nsCloneTest)
		Expect(err).Should(Succeed())

		_, err = getLVInfo(lvName)
		Expect(err).Should(Succeed())
	})

}
