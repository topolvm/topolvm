package e2e

import (
	_ "embed"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const nsNodeDeleteTest = "node-delete-test"

//go:embed testdata/node_delete/statefulset-template.yaml
var statefulSetTemplateYAML string

func testNodeDelete() {

	BeforeEach(func() {
		skipIfSingleNode()

		createNamespace(nsNodeDeleteTest)
	})

	AfterEach(func() {
		_, err := kubectl("delete", "namespaces", nsNodeDeleteTest)
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("should re-create pods and PVCs on other nodes than the deleted node", func() {
		By("waiting for node finalizer set")
		Eventually(func() error {
			var nodes corev1.NodeList
			err := getObjects(&nodes, "nodes", "-l=!node-role.kubernetes.io/control-plane")
			if err != nil {
				return err
			}
			for _, node := range nodes.Items {
				if !controllerutil.ContainsFinalizer(&node, topolvm.GetNodeFinalizer()) {
					return errors.New("topolvm finalizer is not attached")
				}
			}
			return nil
		}).Should(Succeed())

		statefulsetName := "test-sts"
		By("applying statefulset")
		statefulsetYAML := []byte(fmt.Sprintf(statefulSetTemplateYAML, statefulsetName, statefulsetName))
		_, err := kubectlWithInput(statefulsetYAML, "-n", nsNodeDeleteTest, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			var st appsv1.StatefulSet
			err := getObjects(&st, "statefulset", "-n", nsNodeDeleteTest, statefulsetName)
			if err != nil {
				return err
			}
			if st.Status.ReadyReplicas != 3 {
				return fmt.Errorf("statefulset replica is not 3: %d", st.Status.ReadyReplicas)
			}
			return nil
		}).Should(Succeed())

		By("getting a target pod")
		var targetPod *corev1.Pod
		targetNode := "topolvm-e2e-worker3"
		var pods corev1.PodList
		err = getObjects(&pods, "pods", "-n", nsNodeDeleteTest)
		Expect(err).ShouldNot(HaveOccurred())

		for _, pod := range pods.Items {
			if pod.Spec.NodeName == targetNode {
				pod := pod
				targetPod = &pod
				break
			}
		}
		Expect(targetPod).ShouldNot(BeNil())

		By("getting LVs to be cleaned up")
		var logicalVolumeList topolvmv1.LogicalVolumeList
		err = getObjects(&logicalVolumeList, "logicalvolumes")
		Expect(err).ShouldNot(HaveOccurred())

		var targetLVs []topolvmv1.LogicalVolume
		for _, lv := range logicalVolumeList.Items {
			if lv.Spec.NodeName == targetNode {
				targetLVs = append(targetLVs, lv)
			}
		}

		By("deleting Node " + targetNode)
		_, err = kubectl("delete", "node", targetNode, "--wait=true")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming a pod using a PVC is re-scheduled to another node")
		Eventually(func() error {
			var rescheduledPod corev1.Pod
			err = getObjects(&rescheduledPod, "pod", "-n", nsNodeDeleteTest, targetPod.Name)
			if err != nil {
				return fmt.Errorf("can not get target pod: err=%w", err)
			}
			if rescheduledPod.Spec.NodeName == "" || rescheduledPod.Spec.NodeName == targetNode {
				return fmt.Errorf("pod is not scheduled on other than %s", targetNode)
			}
			return nil
		}).Should(Succeed())

		By("confirming statefulset is ready")
		Eventually(func() error {
			var st appsv1.StatefulSet
			err := getObjects(&st, "statefulset", "-n", nsNodeDeleteTest, statefulsetName)
			if err != nil {
				return fmt.Errorf("failed to unmarshal")
			}
			if st.Status.ReadyReplicas != 3 {
				return fmt.Errorf("statefulset replica is not 3: %d", st.Status.ReadyReplicas)
			}
			return nil
		}).Should(Succeed())

		By("cleaning up LVMs that the deleted node had")
		_, err = execAtLocal("docker", nil, "stop", targetNode)
		Expect(err).ShouldNot(HaveOccurred())

		for _, lv := range targetLVs {
			_, err = execAtLocal("sudo", nil, "lvremove", "-y", "--select", fmt.Sprintf("lv_name=%s", lv.Status.VolumeID))
			Expect(err).ShouldNot(HaveOccurred())
		}
	})
}
