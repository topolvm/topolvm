package e2e

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	snapapi "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
)

var (
	//go:embed testdata/snapshot_restore/snapshot-template.yaml
	thinSnapshotTemplateYAML string

	//go:embed testdata/snapshot_restore/restore-pvc-template.yaml
	thinRestorePVCTemplateYAML string

	//go:embed testdata/snapshot_restore/restore-pod-template.yaml
	thinRestorePodTemplateYAML string
)

const (
	volName        = "thinvol"
	snapName       = "thinsnap"
	restorePVCName = "thinrestore"
	restorePodName = "thin-restore-pod"
	// size of PVC in GBs
	pvcSize = "1"
)

func testSnapRestore() {
	var nsSnapTest string
	var snapshot *snapapi.VolumeSnapshot

	BeforeEach(func() {
		nsSnapTest = "snap-test-" + randomString(10)
		createNamespace(nsSnapTest)
	})
	AfterEach(func() {
		if !CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
			kubectl("delete", "namespaces/"+nsSnapTest)
		}
	})

	It("should create a thin-snap", func() {
		By("deploying Pod with PVC")

		nodeName := "topolvm-e2e-worker"
		if isDaemonsetLvmdEnvSet() {
			nodeName = getDaemonsetLvmdNodeName()
		}
		var volumeName string
		thinPvcYAML := []byte(fmt.Sprintf(thinPVCTemplateYAML, volName, pvcSize))
		stdout, stderr, err := kubectlWithInput(thinPvcYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		thinPodYAML := []byte(fmt.Sprintf(thinPodTemplateYAML, "thinpod", volName, nodeName))
		stdout, stderr, err = kubectlWithInput(thinPodYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming if the resources have been created")
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "-n", nsSnapTest, "pvc", volName, "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var pvc corev1.PersistentVolumeClaim
			err = json.Unmarshal(stdout, &pvc)
			if err != nil {
				return fmt.Errorf("failed to unmarshal PVC. stdout: %s, err: %v", stdout, err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", volName)
			}
			return nil
		}).Should(Succeed())

		By("writing file under /test1")
		writePath := "/test1/bootstrap.log"
		Eventually(func() error {
			stdout, stderr, err = kubectl("exec", "-n", nsSnapTest, "thinpod", "--", "cp", "/var/log/bootstrap.log", writePath)
			return err
		}).Should(Succeed())

		stdout, stderr, err = kubectl("exec", "-n", nsSnapTest, "thinpod", "--", "sync")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectl("exec", "-n", nsSnapTest, "thinpod", "--", "cat", writePath)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).ShouldNot(BeEmpty())

		By("creating a snap")
		thinSnapshotYAML := []byte(fmt.Sprintf(thinSnapshotTemplateYAML, snapName, "topolvm-provisioner-thin", "thinvol"))
		stdout, stderr, err = kubectlWithInput(thinSnapshotYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "vs", snapName, "-n", nsSnapTest, "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get VolumeSnapshot. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			err = json.Unmarshal(stdout, &snapshot)
			if err != nil {
				return fmt.Errorf("failed to unmarshal Volumesnapshot. stdout: %s, err: %v", stdout, err)
			}
			if snapshot.Status == nil {
				return fmt.Errorf("waiting for snapshot status")
			}
			if *snapshot.Status.ReadyToUse != true {
				return fmt.Errorf("Snapshot is not Ready To Use")
			}
			return nil
		}).Should(Succeed())

		By("restoring the snap")
		thinPVCRestoreYAML := []byte(fmt.Sprintf(thinRestorePVCTemplateYAML, restorePVCName, pvcSize, snapName))
		stdout, stderr, err = kubectlWithInput(thinPVCRestoreYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		thinPVCRestorePodYAML := []byte(fmt.Sprintf(thinRestorePodTemplateYAML, restorePodName, restorePVCName, topolvm.GetTopologyNodeKey(), nodeName))
		stdout, stderr, err = kubectlWithInput(thinPVCRestorePodYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("verifying if the restored PVC is created")
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "-n", nsSnapTest, "pvc", restorePVCName, "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var pvc corev1.PersistentVolumeClaim
			err = json.Unmarshal(stdout, &pvc)
			if err != nil {
				return fmt.Errorf("failed to unmarshal PVC. stdout: %s, err: %v", stdout, err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", restorePVCName)
			}
			return nil
		}).Should(Succeed())

		Eventually(func() error {
			volumeName, err = getVolumeNameofPVC(restorePVCName, nsSnapTest)
			return err
		}).Should(Succeed())

		var lv *thinlvinfo
		Eventually(func() error {
			lv, err = getThinLVInfo(volumeName)
			return err
		}).Should(Succeed())

		vgName := "node1-myvg4"
		if isDaemonsetLvmdEnvSet() {
			vgName = "node-myvg5"
		}
		Expect(vgName).Should(Equal(lv.vgName))

		poolName := "pool0"
		Expect(poolName).Should(Equal(lv.poolName))

		By("confirming that the file exists")
		Eventually(func() error {
			stdout, stderr, err = kubectl("exec", "-n", nsSnapTest, restorePodName, "--", "cat", writePath)
			if err != nil {
				return fmt.Errorf("failed to cat. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			if len(strings.TrimSpace(string(stdout))) == 0 {
				return fmt.Errorf(writePath + " is empty")
			}
			return nil
		}).Should(Succeed())

	})

	It("validating if the restored PVCs are standalone", func() {
		By("deleting the source PVC")

		nodeName := "topolvm-e2e-worker"
		if isDaemonsetLvmdEnvSet() {
			nodeName = getDaemonsetLvmdNodeName()
		}

		var volumeName string
		By("creating a PVC and application")
		thinPvcYAML := []byte(fmt.Sprintf(thinPVCTemplateYAML, volName, pvcSize))
		stdout, stderr, err := kubectlWithInput(thinPvcYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		thinPodYAML := []byte(fmt.Sprintf(thinPodTemplateYAML, "thinpod", volName, nodeName))
		stdout, stderr, err = kubectlWithInput(thinPodYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		By("verifying if the PVC is created")
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "-n", nsSnapTest, "pvc", volName, "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var pvc corev1.PersistentVolumeClaim
			err = json.Unmarshal(stdout, &pvc)
			if err != nil {
				return fmt.Errorf("failed to unmarshal PVC. stdout: %s, err: %v", stdout, err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", volName)
			}
			return nil
		}).Should(Succeed())

		By("creating a snap of the PVC")
		thinSnapshotYAML := []byte(fmt.Sprintf(thinSnapshotTemplateYAML, snapName, "topolvm-provisioner-thin", "thinvol"))
		stdout, stderr, err = kubectlWithInput(thinSnapshotYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "vs", snapName, "-n", nsSnapTest, "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get VolumeSnapshot. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			err = json.Unmarshal(stdout, &snapshot)
			if err != nil {
				return fmt.Errorf("failed to unmarshal Volumesnapshot. stdout: %s, err: %v", stdout, err)
			}
			if snapshot.Status == nil {
				return fmt.Errorf("waiting for snapshot status")
			}
			if *snapshot.Status.ReadyToUse != true {
				return fmt.Errorf("Snapshot is not Ready To Use")
			}
			return nil
		}).Should(Succeed())

		By("restoring the snap")
		thinPVCRestoreYAML := []byte(fmt.Sprintf(thinRestorePVCTemplateYAML, restorePVCName, pvcSize, snapName))
		stdout, stderr, err = kubectlWithInput(thinPVCRestoreYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		thinPVCRestorePodYAML := []byte(fmt.Sprintf(thinRestorePodTemplateYAML, restorePodName, restorePVCName, topolvm.GetTopologyNodeKey(), nodeName))
		stdout, stderr, err = kubectlWithInput(thinPVCRestorePodYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("verifying if the restored PVC is created")
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "-n", nsSnapTest, "pvc", restorePVCName, "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var pvc corev1.PersistentVolumeClaim
			err = json.Unmarshal(stdout, &pvc)
			if err != nil {
				return fmt.Errorf("failed to unmarshal PVC. stdout: %s, err: %v", stdout, err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", restorePVCName)
			}
			return nil
		}).Should(Succeed())

		By("validating if the restored volume is present")
		Eventually(func() error {
			volumeName, err = getVolumeNameofPVC(restorePVCName, nsSnapTest)
			return err
		}).Should(Succeed())

		var lv *thinlvinfo
		Eventually(func() error {
			lv, err = getThinLVInfo(volumeName)
			return err
		}).Should(Succeed())

		vgName := "node1-myvg4"
		if isDaemonsetLvmdEnvSet() {
			vgName = "node-myvg5"
		}
		Expect(vgName).Should(Equal(lv.vgName))

		poolName := "pool0"
		Expect(poolName).Should(Equal(lv.poolName))

		// delete the source PVC as well as the snapshot
		By("deleting source volume and snap")
		stdout, stderr, err = kubectlWithInput(thinPodYAML, "delete", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		stdout, stderr, err = kubectlWithInput(thinPvcYAML, "delete", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		stdout, stderr, err = kubectlWithInput(thinSnapshotYAML, "delete", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("validating if the restored volume is present and is not deleted.")
		_, err = getVolumeNameofPVC(restorePVCName, nsSnapTest)
		Expect(err).Should(Succeed())

		_, err = getThinLVInfo(volumeName)
		Expect(err).Should(Succeed())
	})

}
