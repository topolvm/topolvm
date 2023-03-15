package e2e

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
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
	var nsCloneTest string
	BeforeEach(func() {
		nsCloneTest = "clone-test-" + randomString(10)
		createNamespace(nsCloneTest)
	})
	AfterEach(func() {
		if !CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
			kubectl("delete", "namespaces/"+nsCloneTest)
		}
	})

	It("should create a PVC Clone", func() {
		By("deploying Pod with PVC")

		nodeName := "topolvm-e2e-worker"
		if isDaemonsetLvmdEnvSet() {
			nodeName = getDaemonsetLvmdNodeName()
		}
		var volumeName string
		thinPvcYAML := []byte(fmt.Sprintf(thinPVCTemplateYAML, volName, pvcSize))
		stdout, stderr, err := kubectlWithInput(thinPvcYAML, "apply", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		thinPodYAML := []byte(fmt.Sprintf(thinPodTemplateYAML, "thinpod", volName, topolvm.GetTopologyNodeKey(), nodeName))
		stdout, stderr, err = kubectlWithInput(thinPodYAML, "apply", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		By("confirming if the source PVC is created")
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "-n", nsCloneTest, "pvc", volName, "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var pvc corev1.PersistentVolumeClaim
			err = json.Unmarshal(stdout, &pvc)
			if err != nil {
				return fmt.Errorf("failed to unmarshal PVC. stdout: %s, err: %v", stdout, err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", volName)
			}
			return nil
		}).Should(Succeed())

		By("writing a file on mountpath")
		writePath := "/test1/bootstrap.log"
		Eventually(func() error {
			stdout, stderr, err = kubectl("exec", "-n", nsCloneTest, "thinpod", "--", "cp", "/var/log/bootstrap.log", writePath)
			return err
		}).Should(Succeed())

		stdout, stderr, err = kubectl("exec", "-n", nsCloneTest, "thinpod", "--", "sync")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectl("exec", "-n", nsCloneTest, "thinpod", "--", "cat", writePath)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).ShouldNot(BeEmpty())

		thinPVCCloneYAML := []byte(fmt.Sprintf(thinPvcCloneTemplateYAML, thinClonePVCName, volName, pvcSize))
		stdout, stderr, err = kubectlWithInput(thinPVCCloneYAML, "apply", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		thinPodCloneYAML := []byte(fmt.Sprintf(thinPodCloneTemplateYAML, thinClonePodName, thinClonePVCName, topolvm.GetTopologyNodeKey(), nodeName))
		stdout, stderr, err = kubectlWithInput(thinPodCloneYAML, "apply", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that the lv for cloned volume was created in the thin volume group and pool")
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "-n", nsCloneTest, "pvc", thinClonePVCName, "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var pvc corev1.PersistentVolumeClaim
			err = json.Unmarshal(stdout, &pvc)
			if err != nil {
				return fmt.Errorf("failed to unmarshal PVC. stdout: %s, err: %v", stdout, err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", thinClonePVCName)
			}
			return nil
		}).Should(Succeed())
		Eventually(func() error {
			volumeName, err = getVolumeNameofPVC(thinClonePVCName, nsCloneTest)
			return err
		}).Should(Succeed())

		var lv *thinlvinfo
		Eventually(func() error {
			lv, err = getThinLVInfo(volumeName)
			return err
		}).Should(Succeed())

		vgName := "node1-myvg4"
		if isDaemonsetLvmdEnvSet() {
			vgName = "node-myvg5"
		}
		Expect(vgName).Should(Equal(lv.vgName))

		poolName := "pool0"
		Expect(poolName).Should(Equal(lv.poolName))

		By("confirming that the file exists in the cloned volume")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pvc", thinClonePVCName, "-n", nsCloneTest)
			if err != nil {
				return fmt.Errorf("failed to get PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			stdout, stderr, err = kubectl("get", "pods", thinClonePodName, "-n", nsCloneTest)
			if err != nil {
				return fmt.Errorf("failed to get Pod. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			stdout, stderr, err = kubectl("exec", "-n", nsCloneTest, thinClonePodName, "--", "cat", writePath)
			if err != nil {
				return fmt.Errorf("failed to cat. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			if len(strings.TrimSpace(string(stdout))) == 0 {
				return fmt.Errorf(writePath + " is empty")
			}
			return nil
		}).Should(Succeed())

	})

	It("validate if the cloned PVC is standalone", func() {

		nodeName := "topolvm-e2e-worker"
		if isDaemonsetLvmdEnvSet() {
			nodeName = getDaemonsetLvmdNodeName()
		}
		By("creating a PVC")
		var volumeName string
		thinPvcYAML := []byte(fmt.Sprintf(thinPVCTemplateYAML, volName, pvcSize))
		stdout, stderr, err := kubectlWithInput(thinPvcYAML, "apply", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		thinPodYAML := []byte(fmt.Sprintf(thinPodTemplateYAML, "thinpod", volName, topolvm.GetTopologyNodeKey(), nodeName))
		stdout, stderr, err = kubectlWithInput(thinPodYAML, "apply", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "-n", nsCloneTest, "pvc", volName, "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var pvc corev1.PersistentVolumeClaim
			err = json.Unmarshal(stdout, &pvc)
			if err != nil {
				return fmt.Errorf("failed to unmarshal PVC. stdout: %s, err: %v", stdout, err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", volName)
			}
			return nil
		}).Should(Succeed())

		By("creating clone of the PVC")
		thinPVCCloneYAML := []byte(fmt.Sprintf(thinPvcCloneTemplateYAML, thinClonePVCName, volName, pvcSize))
		stdout, stderr, err = kubectlWithInput(thinPVCCloneYAML, "apply", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		thinPodCloneYAML := []byte(fmt.Sprintf(thinPodCloneTemplateYAML, thinClonePodName, thinClonePVCName, topolvm.GetTopologyNodeKey(), nodeName))
		stdout, stderr, err = kubectlWithInput(thinPodCloneYAML, "apply", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("validating that the cloned volume is present")
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "-n", nsCloneTest, "pvc", thinClonePVCName, "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var pvc corev1.PersistentVolumeClaim
			err = json.Unmarshal(stdout, &pvc)
			if err != nil {
				return fmt.Errorf("failed to unmarshal PVC. stdout: %s, err: %v", stdout, err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", thinClonePVCName)
			}
			return nil
		}).Should(Succeed())

		Eventually(func() error {
			volumeName, err = getVolumeNameofPVC(thinClonePVCName, nsCloneTest)
			return err
		}).Should(Succeed())

		var lv *thinlvinfo
		Eventually(func() error {
			lv, err = getThinLVInfo(volumeName)
			return err
		}).Should(Succeed())

		vgName := "node1-myvg4"
		if isDaemonsetLvmdEnvSet() {
			vgName = "node-myvg5"
		}
		Expect(vgName).Should(Equal(lv.vgName))

		poolName := "pool0"
		Expect(poolName).Should(Equal(lv.poolName))

		By("deleting the source volume and application")
		stdout, stderr, err = kubectlWithInput(thinPodYAML, "delete", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		stdout, stderr, err = kubectlWithInput(thinPvcYAML, "delete", "-n", nsCloneTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("validating if the cloned volume is present and is not deleted")
		volumeName, err = getVolumeNameofPVC(thinClonePVCName, nsCloneTest)
		Expect(err).Should(Succeed())

		_, err = getThinLVInfo(volumeName)
		Expect(err).Should(Succeed())
	})

}
