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
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	corev1 "k8s.io/api/core/v1"
)

type CleanupContext struct {
	LvmCount            int
	CapacityAnnotations map[string]map[string]string
}

// execAtLocal executes cmd.
func execAtLocal(cmd string, input []byte, args ...string) (stdout []byte, err error) {
	var stdoutBuf, stderrBuf bytes.Buffer
	command := exec.Command(cmd, args...)
	command.Stdout = &stdoutBuf
	command.Stderr = &stderrBuf

	if len(input) != 0 {
		command.Stdin = bytes.NewReader(input)
	}

	err = command.Run()
	stdout = stdoutBuf.Bytes()
	stderr := stderrBuf.Bytes()
	if err != nil {
		err = fmt.Errorf("%s failed. stdout=%s, stderr=%s, err=%w", cmd, stdout, stderr, err)
	}
	return
}

// kubectl executes kubectl command.
func kubectl(args ...string) (stdout []byte, err error) {
	return execAtLocal(kubectlPath, nil, args...)
}

// kubectlWithInput executes kubectl command with stdin input.
func kubectlWithInput(input []byte, args ...string) (stdout []byte, err error) {
	return execAtLocal(kubectlPath, input, args...)
}

func countLVMs() (int, error) {
	stdout, err := execAtLocal("sudo", nil, "lvs", "-o", "lv_name", "--noheadings")
	if err != nil {
		return -1, err
	}
	return bytes.Count(stdout, []byte("\n")), nil
}

func getNodeAnnotationMapWithPrefix(prefix string) (map[string]map[string]string, error) {
	var nodeList corev1.NodeList
	err := getObjects(&nodeList, "node")
	if err != nil {
		return nil, err
	}

	capacities := make(map[string]map[string]string)
	for _, node := range nodeList.Items {
		if node.Name == controlPlaneNodeName {
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

var ErrObjectNotFound = errors.New("no such object")

// getObjects get kubernetes objects into obj.
// obj can be an object (e.g. Pod) or a list of objects (e.g. PodList).
// If any objects are not found, return ErrObjectNotFound error.
func getObjects(obj any, kind string, args ...string) error {
	stdout, err := kubectl(append([]string{"get", "-ojson", "--ignore-not-found", kind}, args...)...)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(stdout)) == "" {
		return ErrObjectNotFound
	}
	return json.Unmarshal(stdout, obj)
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

type ErrLVNotFound struct {
	lvName string
}

func (e ErrLVNotFound) Error() string {
	return fmt.Sprintf("lv_name ( %s ) not found", e.lvName)
}

type lvinfo struct {
	size     int
	poolName string
	vgName   string
}

func getLVInfo(lvName string) (*lvinfo, error) {
	stdout, err := execAtLocal("sudo", nil,
		"lvs", "--noheadings", "-o", "lv_size,pool_lv,vg_name",
		"--units", "b", "--nosuffix", "--separator", ":",
		"--select", fmt.Sprintf("lv_name=%s", lvName))
	if err != nil {
		return nil, err
	}
	output := strings.TrimSpace(string(stdout))
	if output == "" {
		return nil, ErrLVNotFound{lvName: lvName}
	}
	if strings.Contains(output, "\n") {
		return nil, errors.New("found multiple lvs")
	}
	items := strings.Split(output, ":")
	if len(items) != 3 {
		return nil, fmt.Errorf("invalid format: %s", output)
	}
	size, err := strconv.Atoi(items[0])
	if err != nil {
		return nil, err
	}
	return &lvinfo{
		size:     size,
		poolName: items[1],
		vgName:   items[2],
	}, nil
}

func checkLVIsRegisteredInLVM(volName string) error {
	var lv topolvmv1.LogicalVolume
	err := getObjects(&lv, "logicalvolumes", volName)
	if err != nil {
		return err
	}
	_, err = getLVInfo(string(lv.UID))
	return err
}

func checkLVIsDeletedInLVM(lvName string) error {
	_, err := getLVInfo(lvName)
	if err != nil {
		if _, ok := err.(ErrLVNotFound); ok {
			return nil
		}
		return err
	}
	return fmt.Errorf("target LV exists %s", lvName)
}

func getLVNameOfPVC(pvcName, ns string) (lvName string, err error) {
	var pvc corev1.PersistentVolumeClaim
	err = getObjects(&pvc, "pvc", "-n", ns, pvcName)
	if err != nil {
		return "", fmt.Errorf("failed to get PVC. err: %w", err)
	}

	if pvc.Status.Phase != corev1.ClaimBound {
		return "", errors.New("pvc status is not bound")
	}
	if pvc.Spec.VolumeName == "" {
		return "", errors.New("pvc.Spec.VolumeName should not be empty")
	}

	var lv topolvmv1.LogicalVolume
	err = getObjects(&lv, "logicalvolume", pvc.Spec.VolumeName)
	if err != nil {
		return "", fmt.Errorf("failed to get LV. err: %w", err)
	}

	return string(lv.UID), nil
}
