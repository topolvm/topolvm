// This provides not test itself but helpers.

package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
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

func kubectl(args ...string) (stdout []byte, stderr []byte, err error) {
	return execAtLocal(kubectlPath, nil, args...)
}

func kubectlWithInput(input []byte, args ...string) (stdout []byte, stderr []byte, err error) {
	return execAtLocal(kubectlPath, input, args...)
}

func countLVMs() (int, error) {
	stdout, err := exec.Command("sudo", "lvs", "-o", "lv_name", "--noheadings").Output()
	if err != nil {
		return -1, fmt.Errorf("failed to lvs. stdout %s, err %v", stdout, err)
	}
	return bytes.Count(stdout, []byte("\n")), nil
}

func getNodeAnnotationMapWithPrefix(prefix string) (map[string]map[string]string, error) {
	stdout, stderr, err := kubectl("get", "node", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("stdout=%sr stderr=%s, err=%v", stdout, stderr, err)
	}

	var nodeList corev1.NodeList
	err = json.Unmarshal(stdout, &nodeList)
	if err != nil {
		return nil, err
	}

	capacities := make(map[string]map[string]string)
	for _, node := range nodeList.Items {
		if node.Name == "topolvm-e2e-control-plane" {
			continue
		}

		capacities[node.Name] = make(map[string]string)
		for k, v := range node.Annotations {
			if !strings.HasPrefix(k, prefix) {
				continue
			}
			capacities[node.Name][k] = v
		}
	}
	return capacities, nil
}

func commonBeforeEach() CleanupContext {
	var cc CleanupContext
	var err error

	cc.LvmCount, err = countLVMs()
	ExpectWithOffset(1, err).ShouldNot(HaveOccurred())

	cc.CapacityAnnotations, err = getNodeAnnotationMapWithPrefix(topolvm.GetCapacityKeyPrefix())
	ExpectWithOffset(1, err).ShouldNot(HaveOccurred())

	return cc
}

func commonAfterEach(cc CleanupContext) {
	if !CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
		EventuallyWithOffset(1, func() error {
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

			capacitiesAfter, err := getNodeAnnotationMapWithPrefix(topolvm.GetCapacityKeyPrefix())
			if err != nil {
				return err
			}
			if diff := cmp.Diff(cc.CapacityAnnotations, capacitiesAfter); diff != "" {
				return fmt.Errorf("capacities on nodes should be same before and after the test: diff=%s", diff)
			}
			return nil
		}).Should(Succeed())
	}
}

type lvinfo struct {
	lvPath string
	size   int
	vgName string
}

func getLVInfo(lvName string) (*lvinfo, error) {
	stdout, err := exec.Command("sudo", "lvdisplay", "-c", "--select", "lv_name="+lvName).Output()
	if err != nil {
		return nil, fmt.Errorf("err=%v, stdout=%s", err, stdout)
	}
	output := strings.TrimSpace(string(stdout))
	if output == "" {
		return nil, fmt.Errorf("lv_name ( %s ) not found", lvName)
	}
	lines := strings.Split(output, "\n")
	if len(lines) != 1 {
		return nil, errors.New("found multiple lvs")
	}
	// lvdisplay -c format is here
	// https://github.com/lvmteam/lvm2/blob/baf99ff974b408c59dd4f51db6e006d659c061e7/lib/display/display.c#L353
	items := strings.Split(strings.TrimSpace(lines[0]), ":")
	if len(items) < 7 {
		return nil, fmt.Errorf("invalid format: %s", lines[0])
	}
	size, err := strconv.Atoi(items[6])
	if err != nil {
		return nil, err
	}

	return &lvinfo{
		lvPath: items[0],
		vgName: items[1],
		size:   size * 512, // lvdisplay denotes size as 512 byte block
	}, nil
}
