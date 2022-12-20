package e2e

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
)

//go:embed testdata/capacity/pvc-pod-template.yaml
var podPVCTemplateYAML string

const nsCapacityTest = "capacity-test"

func testStorageCapacity() {
	var cc CleanupContext
	BeforeEach(func() {
		createNamespace(nsCapacityTest)
		cc = commonBeforeEach()
	})
	AfterEach(func() {
		kubectl("delete", "namespaces/"+nsCapacityTest)
		commonAfterEach(cc)
	})

	It("PVCs and pods are scheduled or are not scheduled by the Storage Capacity Tracking feature", func() {
		if !isStorageCapacity() {
			Skip("skip because current environment is not storage capacity")
			return
		}

		name := "pod-pvc"
		nodeName := "topolvm-e2e-worker"
		if isDaemonsetLvmdEnvSet() {
			nodeName = getDaemonsetLvmdNodeName()
		}

		By("checking the pod having a PVC that is able to schedule is running")
		result, _, err := kubectl("get", "node", nodeName, "-o=json")
		Expect(err).ShouldNot(HaveOccurred())

		var node corev1.Node
		err = json.Unmarshal(result, &node)
		Expect(err).ShouldNot(HaveOccurred())
		size := func() string {
			sizeStr, exists := node.Annotations[topolvm.GetCapacityKeyPrefix()+"00default"]
			Expect(exists).Should(BeTrue(), "size is not found")
			size, err := strconv.Atoi(sizeStr)
			Expect(err).ShouldNot(HaveOccurred())
			s := size >> 30
			return strconv.Itoa(s)
		}()

		lvYaml := buildPodPVCTemplateYAML(name, size, "topolvm-provisioner-default", nodeName)
		_, _, err = kubectlWithInput(lvYaml, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(func() error {
			result, stderr, err := kubectl("-n="+nsCapacityTest, "get", "pvc", name, "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var pvc corev1.PersistentVolumeClaim
			err = json.Unmarshal(result, &pvc)
			if err != nil {
				return err
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return errors.New("pvc status is not bound")
			}

			result, stderr, err = kubectl("-n="+nsCapacityTest, "get", "pods", name, "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var pod corev1.Pod
			err = json.Unmarshal(result, &pod)
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

		By("checking the pod having a PVC that is not able to schedule is not running")
		name2 := name + "2"

		lvYaml2 := buildPodPVCTemplateYAML(name2, size, "topolvm-provisioner-default", nodeName)
		_, _, err = kubectlWithInput(lvYaml2, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(func() error {
			result, stderr, err := kubectl("-n="+nsCapacityTest, "get", "pods", name2, "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var pod corev1.Pod
			err = json.Unmarshal(result, &pod)
			if err != nil {
				return err
			}

			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return errors.New("pod is running")
				}
			}

			podInfo, stderr, err := kubectl("-n="+nsCapacityTest, "describe", "pods", name2)
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}
			if !strings.Contains(string(podInfo), "did not have enough free storage") {
				return errors.New("scheduling failed message is not found")
			}

			return nil
		}).Should(Succeed())

		By("checking the pod having a PVC that is able to schedule because it using an another device class is running")
		name3 := name + "3"

		result, _, err = kubectl("get", "node", nodeName, "-o=json")
		Expect(err).ShouldNot(HaveOccurred())

		err = json.Unmarshal(result, &node)
		Expect(err).ShouldNot(HaveOccurred())
		size = func() string {
			sizeStr, exists := node.Annotations[topolvm.GetCapacityKeyPrefix()+"hdd1"]
			if !exists {
				Expect(errors.New("size is not found")).ShouldNot(HaveOccurred())
			}
			size, err := strconv.Atoi(sizeStr)
			Expect(err).ShouldNot(HaveOccurred())
			s := size >> 30
			return strconv.Itoa(s)
		}()

		lvYaml3 := buildPodPVCTemplateYAML(name3, size, "topolvm-provisioner2", nodeName)
		_, _, err = kubectlWithInput(lvYaml3, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(func() error {
			result, stderr, err := kubectl("-n="+nsCapacityTest, "get", "pvc", name3, "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var pvc corev1.PersistentVolumeClaim
			err = json.Unmarshal(result, &pvc)
			if err != nil {
				return err
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return errors.New("pvc status is not bound")
			}

			result, stderr, err = kubectl("-n="+nsCapacityTest, "get", "pods", name3, "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var pod corev1.Pod
			err = json.Unmarshal(result, &pod)
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

func buildPodPVCTemplateYAML(name, size, sc, node string) []byte {
	return []byte(fmt.Sprintf(podPVCTemplateYAML, name, nsCapacityTest, size, sc, name, nsCapacityTest, name, node))
}
