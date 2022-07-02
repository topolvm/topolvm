package e2e

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		if !CurrentGinkgoTestDescription().Failed {
			kubectl("delete", "namespaces/"+ns)
		}
	})

	It("should use specified device-class", func() {
		By("deploying Pod with PVC")
		stdout, stderr, err := kubectlWithInput(deviceClassPVCYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput(deviceClassPodYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that the lv was created on specified volume group")
		var volName string
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pvc", "-n", ns, "topo-pvc", "-o=template", "--template={{.spec.volumeName}}")
			if err != nil {
				return fmt.Errorf("failed to get pvc. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			volName = strings.TrimSpace(string(stdout))
			if len(volName) == 0 || volName == "<no value>" {
				return errors.New("failed to get volume name")
			}
			return nil
		}).Should(Succeed())
		var lv *lvinfo
		Eventually(func() error {
			lv, err = getLVInfo(volName)
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
		stdout, stderr, err := kubectlWithInput(noNodesDeviceClassPVCYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput(noNodesDeviceClassPodYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		expectMessage := "no capacity annotation"
		if isStorageCapacity() {
			expectMessage = "node(s) did not have enough free storage."
		}

		By("confirming that the pod wasn't scheduled")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "-n", ns, "pod", "ubuntu", "-o", "json")
			Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

			var pod corev1.Pod
			err = json.Unmarshal(stdout, &pod)
			Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

			for _, c := range pod.Status.Conditions {
				if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse && strings.Contains(c.Message, expectMessage) {
					return nil
				}
			}
			return errors.New("pod doesn't have PodScheduled status")
		}).Should(Succeed())
	})
}

type lvinfo struct {
	lvPath string
	size   string
	vgName string
}

func getLVInfo(volName string) (*lvinfo, error) {
	stdout, stderr, err := kubectl("get", "logicalvolumes.topolvm.io", "-n", "topolvm-system", volName, "-o=template", "--template={{.metadata.uid}}")
	if err != nil {
		return nil, fmt.Errorf("err=%v, stdout=%s, stderr=%s", err, stdout, stderr)
	}
	lvName := strings.TrimSpace(string(stdout))
	stdout, err = exec.Command("sudo", "lvdisplay", "-c", "--select", "lv_name="+lvName).Output()
	if err != nil {
		return nil, fmt.Errorf("err=%v, stdout=%s", err, stdout)
	}
	output := strings.TrimSpace(string(stdout))
	if output == "" {
		return nil, fmt.Errorf("lv_name ( %s ) not found", lvName)
	}
	lines := strings.Split(output, "\n")
	if len(lines) != 1 {
		return nil, errors.New("found multiple lvs")
	}
	items := strings.Split(strings.TrimSpace(lines[0]), ":")
	if len(items) < 4 {
		return nil, fmt.Errorf("invalid format: %s", lines[0])
	}
	return &lvinfo{lvPath: items[0], vgName: items[1], size: items[3]}, nil
}
