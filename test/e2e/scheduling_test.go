package e2e

import (
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
)

//go:embed testdata/scheduling/pvc-pod-template.yaml
var podPVCTemplateYAML string

const nsSchedulingTest = "scheduling-test"

func testScheduling() {
	var cc CleanupContext
	BeforeEach(func() {
		createNamespace(nsSchedulingTest)
		cc = commonBeforeEach()
	})
	AfterEach(func() {
		_, err := kubectl("delete", "namespaces/"+nsSchedulingTest)
		Expect(err).ShouldNot(HaveOccurred())
		commonAfterEach(cc)
	})

	It("should schedule a pod with a PVC if a node has enough capacity", func() {
		name := "pod-pvc"
		nodeName := "topolvm-e2e-worker"

		By("checking the pod with a PVC is running if a node capacity is sufficient")
		var node corev1.Node
		err := getObjects(&node, "node", nodeName)
		Expect(err).ShouldNot(HaveOccurred())
		size := func() string {
			sizeStr, exists := node.Annotations[topolvm.GetCapacityKeyPrefix()+"00default"]
			Expect(exists).Should(BeTrue(), "size is not found")
			size, err := strconv.Atoi(sizeStr)
			Expect(err).ShouldNot(HaveOccurred())
			s := size >> 30
			return strconv.Itoa(s)
		}()

		lvYaml := buildPodPVCTemplateYAML(name, size, "topolvm-provisioner")
		_, err = kubectlWithInput(lvYaml, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(func() error {
			var pod corev1.Pod
			err = getObjects(&pod, "pods", "-n", nsSchedulingTest, name)
			if err != nil {
				return err
			}

			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return nil
				}
			}

			return errors.New("pod is not running")
		}).Should(Succeed())

		By("checking the pod with a PVC is not scheduled if a node capacity is not enough")
		name2 := name + "2"

		lvYaml2 := buildPodPVCTemplateYAML(name2, size, "topolvm-provisioner")
		_, err = kubectlWithInput(lvYaml2, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(func() error {
			var pod corev1.Pod
			err := getObjects(&pod, "pods", "-n", nsSchedulingTest, name2)
			if err != nil {
				return err
			}

			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return errors.New("pod is running")
				}
			}

			expectMessage := "out of VG free space"
			if isStorageCapacity() {
				expectMessage = "did not have enough free storage"
			}

			for _, c := range pod.Status.Conditions {
				if c.Type == corev1.PodScheduled &&
					c.Status == corev1.ConditionFalse &&
					strings.Contains(c.Message, expectMessage) {
					return nil
				}
			}

			return errors.New("scheduling failed message is not found")
		}).Should(Succeed())

		By("checking the pod with a PVC using another device-class is running if the device-class has enough capacity")
		name3 := name + "3"

		err = getObjects(&node, "node", nodeName)
		Expect(err).ShouldNot(HaveOccurred())
		size = func() string {
			sizeStr, exists := node.Annotations[topolvm.GetCapacityKeyPrefix()+"dc2"]
			if !exists {
				Expect(errors.New("size is not found")).ShouldNot(HaveOccurred())
			}
			size, err := strconv.Atoi(sizeStr)
			Expect(err).ShouldNot(HaveOccurred())
			s := size >> 30
			return strconv.Itoa(s)
		}()

		lvYaml3 := buildPodPVCTemplateYAML(name3, size, "topolvm-provisioner2")
		_, err = kubectlWithInput(lvYaml3, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(func() error {
			var pod corev1.Pod
			err = getObjects(&pod, "pods", "-n", nsSchedulingTest, name3)
			if err != nil {
				return err
			}

			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return nil
				}
			}

			return errors.New("pod is not running")
		}).Should(Succeed())
	})
}

func buildPodPVCTemplateYAML(name, size, sc string) []byte {
	return []byte(fmt.Sprintf(podPVCTemplateYAML, name, nsSchedulingTest, size, sc, name, nsSchedulingTest, name))
}
