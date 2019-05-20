package microtest

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cybozu-go/topolvm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Test for lvmetrics", func() {

	It("should be deployed lvmetrics pod", func() {
		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n=kube-system", "pods", "--selector=app.kubernetes.io/name=lvmetrics", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var podlist corev1.PodList
			err = json.Unmarshal(result, &podlist)
			if err != nil {
				return err
			}

			if len(podlist.Items) != 1 {
				return errors.New("pod is not found")
			}

			pod := podlist.Items[0]
			for _, cond := range pod.Status.Conditions {
				fmt.Println(cond)
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return nil
				}
			}

			return errors.New("lvmetrics is not yet ready")
		}).Should(Succeed())
	})

	It("should annotate capacity to node", func() {
		Eventually(func() error {
			result, stderr, err := kubectl("get", "nodes", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var nodes corev1.NodeList
			err = json.Unmarshal(result, &nodes)
			if err != nil {
				return err
			}

			if len(nodes.Items) != 1 {
				return errors.New("node is not found")
			}

			node := nodes.Items[0]
			val, ok := node.Annotations[topolvm.CapacityKey]
			if !ok {
				return errors.New("not annotated")
			}
			if val != "5368709120" {
				return errors.New("unexpected capacity: " + val)
			}

			return nil
		}).Should(Succeed())
	})
})
