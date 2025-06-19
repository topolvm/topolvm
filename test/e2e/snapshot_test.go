package e2e

import (
	_ "embed"
	"fmt"
	"strings"

	snapapi "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	// size of PVC in MBs from the source volume
	pvcSizeBytes int64 = 1023 * 1024 * 1024 // 1023 MiB
	// size of PVC in MBs from the restored PVCs
	restorePVCBytes int64 = 2047 * 1024 * 1024 // 2047 MiB
)

func testSnapRestore() {
	const writePath = "/test1/bootstrap.log"

	var nsSnapTest string
	var snapshot snapapi.VolumeSnapshot

	BeforeEach(func() {
		nsSnapTest = "snap-test-" + randomString()
		createNamespace(nsSnapTest)
	})
	AfterEach(func() {
		if !CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
			_, err := kubectl("delete", "namespaces", nsSnapTest)
			Expect(err).ShouldNot(HaveOccurred())
		}
	})

	DescribeTable("should create a thin-snap with size equal to source", func(provisioner string) {
		By("deploying Pod with PVC")

		thinPvcYAML := []byte(fmt.Sprintf(thinPVCTemplateYAML, volName, pvcSizeBytes))
		_, err := kubectlWithInput(thinPvcYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		thinPodYAML := []byte(fmt.Sprintf(thinPodTemplateYAML, "thinpod", volName))
		_, err = kubectlWithInput(thinPodYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming if the resources have been created")
		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err := getObjects(&pvc, "pvc", "-n", nsSnapTest, volName)
			if err != nil {
				return fmt.Errorf("failed to get PVC. err: %w", err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", volName)
			}
			return nil
		}).Should(Succeed())

		By("writing file under /test1")
		Eventually(func() error {
			_, err = kubectl("exec", "-n", nsSnapTest, "thinpod", "--", "cp", "/var/log/bootstrap.log", writePath)
			return err
		}).Should(Succeed())

		_, err = kubectl("exec", "-n", nsSnapTest, "thinpod", "--", "sync")
		Expect(err).ShouldNot(HaveOccurred())
		stdout, err := kubectl("exec", "-n", nsSnapTest, "thinpod", "--", "cat", writePath)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(strings.TrimSpace(string(stdout))).ShouldNot(BeEmpty())

		By("creating a snap")
		thinSnapshotYAML := []byte(fmt.Sprintf(thinSnapshotTemplateYAML, snapName, provisioner, "thinvol"))
		_, err = kubectlWithInput(thinSnapshotYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			err := getObjects(&snapshot, "vs", snapName, "-n", nsSnapTest)
			if err != nil {
				return fmt.Errorf("failed to get VolumeSnapshot. err: %w", err)
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
		snapRestoreSize := snapshot.Status.RestoreSize.Value()
		thinPVCRestoreYAML := []byte(fmt.Sprintf(thinRestorePVCTemplateYAML, restorePVCName, snapRestoreSize, snapName))
		_, err = kubectlWithInput(thinPVCRestoreYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		thinPVCRestorePodYAML := []byte(fmt.Sprintf(thinRestorePodTemplateYAML, restorePodName, restorePVCName))
		_, err = kubectlWithInput(thinPVCRestorePodYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("verifying if the restored PVC is created")
		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err := getObjects(&pvc, "pvc", "-n", nsSnapTest, restorePVCName)
			if err != nil {
				return fmt.Errorf("failed to get PVC. err: %w", err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", restorePVCName)
			}
			return nil
		}).Should(Succeed())

		var lvName string
		Eventually(func() error {
			lvName, err = getLVNameOfPVC(restorePVCName, nsSnapTest)
			return err
		}).Should(Succeed())

		var lv *lvinfo
		Eventually(func() error {
			lv, err = getLVInfo(lvName)
			return err
		}).Should(Succeed())

		Expect(lv.vgName).Should(Equal("node1-thin1"))
		Expect(lv.poolName).Should(Equal("pool0"))

		By("confirming that the file exists")
		Eventually(func() error {
			stdout, err = kubectl("exec", "-n", nsSnapTest, restorePodName, "--", "cat", writePath)
			if err != nil {
				return fmt.Errorf("failed to cat. err: %w", err)
			}
			if len(strings.TrimSpace(string(stdout))) == 0 {
				return fmt.Errorf("%s is empty", writePath)
			}
			return nil
		}).Should(Succeed())
	},
		Entry("xfs", "topolvm-provisioner-thin"),
		Entry("btrfs", "topolvm-provisioner-thin-btrfs"),
	)

	DescribeTable("should create a thin-snap with size greater than source", func(provisioner string) {
		By("deploying Pod with PVC")
		thinPvcYAML := []byte(fmt.Sprintf(thinPVCTemplateYAML, volName, pvcSizeBytes))
		_, err := kubectlWithInput(thinPvcYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		thinPodYAML := []byte(fmt.Sprintf(thinPodTemplateYAML, "thinpod", volName))
		_, err = kubectlWithInput(thinPodYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming if the resources have been created")
		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err := getObjects(&pvc, "pvc", "-n", nsSnapTest, volName)
			if err != nil {
				return fmt.Errorf("failed to get PVC. err: %w", err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", volName)
			}
			return nil
		}).Should(Succeed())

		By("writing file under /test1")
		Eventually(func() error {
			_, err = kubectl("exec", "-n", nsSnapTest, "thinpod", "--", "cp", "/var/log/bootstrap.log", writePath)
			return err
		}).Should(Succeed())

		_, err = kubectl("exec", "-n", nsSnapTest, "thinpod", "--", "sync")
		Expect(err).ShouldNot(HaveOccurred())
		stdout, err := kubectl("exec", "-n", nsSnapTest, "thinpod", "--", "cat", writePath)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(strings.TrimSpace(string(stdout))).ShouldNot(BeEmpty())

		By("creating a snap")
		thinSnapshotYAML := []byte(fmt.Sprintf(thinSnapshotTemplateYAML, snapName, provisioner, "thinvol"))
		_, err = kubectlWithInput(thinSnapshotYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			err := getObjects(&snapshot, "vs", snapName, "-n", nsSnapTest)
			if err != nil {
				return fmt.Errorf("failed to get VolumeSnapshot. err: %w", err)
			}
			if snapshot.Status == nil {
				return fmt.Errorf("waiting for snapshot status")
			}
			if *snapshot.Status.ReadyToUse != true {
				return fmt.Errorf("snapshot is not Ready To Use")
			}
			return nil
		}).Should(Succeed())

		By("restoring the snap")
		thinPVCRestoreYAML := []byte(fmt.Sprintf(thinRestorePVCTemplateYAML, restorePVCName, restorePVCBytes, snapName))
		_, err = kubectlWithInput(thinPVCRestoreYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		thinPVCRestorePodYAML := []byte(fmt.Sprintf(thinRestorePodTemplateYAML, restorePodName, restorePVCName))
		_, err = kubectlWithInput(thinPVCRestorePodYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("verifying if the restored PVC is created with correct size")
		var restoredSize *resource.Quantity
		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err := getObjects(&pvc, "pvc", "-n", nsSnapTest, restorePVCName)
			if err != nil {
				return fmt.Errorf("failed to get PVC. err: %w", err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", restorePVCName)
			}

			restoredSize = pvc.Status.Capacity.Storage()
			if restoredSize.Cmp(*snapshot.Status.RestoreSize) < 0 {
				return fmt.Errorf("PVC is smaller than snapshot size: %v < %v", restoredSize, snapshot.Status.RestoreSize)
			}
			if restoredSize.CmpInt64(restorePVCBytes) <= 0 {
				return fmt.Errorf("PVC is smaller than unrounded restore size: %v < %v",
					restoredSize, resource.NewQuantity(restorePVCBytes, resource.BinarySI))
			}
			return nil
		}).Should(Succeed())

		var lvName string
		Eventually(func() error {
			lvName, err = getLVNameOfPVC(restorePVCName, nsSnapTest)
			return err
		}).Should(Succeed())

		var lv *lvinfo
		Eventually(func() error {
			lv, err = getLVInfo(lvName)
			return err
		}).Should(Succeed())

		By(fmt.Sprintf("using lv with size %v", lv.size))

		Expect(lv.vgName).Should(Equal("node1-thin1"))
		Expect(lv.poolName).Should(Equal("pool0"))

		By("confirming that the file exists")
		Eventually(func() error {
			stdout, err = kubectl("exec", "-n", nsSnapTest, restorePodName, "--", "cat", writePath)
			if err != nil {
				return fmt.Errorf("failed to cat. err: %w", err)
			}
			if len(strings.TrimSpace(string(stdout))) == 0 {
				return fmt.Errorf("%s is empty", writePath)
			}
			return nil
		}).Should(Succeed())

		By("confirming that the specified device is resized in the Pod")
		Eventually(func() error {
			// The sizes reported by `df` exclude filesystem overhead (e.g. reserved blocks), so they don't exactly match
			// LVM’s sizes. In this test, we'll round them to gigabytes using the `-BG` option for simplicity.
			stdout, err := kubectl("exec", "-n", nsSnapTest, restorePodName, "--", "df", "-BG", "--output=size", "/test1")
			if err != nil {
				return fmt.Errorf("failed to get volume size. err: %w", err)
			}
			volSize := resource.MustParse(strings.Fields(string(stdout))[1] + "i")
			if restoredSize.Cmp(volSize) != 0 {
				return fmt.Errorf("restored PVC size is wrong: %d, expected: %d", volSize.Value(), restoredSize.Value())
			}
			return nil
		}).Should(Succeed())
	},
		Entry("xfs", "topolvm-provisioner-thin"),
		Entry("btrfs", "topolvm-provisioner-thin-btrfs"),
	)

	DescribeTable("validating if the restored PVCs are standalone", func(provisioner string) {
		By("creating a PVC and application")
		thinPvcYAML := []byte(fmt.Sprintf(thinPVCTemplateYAML, volName, pvcSizeBytes))
		_, err := kubectlWithInput(thinPvcYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		thinPodYAML := []byte(fmt.Sprintf(thinPodTemplateYAML, "thinpod", volName))
		_, err = kubectlWithInput(thinPodYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		By("verifying if the PVC is created")
		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err := getObjects(&pvc, "pvc", "-n", nsSnapTest, volName)
			if err != nil {
				return fmt.Errorf("failed to get PVC. err: %w", err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", volName)
			}
			return nil
		}).Should(Succeed())

		By("creating a snap of the PVC")
		thinSnapshotYAML := []byte(fmt.Sprintf(thinSnapshotTemplateYAML, snapName, provisioner, "thinvol"))
		_, err = kubectlWithInput(thinSnapshotYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(func() error {
			err := getObjects(&snapshot, "vs", snapName, "-n", nsSnapTest)
			if err != nil {
				return fmt.Errorf("failed to get VolumeSnapshot. err: %w", err)
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
		snapRestoreSize := snapshot.Status.RestoreSize.Value()
		thinPVCRestoreYAML := []byte(fmt.Sprintf(thinRestorePVCTemplateYAML, restorePVCName, snapRestoreSize, snapName))
		_, err = kubectlWithInput(thinPVCRestoreYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		thinPVCRestorePodYAML := []byte(fmt.Sprintf(thinRestorePodTemplateYAML, restorePodName, restorePVCName))
		_, err = kubectlWithInput(thinPVCRestorePodYAML, "apply", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("verifying if the restored PVC is created")
		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err := getObjects(&pvc, "pvc", "-n", nsSnapTest, restorePVCName)
			if err != nil {
				return fmt.Errorf("failed to get PVC. err: %w", err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return fmt.Errorf("PVC %s is not bound", restorePVCName)
			}
			return nil
		}).Should(Succeed())

		By("validating if the restored volume is present")
		var lvName string
		Eventually(func() error {
			lvName, err = getLVNameOfPVC(restorePVCName, nsSnapTest)
			return err
		}).Should(Succeed())

		var lv *lvinfo
		Eventually(func() error {
			lv, err = getLVInfo(lvName)
			return err
		}).Should(Succeed())

		Expect(lv.vgName).Should(Equal("node1-thin1"))
		Expect(lv.poolName).Should(Equal("pool0"))

		// delete the source PVC as well as the snapshot
		By("deleting source volume and snap")
		_, err = kubectlWithInput(thinPodYAML, "delete", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		_, err = kubectlWithInput(thinPvcYAML, "delete", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		_, err = kubectlWithInput(thinSnapshotYAML, "delete", "-n", nsSnapTest, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("validating if the restored volume is present and is not deleted.")
		lvName, err = getLVNameOfPVC(restorePVCName, nsSnapTest)
		Expect(err).Should(Succeed())

		_, err = getLVInfo(lvName)
		Expect(err).Should(Succeed())
	},
		Entry("xfs", "topolvm-provisioner-thin"),
		Entry("btrfs", "topolvm-provisioner-thin-btrfs"),
	)
}
