package microtest

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMtest(t *testing.T) {
	circleci := os.Getenv("CIRCLECI") == "true"
	if circleci {
		executorType := os.Getenv("CIRCLECI_EXECUTOR")
		if executorType != "machine" {
			t.Skip("run on machine executor")
		}
	}

	RegisterFailHandler(Fail)

	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultEventuallyTimeout(time.Minute)

	RunSpecs(t, "Test on microk8s")
}

func createNamespace(ns string) {
	stdout, stderr, err := kubectl("create", "namespace", ns)
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	Eventually(func() error {
		return waitCreatingDefaultSA(ns)
	}).Should(Succeed())
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
	//
	// Prepare for E2E test
	//
	createNamespace("topolvm-system")
	stdout, stderr, err := kubectl("apply", "-f", "../topolvm-node/config/crd/bases/topolvm.cybozu.com_logicalvolumes.yaml")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	stdout, stderr, err = kubectl("apply", "-f", "./csi.yml")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	Eventually(func() error {
		stdout, stderr, err = kubectl("get", "pod", "-n=kube-system", "-o=custom-columns=:.status.phase", "--no-headers")
		if err != nil {
			return fmt.Errorf("stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
		}
		for _, l := range strings.Split(strings.TrimSpace(string(stdout)), "\n") {
			if l != "Running" {
				return errors.New("there is a pod not running")
			}
		}
		return nil
	}).Should(Succeed())
})
