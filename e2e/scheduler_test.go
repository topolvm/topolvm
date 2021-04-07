package e2e

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

//go:embed testdata/scheduler/capacity-pod-template.yaml
var capacityPodTemplateYAML string

func testScheduler() {
	var cc CleanupContext
	BeforeEach(func() {
		cc = commonBeforeEach()
	})
	AfterEach(func() {
		commonAfterEach(cc)
	})

	testNamespacePrefix := "scheduler-test"

	It("should be deployed topolvm-scheduler pod", func() {
		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n=topolvm-system", "pods", "--selector=app.kubernetes.io/name=topolvm-scheduler", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var podlist corev1.PodList
			err = json.Unmarshal(result, &podlist)
			if err != nil {
				return err
			}

			if len(podlist.Items) == 0 {
				return errors.New("pod is not found")
			}

			for _, pod := range podlist.Items {
				podReady := false
				for _, cond := range pod.Status.Conditions {
					fmt.Fprintln(GinkgoWriter, cond)
					if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
						podReady = true
						break
					}
				}
				if !podReady {
					return errors.New("topolvm-scheduler is not yet ready")
				}
			}

			return nil
		}).Should(Succeed())
	})

	It("should schedule pod if requested capacity is sufficient", func() {
		ns := testNamespacePrefix + randomString(10)
		createNamespace(ns)
		stdout, stderr, err := kubectlWithInput([]byte(fmt.Sprintf(capacityPodTemplateYAML, ns, "1073741824")), "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n", ns, "pods/testhttpd", "-o=json")
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

			return fmt.Errorf("testhttpd is not yet ready: %v", pod.Status)
		}).Should(Succeed())
		kubectl("delete", "namespaces", ns)
	})

	It("should not schedule pod if requested capacity is not sufficient", func() {
		ns := testNamespacePrefix + randomString(10)
		createNamespace(ns)
		stdout, stderr, err := kubectlWithInput([]byte(fmt.Sprintf(capacityPodTemplateYAML, ns, "21474836480")), "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n", ns, "pods/testhttpd", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var pod corev1.Pod
			err = json.Unmarshal(result, &pod)
			if err != nil {
				return err
			}

			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse {
					return nil
				}
			}

			return errors.New("testhttpd should not be scheduled")
		}).Should(Succeed())
		kubectl("delete", "namespaces", ns)
	})
}
