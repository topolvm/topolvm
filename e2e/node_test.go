package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/expfmt"
	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
)

func testNode() {
	var cc CleanupContext
	BeforeEach(func() {
		cc = commonBeforeEach()
	})
	AfterEach(func() {
		commonAfterEach(cc)
	})

	It("should be deployed", func() {
		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n=topolvm-system", "pods", "--selector=app.kubernetes.io/component=node,app.kubernetes.io/name=topolvm", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var podlist corev1.PodList
			err = json.Unmarshal(result, &podlist)
			if err != nil {
				return err
			}

			count := 3
			if isDaemonsetLvmdEnvSet() {
				count = 1
			}

			if len(podlist.Items) != count {
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
					return errors.New("topolvm-node is not yet ready: " + pod.Name)
				}
			}
			return nil
		}).Should(Succeed())
	})

	It("should annotate capacity to node", func() {
		stdout, stderr, err := kubectl("get", "nodes", "-o=json")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		count := 4
		if isDaemonsetLvmdEnvSet() {
			count = 1
		}

		var nodes corev1.NodeList
		err = json.Unmarshal(stdout, &nodes)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(len(nodes.Items)).To(Equal(count))

		vgNameMap := map[string]string{
			"topolvm-e2e-worker":        "node1-myvg1",
			"topolvm-e2e-worker2":       "node2-myvg1",
			"topolvm-e2e-worker3":       "node3-myvg1",
			"topolvm-e2e-control-plane": "",
		}
		if isDaemonsetLvmdEnvSet() {
			vgNameMap = map[string]string{}
			vgNameMap[nodes.Items[0].Name] = "node-myvg1"
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
			val, ok := node.Annotations[topolvm.GetCapacityKeyPrefix()+"ssd"]
			Expect(ok).To(Equal(true), "capacity is not annotated: "+node.Name)
			Expect(val).To(Equal(strings.TrimSpace(string(targetBytes))), "unexpected capacity: "+node.Name)
		}
	})

	It("should expose Prometheus metrics", func() {
		stdout, stderr, err := kubectl("get", "pods", "-n=topolvm-system", "-l=app.kubernetes.io/component=node,app.kubernetes.io/name=topolvm", "-o=json")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		var pods corev1.PodList
		err = json.Unmarshal(stdout, &pods)
		Expect(err).ShouldNot(HaveOccurred())

		pod := pods.Items[0]
		stdout, stderr, err = kubectl("exec", "-n", "topolvm-system", pod.Name, "-c=topolvm-node", "--", "curl", "http://localhost:8080/metrics")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		var parser expfmt.TextParser
		metricFamilies, err := parser.TextToMetricFamilies(bytes.NewReader(stdout))
		Expect(err).ShouldNot(HaveOccurred())

		foundSize := false
		for _, family := range metricFamilies {
			if family.GetName() == "topolvm_volumegroup_size_bytes" {
				foundSize = true
			}
		}
		Expect(foundSize).Should(BeTrue())

		found := false
		for _, family := range metricFamilies {
			if family.GetName() != "topolvm_volumegroup_available_bytes" {
				continue
			}
			found = true

			length := 3
			if isDaemonsetLvmdEnvSet() {
				length = 4
			}
			Expect(family.Metric).Should(HaveLen(length))

			stdout, stderr, err := kubectl("get", "node", pod.Spec.NodeName, "-o=json")
			Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

			var node corev1.Node
			err = json.Unmarshal(stdout, &node)
			Expect(err).ShouldNot(HaveOccurred())
			for k, v := range node.Annotations {
				if !strings.HasPrefix(k, topolvm.GetCapacityKeyPrefix()) {
					continue
				}
				dc := k[len(topolvm.GetCapacityKeyPrefix()):]
				if dc == topolvm.DefaultDeviceClassAnnotationName {
					continue
				}
				expected, err := strconv.ParseFloat(v, 64)
				Expect(err).ShouldNot(HaveOccurred())

				var targetValue *float64
				for _, m := range family.Metric {
					for _, label := range m.Label {
						if *label.Name == "device_class" && *label.Value == dc {
							targetValue = m.Gauge.Value
							break
						}
					}
				}
				Expect(targetValue).ShouldNot(BeNil())
				Expect(*targetValue).Should(BeNumerically("==", expected))
			}
			break
		}
		Expect(found).Should(BeTrue())
	})
}
