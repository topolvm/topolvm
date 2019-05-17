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
				return errors.New("pod is not found.")
			}

			pod := podlist.Items[0]
			for _, cond := range pod.Status.Conditions {
				fmt.Println(cond)
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return nil
				}
			}

			return errors.New("pod is not yet ready.")
		}).Should(Succeed())
	})

	It("should schedule pod if requested capacity is sufficient", func() {
		podYml := `
apiVersion: v1
kind: Pod
metadata:
  name: testapp-pod
  labels:
    app.kubernetes.io/name: testapp
spec:
  containers:
  - name: test-container
    image: quey.io/cybozu/testhttpd:0
	resources:
	  requests:
    	topolvm.cybozu.com/capacity: 1Gi
	  limits:
		topolvm.cybozu.com/capacity: 1Gi
`

	})

	It("should not schedule pod if requested capacity is not sufficient", func() {

	})

})

func execAtLocal(cmd string, input []byte, args ...string) ([]byte, error) {
	fmt.Println(args)

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
