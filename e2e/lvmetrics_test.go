package e2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cybozu-go/topolvm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

func testLvmetrics() {
	It("should be deployed csi-topolvm-node pod", func() {
		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n=topolvm-system", "pods", "--selector=app.kubernetes.io/name=csi-topolvm-node", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var podlist corev1.PodList
			err = json.Unmarshal(result, &podlist)
			if err != nil {
				return err
			}

			if len(podlist.Items) != 3 {
				return fmt.Errorf("the number of pods is not equal to 3: %d", len(podlist.Items))
			}

			for _, pod := range podlist.Items {
				isReady := false
				for _, cond := range pod.Status.Conditions {
					if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
						isReady = true
						break
					}
				}
				if !isReady {
					return errors.New("csi-topolvm-node is not yet ready: " + pod.Name)
				}
			}
			return nil
		}).Should(Succeed())
	})

	It("should annotate capacity to node", func() {
		stdout, stderr, err := kubectl("get", "nodes", "-o=json")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		var nodes corev1.NodeList
		err = json.Unmarshal(stdout, &nodes)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(len(nodes.Items)).To(Equal(4))

		vgNameMap := map[string]string{
			"kind-worker":        "myvg1",
			"kind-worker2":       "myvg2",
			"kind-worker3":       "myvg3",
			"kind-control-plane": "",
		}

		for _, node := range nodes.Items {
			vgName, ok := vgNameMap[node.Name]
			if !ok {
				panic(node.Name + " does not exist")
			}

			if len(vgName) == 0 {
				continue
			}

			By("checking " + node.Name)
			targetBytes, stderr, err := execAtLocal("sudo", nil, "vgs",
				"-o", "vg_free",
				"--noheadings",
				"--units=b",
				"--nosuffix",
				vgName,
			)
			Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", targetBytes, stderr)
			val, ok := node.Annotations[topolvm.CapacityKey]
			Expect(ok).To(Equal(true), "capacity is not annotated: "+node.Name)
			Expect(val).To(Equal(strings.TrimSpace(string(targetBytes))), "unexpected capacity: "+node.Name)
		}
	})
}
