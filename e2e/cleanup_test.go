package e2e

import (
	"encoding/json"
	"fmt"

	"github.com/cybozu-go/topolvm"
	"github.com/cybozu-go/topolvm/controller/controllers"
	logicalvolumev1 "github.com/cybozu-go/topolvm/topolvm-node/api/v1"
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

		statefulsetName := "test-sts"
		By("applying statefulset")
		statefulsetYAML := `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ` + statefulsetName + `
  labels:
    app.kubernetes.io/name: test-sts-container
spec:
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
          image: quay.io/cybozu/ubuntu:18.04
          command: ["pause"]
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
			stdout, stderr, err := kubectl("-n", cleanupTest, "get", "statefulset", statefulsetName, "-o=json")
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
		var deletedPod corev1.Pod
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
						deletedPod = pod
					}
				}
			}
		}
		stdout, stderr, err = kubectl("-n", cleanupTest, "get", "pvc", "-o=json")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		var deletedPVC corev1.PersistentVolumeClaim
		var pvcs corev1.PersistentVolumeClaimList
		err = json.Unmarshal(stdout, &pvcs)
		Expect(err).ShouldNot(HaveOccurred())
		for _, pvc := range pvcs.Items {
			if _, ok := pvc.Annotations[controllers.AnnSelectedNode]; !ok {
				continue
			}
			if pvc.Annotations[controllers.AnnSelectedNode] != deletedNode {
				continue
			}
			deletedPVC = pvc
		}

		stdout, stderr, err = kubectl("get", "logicalvolume", "-o=json")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		var deleteLogicalVolumes []logicalvolumev1.LogicalVolume
		var logicalVolumeList logicalvolumev1.LogicalVolumeList
		err = json.Unmarshal(stdout, &logicalVolumeList)
		Expect(err).ShouldNot(HaveOccurred())
		for _, lv := range logicalVolumeList.Items {
			if lv.Spec.NodeName == deletedNode {
				deleteLogicalVolumes = append(deleteLogicalVolumes, lv)
			}
		}

		By("deleting Node kind-worker3")
		stdout, stderr, err = kubectl("delete", "node", deletedNode)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming pvc/pod")
		_, _, err = kubectl("-n", cleanupTest, "get", "pvc", deletedPVC.Name)
		Expect(err).Should(HaveOccurred())
		stdout, _, err = kubectl("-n", cleanupTest, "get", "pod", deletedPod.Name, "-o=json")
		if err == nil {
			var rescheduledPod corev1.Pod
			err = json.Unmarshal(stdout, &rescheduledPod)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(rescheduledPod.ObjectMeta.UID).ShouldNot(Equal(deletedPod.ObjectMeta.UID))
		}

		By("confirming logicalvolumes are deleted")
		Eventually(func() error {
			for _, lv := range deleteLogicalVolumes {
				_, _, err := kubectl("get", "logicalvolume", lv.Name)
				if err == nil {
					return fmt.Errorf("logicalvolume still exists: %s", lv.Name)
				}
			}
			return nil
		}).Should(Succeed())

		By("confirming statefulset is ready")
		stdout, stderr, err = kubectl("-n", cleanupTest, "delete", "pod", deletedPod.Name)
		Expect(err).Should(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Eventually(func() error {
			stdout, stderr, err := kubectl("-n", cleanupTest, "get", "statefulset", statefulsetName, "-o=json")
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

		By("confirming pvc is recreated")
		stdout, stderr, err = kubectl("-n", cleanupTest, "get", "pvc", deletedPVC.Name, "-o=json")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		var recreatedPVC corev1.PersistentVolumeClaim
		err = json.Unmarshal(stdout, &recreatedPVC)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(recreatedPVC.ObjectMeta.UID).ShouldNot(Equal(deletedPVC.ObjectMeta.UID))
	})
}
