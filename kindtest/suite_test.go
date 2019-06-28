package kindtest

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
)

func TestMtest(t *testing.T) {
	circleci := os.Getenv("CIRCLECI") == "true"
	if circleci {
		executorType := os.Getenv("CIRCLECI_EXECUTOR")
		if executorType != "machine" {
			t.Skip("run on machine executor")
		}
	}

	rand.Seed(time.Now().UnixNano())

	RegisterFailHandler(Fail)

	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultEventuallyTimeout(time.Minute)

	RunSpecs(t, "Test on sanity")
}

func createNamespace(ns string) {
	stdout, stderr, err := kubectl("create", "namespace", ns)
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	Eventually(func() error {
		return waitCreatingDefaultSA(ns)
	}).Should(Succeed())
	fmt.Fprintln(os.Stderr, "created namespace: "+ns)
}

func randomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxyz")

	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

var _ = BeforeSuite(func() {
	fmt.Println("Waiting for mutating webhook to get ready")
	Eventually(func() error {
		stdout, stderr, err := kubectl("-n=kube-system", "get", "ds/kindnet", "-o", "json")
		if err != nil {
			return errors.New(string(stderr))
		}

		var ds appsv1.DaemonSet
		err = json.Unmarshal(stdout, &ds)
		if err != nil {
			return err
		}

		if ds.Status.NumberReady != 4 {
			return fmt.Errorf("numberReady is not 4: %d", ds.Status.NumberReady)
		}
		return nil
	}).Should(Succeed())
	time.Sleep(5 * time.Second)
	Eventually(func() error {
		stdout, stderr, err := kubectl("-n=kube-system", "get", "ds/kindnet", "-o", "json")
		if err != nil {
			return errors.New(string(stderr))
		}

		var ds appsv1.DaemonSet
		err = json.Unmarshal(stdout, &ds)
		if err != nil {
			return err
		}

		if ds.Status.NumberReady != 4 {
			return fmt.Errorf("numberReady is not 4: %d", ds.Status.NumberReady)
		}
		return nil
	}).Should(Succeed())

	podYAML := `apiVersion: v1
kind: Pod
metadata:
  name: ubuntu
  labels:
    app.kubernetes.io/name: ubuntu
spec:
  containers:
    - name: ubuntu
      image: quay.io/cybozu/ubuntu:18.04
      command: ["sleep", "infinity"]
`
	Eventually(func() error {
		_, stderr, err := kubectlWithInput([]byte(podYAML), "apply", "-f", "-")
		if err != nil {
			return errors.New(string(stderr))
		}
		return nil
	}).Should(Succeed())
})
