package microtest_test

import (
	"bytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cybozu-go/topolvm/microtest"
)

var _ = Describe("Test topolvm-scheduler", func() {
	It("should be deployed topolvm-scheduler pod", func() {

	})

	It("should schedule pod if requested capacity is sufficient", func() {

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

func kubectl(args ...string) ([]byte, []byte, error) {
	// args = append([]string{"/opt/bin/kubectl"}, args...)
	return execAtLocal("/snap/bin/microk8s.kubectl", nil, args...)
}

func kubectlWithInput(input []byte, args ...string) ([]byte, []byte, error) {
	// args = append([]string{"/opt/bin/kubectl"}, args...)
	return execAtLocal("/snap/bin/microk8s.kubectl", input, args...)
}