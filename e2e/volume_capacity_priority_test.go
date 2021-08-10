package e2e

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
)

//go:embed testdata/volume_capacity_priority/pod-pvc-for-proiority-template.yaml
var podPVCPriotityTemplateYAML string

func testVolumeCapacityPriority() {
	ns := "volume-capacity-priority"
	var cc CleanupContext

	BeforeEach(func() {
		cc = commonBeforeEach()
		createNamespace(ns)
	})

	AfterEach(func() {
		kubectl("delete", "namespaces/"+ns)
		commonAfterEach(cc)
	})

	It("should scheduled to appropriate nodes", func() {
		if !isStorageCapacity() || !isVolumeCapacityPriority() || isDaemonsetLvmdEnvSet() {
			Skip("This test run when only enable the VolumeCapacityPriority feature gate")
			return
		}

		for i := 1; i <= 3; i++ {
			candidateNodes := getCandidateNodes()
			outputCandidateNodes(candidateNodes)
			podName := fmt.Sprintf("testpod-%d", i)
			pvcName := fmt.Sprintf("testpvc-%d", i)
			size := "5Gi"

			podPVCPriotityYaml := buildPodPVCPriotityTemplateYAML(ns, podName, pvcName, size)
			_, _, err := kubectlWithInput(podPVCPriotityYaml, "apply", "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())
			nodeName := checkingPodPVCStatus(ns, pvcName, podName)
			fmt.Printf("Scheduled node: %s\n", nodeName)
			Expect(isExistingNode(nodeName, candidateNodes)).Should(BeTrue())
		}
	})
}

func buildPodPVCPriotityTemplateYAML(ns, pod, pvc, size string) []byte {
	return []byte(fmt.Sprintf(podPVCPriotityTemplateYAML, pvc, ns, size, pod, ns, pvc))
}

func checkingPodPVCStatus(ns, pvcName, podName string) string {
	var nodeName string
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
				nodeName = pod.Spec.NodeName
				return nil
			}
		}

		return errors.New("pod is not running")
	}).Should(Succeed())
	Expect(nodeName).ShouldNot(BeEmpty())
	return nodeName
}

func getCandidateNodes() []string {
	var maxCapNodes []string
	Eventually(func() error {
		var maxCapacity int
		stdout, stderr, err := kubectl("get", "nodes", "-o", "json")
		if err != nil {
			return fmt.Errorf("kubectl get nodes error: stdout=%s, stderr=%s", stdout, stderr)
		}
		var nodes corev1.NodeList
		err = json.Unmarshal(stdout, &nodes)
		if err != nil {
			return fmt.Errorf("unmarshal error: stdout=%s", stdout)
		}
		for _, node := range nodes.Items {
			if node.Name == "topolvm-e2e-control-plane" {
				continue
			}
			strCap, ok := node.Annotations[topolvm.CapacityKeyPrefix+"ssd"]
			if !ok {
				return fmt.Errorf("capacity is not annotated: %s", node.Name)
			}
			capacity, err := strconv.Atoi(strCap)
			if err != nil {
				return err
			}
			switch {
			case capacity > maxCapacity:
				maxCapacity = capacity
				maxCapNodes = []string{node.GetName()}
			case capacity == maxCapacity:
				maxCapNodes = append(maxCapNodes, node.GetName())
			}
			fmt.Printf("%s: %d bytes\n", node.Name, capacity)
		}

		return nil
	}).Should(Succeed())
	return maxCapNodes
}

func outputCandidateNodes(candidateNodes []string) {
	fmt.Println("caididate nodes")
	for _, candidateNode := range candidateNodes {
		fmt.Println(candidateNode)
	}
	fmt.Println("")
}

func isExistingNode(node string, nodes []string) bool {
	for _, n := range nodes {
		if n == node {
			return true
		}
	}
	return false
}
