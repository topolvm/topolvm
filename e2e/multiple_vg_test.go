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

//go:embed testdata/multiple_vg/device-class-pod.yaml
var deviceClassPodYAML []byte

//go:embed testdata/multiple_vg/device-class-pvc.yaml
var deviceClassPVCYAML []byte

//go:embed testdata/multiple_vg/no-nodes-device-class-pod.yaml
var noNodesDeviceClassPodYAML []byte

//go:embed testdata/multiple_vg/no-nodes-device-class-pvc.yaml
var noNodesDeviceClassPVCYAML []byte

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

	It("should use specified device-class", func() {
		By("deploying Pod with PVC")
		_, err := kubectlWithInput(deviceClassPVCYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectlWithInput(deviceClassPodYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming that the lv was created on specified volume group")
		var volName string
		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err := getObjects(&pvc, "pvc", "-n", ns, "topo-pvc")
			if err != nil {
				return fmt.Errorf("failed to get pvc. err: %v", err)
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

		vgName := "node3-myvg2"
		if isDaemonsetLvmdEnvSet() {
			vgName = "node-myvg3"
		}
		Expect(vgName).Should(Equal(lv.vgName))
	})

	It("should not schedule pod because there are no nodes that have specified device-classes", func() {
		By("deploying Pod with PVC")
		_, err := kubectlWithInput(noNodesDeviceClassPVCYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectlWithInput(noNodesDeviceClassPodYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		expectMessage := "no capacity annotation"
		if isStorageCapacity() {
			expectMessage = "node(s) did not have enough free storage."
		}

		By("confirming that the pod wasn't scheduled")
		Eventually(func() error {
			var pod corev1.Pod
			err := getObjects(&pod, "pod", "-n", ns, "pause")
			Expect(err).ShouldNot(HaveOccurred())

			for _, c := range pod.Status.Conditions {
				if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse && strings.Contains(c.Message, expectMessage) {
					return nil
				}
			}
			return errors.New("pod doesn't have PodScheduled status")
		}).Should(Succeed())
	})
}
