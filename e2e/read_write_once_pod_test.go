package e2e

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
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
		kubectl("delete", "namespaces/"+ns)
		commonAfterEach(cc)
	})

	It("should not schedule pods if the pods will use the PVC that is already used from another pod and an access mode of the PVC is ReadWriteOncePod", func() {
		if !isReadWriteOncePod() {
			Skip("This test run when only enable the ReadWriteOncePod feature gate")
			return
		}

		podName := "testpod-1"
		pvcName := "testpod"
		size := "5Gi"

		By("Create a pod and a PVC with a ReadWriteOncePod access mode")
		pvcYaml := buildPVCTemplateYAML(pvcName, ns, size)
		stdout, stderr, err := kubectlWithInput(pvcYaml, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		podYaml := buildPodTemplateYAML(podName, ns, pvcName)
		stdout, stderr, err = kubectlWithInput(podYaml, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("Checking the pod is running")
		Eventually(func() error {
			result, stderr, err := kubectl("-n="+ns, "get", "pvc", pvcName, "-o=json")
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

			result, stderr, err = kubectl("-n="+ns, "get", "pods", podName, "-o=json")
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

		By("Create a pod with a ReadWriteOncePod access mode and the already used PVC")
		podName = "testpod-2"
		podYaml = buildPodTemplateYAML(podName, ns, pvcName)
		stdout, stderr, err = kubectlWithInput(podYaml, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("Checking the pod is not running")
		Eventually(func() error {
			result, stderr, err := kubectl("-n="+ns, "get", "pods", podName, "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var pod corev1.Pod
			err = json.Unmarshal(result, &pod)
			if err != nil {
				return err
			}

			for _, c := range pod.Status.Conditions {
				// https://github.com/kubernetes/kubernetes/blob/v1.22.0/pkg/scheduler/framework/plugins/volumerestrictions/volume_restrictions.go#L53-L54
				if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse && strings.Contains(c.Message, "node has pod using PersistentVolumeClaim with the same name and ReadWriteOncePod access mode") {
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
