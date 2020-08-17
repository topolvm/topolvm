package e2e

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/topolvm/topolvm"
)

type CleanupContext struct {
	LvmCount            int
	CapacityAnnotations map[string]map[string]string
}

func execAtLocal(cmd string, input []byte, args ...string) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
	command := exec.Command(cmd, args...)
	command.Stdout = &stdout
	command.Stderr = &stderr

	if len(input) != 0 {
		command.Stdin = bytes.NewReader(input)
	}

	err := command.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func kubectl(args ...string) ([]byte, []byte, error) {
	return execAtLocal("kubectl", nil, args...)
}

func kubectlWithInput(input []byte, args ...string) ([]byte, []byte, error) {
	return execAtLocal("kubectl", input, args...)
}

func containString(s []string, target string) bool {
	for _, ss := range s {
		if ss == target {
			return true
		}
	}
	return false
}

func commonBeforeEach() CleanupContext {
	var cc CleanupContext
	var err error

	cc.LvmCount, err = countLVMs()
	Expect(err).ShouldNot(HaveOccurred())

	cc.CapacityAnnotations, err = getNodeAnnotationMapWithPrefix(topolvm.CapacityKeyPrefix)
	Expect(err).ShouldNot(HaveOccurred())

	return cc
}

func commonAfterEach(cc CleanupContext) {
	if !CurrentGinkgoTestDescription().Failed {
		Eventually(func() error {
			lvmCountAfter, err := countLVMs()
			if err != nil {
				return err
			}
			if cc.LvmCount != lvmCountAfter {
				return fmt.Errorf("lvm num mismatched. before: %d, after: %d", cc.LvmCount, lvmCountAfter)
			}

			stdout, stderr, err := kubectl("get", "node", "-o", "json")
			if err != nil {
				return fmt.Errorf("stdout=%s, stderr=%s", stdout, stderr)
			}

			capacitiesAfter, err := getNodeAnnotationMapWithPrefix(topolvm.CapacityKeyPrefix)
			if err != nil {
				return err
			}
			if diff := cmp.Diff(cc.CapacityAnnotations, capacitiesAfter); diff != "" {
				return fmt.Errorf("capacities on nodes should be same before and after the test: diff=%q", diff)
			}
			return nil
		}).Should(Succeed())
	}
}
