package e2e

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/controllers"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const cleanupTest = "cleanup-test"

//go:embed testdata/cleanup/statefulset-template.yaml
var statefulSetTemplateYAML string

func testCleanup() {

	BeforeEach(func() {
		// Skip because cleanup tests require multiple nodes but there is just one node in daemonset lvmd test environment.
		skipIfDaemonsetLvmd()
	})

	It("should create cleanup-test namespace", func() {
		createNamespace(cleanupTest)
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
		_, err := kubectlWithInput(statefulsetYAML, "-n", cleanupTest, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			var st appsv1.StatefulSet
			err := getObjects(&st, "statefulset", "-n", cleanupTest, statefulsetName)
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
		err = getObjects(&pods, "pods", "-n", cleanupTest)
		Expect(err).ShouldNot(HaveOccurred())

		for _, pod := range pods.Items {
			if pod.Spec.NodeName == targetNode {
				targetPod = &pod
				break
			}
		}
		Expect(targetPod).ShouldNot(BeNil())

		var pvcs corev1.PersistentVolumeClaimList
		err = getObjects(&pvcs, "pvc", "-n", cleanupTest)
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

		By("setting unschedule flag to Node topolvm-e2e-worker3")
		_, err = kubectl("cordon", targetNode)
		Expect(err).ShouldNot(HaveOccurred())

		By("deleting topolvm-node pod")
		err = getObjects(&pods, "pods", "-n", "topolvm-system")
		Expect(err).ShouldNot(HaveOccurred())

		var targetTopolvmNode string
		for _, pod := range pods.Items {
			if strings.HasPrefix(pod.Name, "topolvm-node-") && pod.Spec.NodeName == targetNode {
				targetTopolvmNode = pod.Name
				break
			}
		}
		Expect(targetTopolvmNode).ShouldNot(Equal(""), "cannot get topolmv-node name on topolvm-e2e-worker3")
		_, err = kubectl("-n", "topolvm-system", "delete", "pod", targetTopolvmNode)
		Expect(err).ShouldNot(HaveOccurred())

		By("deleting Node topolvm-e2e-worker3")
		_, err = kubectl("delete", "node", targetNode, "--wait=true")
		Expect(err).ShouldNot(HaveOccurred())

		// Confirming if the finalizer of the node resources works by checking by deleted pod's uid and pvc's uid if exist
		By("confirming pvc/pod are deleted and recreated if and only if node finalize is not skipped")
		Eventually(func() error {
			var pvcAfterNodeDelete corev1.PersistentVolumeClaim
			err := getObjects(&pvcAfterNodeDelete, "pvc", "-n", cleanupTest, targetPVC.Name)
			if err != nil {
				return fmt.Errorf("can not get target pvc: err=%w", err)
			}
			if pvcAfterNodeDelete.ObjectMeta.UID == targetPVC.ObjectMeta.UID {
				return fmt.Errorf("pvc is not deleted but finalizer is enabled. uid: %s", string(targetPVC.ObjectMeta.UID))
			}

			var rescheduledPod corev1.Pod
			err = getObjects(&rescheduledPod, "pod", "-n", cleanupTest, targetPod.Name)
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
			err := getObjects(&st, "statefulset", "-n", cleanupTest, statefulsetName)
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
		err = getObjects(&pvcAfterNodeDelete, "pvc", "-n", cleanupTest, targetPVC.Name)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(pvcAfterNodeDelete.ObjectMeta.UID).ShouldNot(Equal(targetPVC.ObjectMeta.UID))
	})

	It("should delete namespace", func() {
		_, err := kubectl("delete", "ns", cleanupTest)
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("should stop undeleted container in case that the container is undeleted", func() {
		_, err := execAtLocal(
			"docker", nil, "exec", "topolvm-e2e-worker3",
			"systemctl", "stop", "kubelet.service",
		)
		Expect(err).ShouldNot(HaveOccurred())

		stdout, err := execAtLocal(
			"docker", nil, "exec", "topolvm-e2e-worker3",
			"/usr/local/bin/crictl", "ps", "-o=json",
		)
		Expect(err).ShouldNot(HaveOccurred())

		type containerList struct {
			Containers []struct {
				ID       string `json:"id"`
				Metadata struct {
					Name string `json:"name"`
				}
			} `json:"containers"`
		}
		var l containerList
		err = json.Unmarshal(stdout, &l)
		Expect(err).ShouldNot(HaveOccurred(), "data=%s", stdout)

		for _, c := range l.Containers {
			_, err := execAtLocal(
				"docker", nil,
				"exec", "topolvm-e2e-worker3", "/usr/local/bin/crictl", "stop", c.ID,
			)
			Expect(err).ShouldNot(HaveOccurred())
			fmt.Printf("stop pause container with id=%s\n", c.ID)
		}
	})

	It("should cleanup volumes", func() {
		for _, lv := range targetLVs {
			_, err := execAtLocal("sudo", nil, "umount", "/dev/topolvm/"+lv.Status.VolumeID)
			Expect(err).ShouldNot(HaveOccurred())

			_, err = execAtLocal("sudo", nil, "lvremove", "-y", "--select", "lv_name="+lv.Status.VolumeID)
			Expect(err).ShouldNot(HaveOccurred())
		}
	})
}
