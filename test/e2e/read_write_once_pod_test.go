package e2e

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

//go:embed testdata/read_write_once_pod/pvc-for-read-write-once-pod-template.yaml
var pvcForReadWriteOncePodTemplateYAML string

//go:embed testdata/read_write_once_pod/pod-for-read-write-once-pod-template.yaml
var podForReadWriteOncePodTemplateYAML string

func testReadWriteOncePod() {
	ns := "read-write-once-pod-test"
	var cc CleanupContext

	BeforeEach(func() {
		cc = commonBeforeEach()
		createNamespace(ns)
	})

	AfterEach(func() {
		_, err := kubectl("delete", "namespaces/"+ns)
		Expect(err).ShouldNot(HaveOccurred())
		commonAfterEach(cc)
	})

	It("should not schedule pods if the pods will use the PVC that is already used from another pod", func() {
		podName := "testpod-1"
		pvcName := "testpod"
		size := "5Gi"

		By("Create a pod and a PVC with a ReadWriteOncePod access mode")
		pvcYaml := buildPVCTemplateYAML(pvcName, ns, size)
		_, err := kubectlWithInput(pvcYaml, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		podYaml := buildPodTemplateYAML(podName, ns, pvcName)
		_, err = kubectlWithInput(podYaml, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("Checking the pod is running")
		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err := getObjects(&pvc, "pvc", "-n", ns, pvcName)
			if err != nil {
				return err
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return errors.New("pvc status is not bound")
			}

			var pod corev1.Pod
			err = getObjects(&pod, "pods", "-n", ns, podName)
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

		By("Create a pod with a ReadWriteOncePod access mode and the already used PVC")
		podName = "testpod-2"
		podYaml = buildPodTemplateYAML(podName, ns, pvcName)
		_, err = kubectlWithInput(podYaml, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("Checking the pod is not running")
		Eventually(func() error {
			var pod corev1.Pod
			err := getObjects(&pod, "pods", "-n", ns, podName)
			if err != nil {
				return err
			}

			for _, c := range pod.Status.Conditions {
				//nolint:lll
				// https://github.com/kubernetes/kubernetes/blob/v1.22.0/pkg/scheduler/framework/plugins/volumerestrictions/volume_restrictions.go#L53-L54

				if c.Type == corev1.PodScheduled &&
					c.Status == corev1.ConditionFalse &&
					strings.Contains(c.Message,
						"node has pod using PersistentVolumeClaim with the same name and ReadWriteOncePod access mode") {
					return nil
				}
			}

			return errors.New("pod is running")
		}).Should(Succeed())
	})
}

func buildPVCTemplateYAML(name, ns, size string) []byte {
	return []byte(fmt.Sprintf(pvcForReadWriteOncePodTemplateYAML, name, ns, size))
}

func buildPodTemplateYAML(name, ns, claimName string) []byte {
	return []byte(fmt.Sprintf(podForReadWriteOncePodTemplateYAML, name, ns, claimName))
}
