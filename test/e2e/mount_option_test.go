package e2e

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

//go:embed testdata/mount_option/pod-with-mount-option-pvc.yaml
var podWithMountOptionPVCYAML []byte

func testMountOption() {
	var cc CleanupContext

	BeforeEach(func() {
		cc = commonBeforeEach()
	})

	AfterEach(func() {
		commonAfterEach(cc)
	})

	It("should publish filesystem with mount option", func() {
		By("creating a PVC and Pod")
		_, err := kubectlWithInput(podWithMountOptionPVCYAML, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			var pod corev1.Pod
			err := getObjects(&pod, "pod", "pause-mount-option")
			if err != nil {
				return fmt.Errorf("failed to get pod. err: %w", err)
			}

			if pod.Status.Phase != corev1.PodRunning {
				return errors.New("Pod is not running")
			}

			return nil
		}).Should(Succeed())

		By("check mount option")
		var pvc corev1.PersistentVolumeClaim
		err = getObjects(&pvc, "pvc", "topo-pvc-mount-option")
		Expect(err).ShouldNot(HaveOccurred())

		f, err := os.Open("/proc/mounts")
		Expect(err).ShouldNot(HaveOccurred())
		defer func() { _ = f.Close() }()
		mounts, err := io.ReadAll(f)
		Expect(err).ShouldNot(HaveOccurred())

		var isExistingOption bool
		lines := strings.Split(string(mounts), "\n")
		for _, line := range lines {
			if strings.Contains(line, pvc.Spec.VolumeName) {
				fields := strings.Split(line, " ")
				Expect(len(fields)).To(Equal(6))
				options := strings.Split(fields[3], ",")
				for _, option := range options {
					if option == "debug" {
						isExistingOption = true
					}
				}
			}
		}
		Expect(isExistingOption).Should(BeTrue())

		By("cleaning pvc/pod")
		_, err = kubectlWithInput(podWithMountOptionPVCYAML, "delete", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
	})
}
