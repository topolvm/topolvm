package e2e

import (
	_ "embed"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/controllers"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const nsCleanupTest = "cleanup-test"

//go:embed testdata/cleanup/statefulset-template.yaml
var statefulSetTemplateYAML string

func testCleanup() {

	BeforeEach(func() {
		// Skip because cleanup tests require multiple nodes but there is just one node in daemonset lvmd test environment.
		skipIfDaemonsetLvmd()

		createNamespace(nsCleanupTest)
	})

	AfterEach(func() {
		kubectl("delete", "namespaces/"+nsCleanupTest)
	})

	var targetLVs []topolvmv1.LogicalVolume

	It("should finalize the delete node if and only if the node finalize isn't skipped", func() {
		By("checking Node finalizer")
		Eventually(func() error {
			var nodes corev1.NodeList
			err := getObjects(&nodes, "nodes", "-l=node-role.kubernetes.io/control-plane!=")
			if err != nil {
				return err
			}
			for _, node := range nodes.Items {
				// Even if node finalize is skipped, the finalizer is still present on the node
				// The finalizer is added by the metrics-exporter runner from topolvm-node
				if !controllerutil.ContainsFinalizer(&node, topolvm.GetNodeFinalizer()) {
					return errors.New("topolvm finalizer is not attached")
				}
			}
			return nil
		}).Should(Succeed())

		statefulsetName := "test-sts"
		By("applying statefulset")
		statefulsetYAML := []byte(fmt.Sprintf(statefulSetTemplateYAML, statefulsetName, statefulsetName))
		_, err := kubectlWithInput(statefulsetYAML, "-n", nsCleanupTest, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			var st appsv1.StatefulSet
			err := getObjects(&st, "statefulset", "-n", nsCleanupTest, statefulsetName)
			if err != nil {
				return err
			}
			if st.Status.ReadyReplicas != 3 {
				return fmt.Errorf("statefulset replica is not 3: %d", st.Status.ReadyReplicas)
			}
			return nil
		}).Should(Succeed())

		// As pvc and pod are deleted in the finalizer of node resource, confirm the resources before deleted.
		By("getting target pvcs/pods")
		var targetPod *corev1.Pod
		targetNode := "topolvm-e2e-worker3"
		var pods corev1.PodList
		err = getObjects(&pods, "pods", "-n", nsCleanupTest)
		Expect(err).ShouldNot(HaveOccurred())

		for _, pod := range pods.Items {
			if pod.Spec.NodeName == targetNode {
				targetPod = &pod
				break
			}
		}
		Expect(targetPod).ShouldNot(BeNil())

		var pvcs corev1.PersistentVolumeClaimList
		err = getObjects(&pvcs, "pvc", "-n", nsCleanupTest)
		Expect(err).ShouldNot(HaveOccurred())

		var targetPVC *corev1.PersistentVolumeClaim
		for _, pvc := range pvcs.Items {
			if _, ok := pvc.Annotations[controllers.AnnSelectedNode]; !ok {
				continue
			}
			if pvc.Annotations[controllers.AnnSelectedNode] != targetNode {
				continue
			}
			targetPVC = &pvc
			break
		}
		Expect(targetPVC).ShouldNot(BeNil())

		var logicalVolumeList topolvmv1.LogicalVolumeList
		err = getObjects(&logicalVolumeList, "logicalvolumes")
		Expect(err).ShouldNot(HaveOccurred())

		for _, lv := range logicalVolumeList.Items {
			if lv.Spec.NodeName == targetNode {
				targetLVs = append(targetLVs, lv)
			}
		}

		By("deleting Node topolvm-e2e-worker3")
		_, err = kubectl("delete", "node", targetNode, "--wait=true")
		Expect(err).ShouldNot(HaveOccurred())

		// Confirming if the finalizer of the node resources works by checking by deleted pod's uid and pvc's uid if exist
		By("confirming pvc/pod are deleted and recreated if and only if node finalize is not skipped")
		Eventually(func() error {
			var pvcAfterNodeDelete corev1.PersistentVolumeClaim
			err := getObjects(&pvcAfterNodeDelete, "pvc", "-n", nsCleanupTest, targetPVC.Name)
			if err != nil {
				return fmt.Errorf("can not get target pvc: err=%w", err)
			}
			if pvcAfterNodeDelete.ObjectMeta.UID == targetPVC.ObjectMeta.UID {
				return fmt.Errorf("pvc is not deleted but finalizer is enabled. uid: %s", string(targetPVC.ObjectMeta.UID))
			}

			var rescheduledPod corev1.Pod
			err = getObjects(&rescheduledPod, "pod", "-n", nsCleanupTest, targetPod.Name)
			if err != nil {
				return fmt.Errorf("can not get target pod: err=%w", err)
			}
			if rescheduledPod.ObjectMeta.UID == targetPod.ObjectMeta.UID {
				return fmt.Errorf("pod is not deleted. uid: %s", string(targetPVC.ObjectMeta.UID))
			}
			return nil
		}).Should(Succeed())

		// The pods of statefulset would be recreated after they are deleted by the finalizer of node resource.
		// Though, because of the deletion timing of the pvcs and pods, the recreated pods can get pending status or running status.
		//  If they takes running status, delete them for rescheduling them
		By("confirming statefulset is ready")
		Eventually(func() error {
			var st appsv1.StatefulSet
			err := getObjects(&st, "statefulset", "-n", nsCleanupTest, statefulsetName)
			if err != nil {
				return fmt.Errorf("failed to unmarshal")
			}
			if st.Status.ReadyReplicas != 3 {
				return fmt.Errorf("statefulset replica is not 3: %d", st.Status.ReadyReplicas)
			}
			return nil
		}).Should(Succeed())

		By("confirming pvc is recreated if and only if the node finalizer is enabled")
		var pvcAfterNodeDelete corev1.PersistentVolumeClaim
		err = getObjects(&pvcAfterNodeDelete, "pvc", "-n", nsCleanupTest, targetPVC.Name)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(pvcAfterNodeDelete.ObjectMeta.UID).ShouldNot(Equal(targetPVC.ObjectMeta.UID))

		By("cleaning up LVMs that the deleted node had")
		_, err = execAtLocal("docker", nil, "stop", "topolvm-e2e-worker3")
		Expect(err).ShouldNot(HaveOccurred())

		for _, lv := range targetLVs {
			_, err = execAtLocal("sudo", nil, "lvremove", "-y", "--select", "lv_name="+lv.Status.VolumeID)
			Expect(err).ShouldNot(HaveOccurred())
		}
	})
}
