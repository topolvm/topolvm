package e2e

import (
	"encoding/json"
	"fmt"

	"github.com/cybozu-go/topolvm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const cleanupTest = "cleanup-test"

func testCleanup() {
	BeforeEach(func() {
		createNamespace(cleanupTest)
	})
	AfterEach(func() {
		kubectl("delete", "namespaces/"+cleanupTest)
	})

	It("should be finalized by node", func() {
		By("checking Node finalizer")
		stdout, stderr, err := kubectl("get", "nodes", "-l=node-role.kubernetes.io/master!=", "-o=json")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		var nodes corev1.NodeList
		err = json.Unmarshal(stdout, &nodes)
		Expect(err).ShouldNot(HaveOccurred())
		for _, node := range nodes.Items {
			topolvmFinalize := false
			for _, fn := range node.Finalizers {
				if fn == topolvm.NodeFinalizer {
					topolvmFinalize = true
				}
			}
			Expect(topolvmFinalize).To(Equal(true))
		}

		By("applying statefulset")
		statefulsetYAML := `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test-sts
  labels:
    app.kubernetes.io/name: test-sts-container
spec:
  serviceName: "test-sts"
  replicas: 3
  podManagementPolicy: Parallel
  selector:
    matchLabels:
      app.kubernetes.io/name: test-sts-container
  template:
    metadata:
      labels:
        app.kubernetes.io/name: test-sts-container
    spec:
      containers:
        - name: ubuntu
          image: ubuntu:18.04
          command: ["sleep", "infinity"]
          volumeMounts:
          - mountPath: /test1
            name: test-sts-pvc
  volumeClaimTemplates:
  - metadata:
      name: test-sts-pvc
    spec:
      accessModes: [ "ReadWriteOnce" ]
      storageClassName: topolvm-provisioner
      resources:
        requests:
          storage: 1Gi`
		stdout, stderr, err = kubectlWithInput([]byte(statefulsetYAML), "-n", cleanupTest, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		Eventually(func() error {
			stdout, stderr, err := kubectl("-n", cleanupTest, "get", "statefulset", "test-sts", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, stdout, stderr)
			}
			var st appsv1.StatefulSet
			err = json.Unmarshal(stdout, &st)
			if err != nil {
				return fmt.Errorf("failed to unmarshal")
			}
			if st.Status.ReadyReplicas != 3 {
				return fmt.Errorf("statefulset replica is not 3: %d", st.Status.ReadyReplicas)
			}
			return nil
		}).Should(Succeed())

		By("checking target pvcs/pods")
		deletedNode := "kind-worker3"
		var deletedPodsPVCs map[string]string
		stdout, stderr, err = kubectl("-n", cleanupTest, "get", "pods", "-o=json")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		var pods corev1.PodList
		err = json.Unmarshal(stdout, &pods)
		Expect(err).ShouldNot(HaveOccurred())
		for _, pod := range pods.Items {
			if pod.Spec.NodeName == deletedNode {
				for _, volume := range pod.Spec.Volumes {
					if volume.PersistentVolumeClaim == nil {
						continue
					}
					if volume.PersistentVolumeClaim.ClaimName == "test-sts-pvc" {
						deletedPodsPVCs[pod.Name] = volume.PersistentVolumeClaim.ClaimName
					}
				}
			}
		}

		By("draining Node kind-worker3")
		//		stdout, stderr, err = kubectl("drain", "--ignore-daemonsets", deletedNode)
		//		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		//
		By("confirming deleted Node is tainted")
		//		//stdout, stderr, err := kubectl("get", "nodes", deletedNode, "-o=json")
		//		//Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		//		//var node corev1.Node
		//		//err = json.Unmarshal(stdout, &node)
		//		//Expect(err).ShouldNot(HaveOccurred())
		//
		By("deleting Node kind-worker3")
		//		stdout, stderr, err = kubectl("delete", "node", deletedNode)
		//		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		//
		By("confirming logicalvolumes are deleted")

		By("confirming statefulset is ready")
	})
}
