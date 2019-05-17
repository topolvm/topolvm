package microtest_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Test topolvm-scheduler", func() {
	BeforeEach(func() {
		kubectl("delete", "namespace", "scheduler-test")
		_, err := kubectl("create", "namespace", "scheduler-test")
		Expect(err).ShouldNot(HaveOccurred())
	})
	AfterEach(func() {
		kubectl("delete", "namespace", "scheduler-test")
	})

	It("should be deployed topolvm-scheduler pod", func() {
		Eventually(func() error {
			result, err := kubectl("get", "-n=kube-system", "pods", "--selector=app.kubernetes.io/name=topolvm-scheduler", "-o=json")
			if err != nil {
				return err
			}

			var podlist corev1.PodList
			err = json.Unmarshal(result, &podlist)
			if err != nil {
				return err
			}

			if len(podlist.Items) != 1 {
				return errors.New("pod is not found")
			}

			pod := podlist.Items[0]
			for _, cond := range pod.Status.Conditions {
				fmt.Println(cond)
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return nil
				}
			}

			return errors.New("topolvm-scheduler is not yet ready")
		}).Should(Succeed())
	})

	It("should schedule pod if requested capacity is sufficient", func() {
		podYml := `
apiVersion: v1
kind: Pod
metadata:
  name: testhttpd
  namespace: scheduler-test
  labels:
    app.kubernetes.io/name: testhttpd
spec:
  containers:
  - name: testhttpd
    image: quay.io/cybozu/testhttpd:0
    resources:
      requests:
        topolvm.cybozu.com/capacity: 1Gi
      limits:
        topolvm.cybozu.com/capacity: 1Gi
`
		_, err := kubectlWithInput([]byte(podYml), "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			result, err := kubectl("get", "-n=scheduler-test", "pods/testhttpd", "-o=json")
			if err != nil {
				return err
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

			return errors.New("testhttpd is not yet ready")
		}).Should(Succeed())
	})

	It("should not schedule pod if requested capacity is not sufficient", func() {

	})

})

func execAtLocal(cmd string, input []byte, args ...string) ([]byte, error) {
	var stdout bytes.Buffer
	command := exec.Command(cmd, args...)
	command.Stdout = &stdout
	command.Stderr = GinkgoWriter

	if len(input) != 0 {
		command.Stdin = bytes.NewReader(input)
	}

	err := command.Run()
	if err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

func kubectl(args ...string) ([]byte, error) {
	return execAtLocal("/snap/bin/microk8s.kubectl", nil, args...)
}

func kubectlWithInput(input []byte, args ...string) ([]byte, error) {
	return execAtLocal("/snap/bin/microk8s.kubectl", input, args...)
}
