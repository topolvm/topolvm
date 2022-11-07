package e2e

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/controllers"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
			stdout, stderr, err := kubectl("get", "nodes", "-l=node-role.kubernetes.io/control-plane!=", "-o=json")
			if err != nil {
				return fmt.Errorf("%v, stdout=%s, stderr=%s", err, stdout, stderr)
			}
			var nodes corev1.NodeList
			err = json.Unmarshal(stdout, &nodes)
			if err != nil {
				return err
			}
			for _, node := range nodes.Items {
				topolvmFinalize := false
				for _, fn := range node.Finalizers {
					if fn == topolvm.GetNodeFinalizer() {
						topolvmFinalize = true
					}
				}
				// Even if node finalize is skipped, the finalizer is still present on the node
				// The finalizer is added by the metrics-exporter runner from topolvm-node
				if !topolvmFinalize {
					return errors.New("topolvm finalizer is not attached")
				}
			}
			return nil
		}).Should(Succeed())

		statefulsetName := "test-sts"
		By("applying statefulset")
		statefulsetYAML := []byte(fmt.Sprintf(statefulSetTemplateYAML, statefulsetName, statefulsetName))
		stdout, stderr, err := kubectlWithInput(statefulsetYAML, "-n", cleanupTest, "apply", "-f", "-")
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

		// As pvc and pod are deleted in the finalizer of node resource, confirm the resources before deleted.
		By("getting target pvcs/pods")
		var targetPod *corev1.Pod
		targetNode := "topolvm-e2e-worker3"
		stdout, stderr, err = kubectl("-n", cleanupTest, "get", "pods", "-o=json")
		Expect(err).ShouldNot(HaveOccurred())
		var pods corev1.PodList
		err = json.Unmarshal(stdout, &pods)
		Expect(err).ShouldNot(HaveOccurred())

	Outer:
		for _, pod := range pods.Items {
			if pod.Spec.NodeName != targetNode {
				continue
			}
			for _, volume := range pod.Spec.Volumes {
				if volume.PersistentVolumeClaim == nil {
					continue
				}
				if strings.Contains(volume.PersistentVolumeClaim.ClaimName, "test-sts-pvc") {
					targetPod = &pod
					break Outer
				}
			}
		}
		Expect(targetPod).ShouldNot(BeNil())

		stdout, stderr, err = kubectl("-n", cleanupTest, "get", "pvc", "-o=json")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		var pvcs corev1.PersistentVolumeClaimList
		err = json.Unmarshal(stdout, &pvcs)
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

		stdout, stderr, err = kubectl("get", "logicalvolumes", "-o=json")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		var logicalVolumeList topolvmv1.LogicalVolumeList
		err = json.Unmarshal(stdout, &logicalVolumeList)
		Expect(err).ShouldNot(HaveOccurred())

		for _, lv := range logicalVolumeList.Items {
			if lv.Spec.NodeName == targetNode {
				targetLVs = append(targetLVs, lv)
			}
		}

		By("setting unschedule flag to Node topolvm-e2e-worker3")
		stdout, stderr, err = kubectl("cordon", targetNode)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("deleting topolvm-node pod")
		stdout, stderr, err = kubectl("-n", "topolvm-system", "get", "pods", "-o=json")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		err = json.Unmarshal(stdout, &pods)
		Expect(err).ShouldNot(HaveOccurred())

		var targetTopolvmNode string
		for _, pod := range pods.Items {
			if strings.HasPrefix(pod.Name, "topolvm-node-") && pod.Spec.NodeName == targetNode {
				targetTopolvmNode = pod.Name
				break
			}
		}
		Expect(targetTopolvmNode).ShouldNot(Equal(""), "cannot get topolmv-node name on topolvm-e2e-worker3")
		stdout, stderr, err = kubectl("-n", "topolvm-system", "delete", "pod", targetTopolvmNode)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("deleting Node topolvm-e2e-worker3")
		stdout, stderr, err = kubectl("delete", "node", targetNode, "--wait=true")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		// Confirming if the finalizer of the node resources works by checking by deleted pod's uid and pvc's uid if exist
		By("confirming pvc/pod are deleted and recreated if and only if node finalize is not skipped")
		Eventually(func() error {
			stdout, stderr, err = kubectl("-n", cleanupTest, "get", "pvc", targetPVC.Name, "-o=json")
			if err != nil {
				return fmt.Errorf("can not get target pvc: err=%v, stdout=%s, stderr=%s", err, stdout, stderr)
			}
			var pvcAfterNodeDelete corev1.PersistentVolumeClaim
			err = json.Unmarshal(stdout, &pvcAfterNodeDelete)
			if err != nil {
				return err
			}
			switch {
			case !isNodeFinalizeSkipped() && pvcAfterNodeDelete.ObjectMeta.UID == targetPVC.ObjectMeta.UID:
				return fmt.Errorf("pvc is not deleted but finalizer is enabled. uid: %s", string(targetPVC.ObjectMeta.UID))
			case isNodeFinalizeSkipped() && pvcAfterNodeDelete.ObjectMeta.UID != targetPVC.ObjectMeta.UID:
				return fmt.Errorf("pvc was deleted but finalizer is disabled. old uid: %s, new uid %s",
					string(targetPVC.ObjectMeta.UID),
					string(pvcAfterNodeDelete.ObjectMeta.UID))
			}

			stdout, stderr, err = kubectl("-n", cleanupTest, "get", "pod", targetPod.Name, "-o=json")
			if err != nil {
				return fmt.Errorf("can not get target pod: err=%v, stdout=%s, stderr=%s", err, stdout, stderr)
			}
			var rescheduledPod corev1.Pod
			err = json.Unmarshal(stdout, &rescheduledPod)
			if err != nil {
				return err
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
			if isNodeFinalizeSkipped() {
				// the stateful set will never be ready if the node finalize process is skipped
				// the stateful set will not be rescheduled due to volume node affinity conflict
				return nil
			}
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

		By("confirming pvc is recreated if and only if the node finalizer is enabled")
		stdout, stderr, err = kubectl("-n", cleanupTest, "get", "pvc", targetPVC.Name, "-o=json")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		var pvcAfterNodeDelete corev1.PersistentVolumeClaim
		err = json.Unmarshal(stdout, &pvcAfterNodeDelete)
		Expect(err).ShouldNot(HaveOccurred())
		if isNodeFinalizeSkipped() {
			Expect(pvcAfterNodeDelete.ObjectMeta.UID).Should(Equal(targetPVC.ObjectMeta.UID))
		} else {
			Expect(pvcAfterNodeDelete.ObjectMeta.UID).ShouldNot(Equal(targetPVC.ObjectMeta.UID))
		}
	})

	It("should clean up LogicalVolume resources connected to the deleted node", func() {
		if isNodeFinalizeSkipped() {
			Skip("logical volumes should not be deleted when node finalize is skipped")
		}
		By("confirming logicalvolumes are deleted")
		Eventually(func() error {
			for _, lv := range targetLVs {
				_, _, err := kubectl("get", "logicalvolumes", lv.Name)
				if err == nil {
					return fmt.Errorf("logicalvolume still exists: %s", lv.Name)
				}
			}
			return nil
		}).Should(Succeed())
	})

	It("should not clean up LogicalVolume resources connected to the deleted node when skipping node finalize", func() {
		if !isNodeFinalizeSkipped() {
			Skip("logical volumes should be deleted when node finalize is not skipped")
		}
		Eventually(func() error {
			for _, lv := range targetLVs {
				_, _, err := kubectl("get", "logicalvolumes", lv.Name)
				if err != nil {
					return fmt.Errorf("error getting logicalvolume (which should still exist): %s", lv.Name)
				}
			}
			return nil
		}).Should(Succeed())
	})

	It("should delete namespace", func() {
		stdout, stderr, err := kubectl("delete", "ns", cleanupTest)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	})

	It("should stop undeleted container in case that the container is undeleted", func() {
		stdout, stderr, err := execAtLocal(
			"docker", nil, "exec", "topolvm-e2e-worker3",
			"systemctl", "stop", "kubelet.service",
		)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		stdout, stderr, err = execAtLocal(
			"docker", nil, "exec", "topolvm-e2e-worker3",
			"/usr/local/bin/crictl", "ps", "-o=json",
		)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

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
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s", stdout)

		for _, c := range l.Containers {
			stdout, stderr, err = execAtLocal(
				"docker", nil,
				"exec", "topolvm-e2e-worker3", "/usr/local/bin/crictl", "stop", c.ID,
			)
			Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
			fmt.Printf("stop ubuntu container with id=%s\n", c.ID)
		}
	})

	It("should cleanup volumes", func() {
		for _, lv := range targetLVs {
			stdout, stderr, err := execAtLocal("sudo", nil, "umount", "/dev/topolvm/"+lv.Status.VolumeID)
			Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

			stdout, stderr, err = execAtLocal("sudo", nil, "lvremove", "-y", "--select", "lv_name="+lv.Status.VolumeID)
			Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		}
	})
}
