package e2e

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	corev1 "k8s.io/api/core/v1"
)

//go:embed testdata/multiple_vg/pod-pvc-template.yaml
var multipleVGPodPVCTemplateYAML string

func testMultipleVolumeGroups() {
	testNamespacePrefix := "multivgtest-"
	var ns string
	BeforeEach(func() {
		ns = testNamespacePrefix + randomString(10)
		createNamespace(ns)
	})

	AfterEach(func() {
		// When a test fails, I want to investigate the cause. So please don't remove the namespace!
		if !CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
			kubectl("delete", "namespaces/"+ns)
		}
	})

	It("should use the specified device-class", func() {
		By("deploying Pod with PVC")
		nodeName := "topolvm-e2e-worker"
		if isDaemonsetLvmdEnvSet() {
			nodeName = getDaemonsetLvmdNodeName()
		}
		manifest := fmt.Sprintf(multipleVGPodPVCTemplateYAML, "topo-pvc-dc", "topolvm-provisioner2", "pause-dc", "topo-pvc-dc", nodeName)
		_, err := kubectlWithInput([]byte(manifest), "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming that the lv was created on specified volume group")
		var volName string
		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err := getObjects(&pvc, "pvc", "-n", ns, "topo-pvc-dc")
			if err != nil {
				return fmt.Errorf("failed to get pvc. err: %w", err)
			}
			volName = pvc.Spec.VolumeName
			if len(volName) == 0 {
				return errors.New("failed to get volume name")
			}
			return nil
		}).Should(Succeed())
		var logicalvolume topolvmv1.LogicalVolume
		err = getObjects(&logicalvolume, "logicalvolumes", volName)
		Expect(err).ShouldNot(HaveOccurred())
		var lv *lvinfo
		Eventually(func() error {
			lv, err = getLVInfo(string(logicalvolume.UID))
			return err
		}).Should(Succeed())

		vgName := "node1-thick2"
		Expect(vgName).Should(Equal(lv.vgName))
	})

	It("should not schedule a pod because there are no nodes that have specified device-classes", func() {
		By("deploying Pod with PVC")
		nodeName := "topolvm-e2e-worker"
		if isDaemonsetLvmdEnvSet() {
			nodeName = getDaemonsetLvmdNodeName()
		}
		manifest := fmt.Sprintf(multipleVGPodPVCTemplateYAML, "topo-pvc-not-found-dc", "topolvm-provisioner-not-found-device", "pause-not-found-dc", "topo-pvc-not-found-dc", nodeName)
		_, err := kubectlWithInput([]byte(manifest), "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		expectMessage := "no capacity annotation"
		if isStorageCapacity() {
			expectMessage = "node(s) did not have enough free storage."
		}

		By("confirming that the pod wasn't scheduled")
		Eventually(func() error {
			var pod corev1.Pod
			err := getObjects(&pod, "pod", "-n", ns, "pause-not-found-dc")
			if err != nil {
				return err
			}

			for _, c := range pod.Status.Conditions {
				if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse && strings.Contains(c.Message, expectMessage) {
					return nil
				}
			}
			return errors.New("pod doesn't have PodScheduled status")
		}).Should(Succeed())
	})

	It("should run a pod using the default device-class", func() {
		By("deploying Pod with PVC")
		nodeName := "topolvm-e2e-worker"
		if isDaemonsetLvmdEnvSet() {
			nodeName = getDaemonsetLvmdNodeName()
		}
		manifest := fmt.Sprintf(multipleVGPodPVCTemplateYAML, "topo-pvc-default", "topolvm-provisioner-default", "pause-default", "topo-pvc-default", nodeName)
		_, err := kubectlWithInput([]byte(manifest), "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming the pod running")
		Eventually(func() error {
			var pod corev1.Pod
			err := getObjects(&pod, "pod", "-n", ns, "pause-default")
			if err != nil {
				return err
			}

			if pod.Status.Phase != corev1.PodRunning {
				return errors.New("pod is not running")
			}
			return nil
		}).Should(Succeed())
	})
}
