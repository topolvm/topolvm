package v2

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"os/exec"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func testE2E() {
	Describe("Basic I/O functionality", testBaseIO)
}

func testBaseIO() {
	testcases := []struct {
		filesystem  string
		deviceClass string
		provisioner string
		size        resource.Quantity
	}{
		{
			filesystem:  "xfs",
			deviceClass: "dc1",
			provisioner: "topolvm.io",
			size:        resource.MustParse("300Mi"),
		},
		{
			filesystem:  "ext4",
			deviceClass: "dc1",
			provisioner: "topolvm.io",
			size:        resource.MustParse("300Mi"),
		},
		{
			filesystem:  "btrfs",
			deviceClass: "dc1",
			provisioner: "topolvm.io",
			size:        resource.MustParse("300Mi"),
		},
	}
	for _, tc := range testcases {
		Context(fmt.Sprintf("filesystem: %s, deviceClass: %s, provisioner: %s, size: %s", tc.filesystem, tc.deviceClass, tc.provisioner, tc.size.String()), func() {
			testBaseIORun(tc.filesystem, tc.deviceClass, tc.provisioner, tc.size)
		})
	}
}

func testBaseIORun(filesystem string, deviceClass string, provisioner string, size resource.Quantity) {
	var ns *corev1.Namespace
	var sc *storagev1.StorageClass

	BeforeEach(func(ctx SpecContext) {
		ns = testNamespace()
		Expect(e2eclient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(ctx, func(ctx SpecContext) {
			Expect(e2eclient.Delete(ctx, ns)).To(Succeed())
		})
	})

	It("should be mounted in specified path", func(ctx SpecContext) {
		sc = templateStorageClass(
			ns.GetName(),
			provisioner,
			filesystem,
			deviceClass,
		)

		Expect(e2eclient.Create(ctx, sc)).To(Succeed())
		DeferCleanup(ctx, func(ctx SpecContext) {
			Expect(e2eclient.Delete(ctx, sc)).To(Succeed())
		})

		By("deploying Pod with PVC")
		pvc := templatePVCForStorageClass(ns, sc, corev1.PersistentVolumeFilesystem, size)
		pod := getPodConsumingPVC("pod", pvc)

		Expect(e2eclient.Create(ctx, pvc.DeepCopy())).To(Succeed())
		Expect(e2eclient.Create(ctx, pod.DeepCopy())).To(Succeed())

		verifyMountAndFileSystem := func() {
			mountLine, err := podrunner.GetProcMountLineForFirstVolumeMount(ctx, pod)
			Expect(err).NotTo(HaveOccurred())
			mountLineSeparated := strings.Fields(mountLine)
			Expect(len(mountLineSeparated) > 2).To(BeTrue())
			Expect(mountLineSeparated[2]).To(Equal(filesystem))
		}

		By("verifying the mount point and filesystem", verifyMountAndFileSystem)

		By("verifying I/O by writing a file", func() {
			data := randomString(100)
			Expect(podrunner.WriteDataInPod(ctx, pod, data, ContentModeFile)).To(Succeed())
			dataFromPod, err := podrunner.GetDataInPod(ctx, pod, ContentModeFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataFromPod).To(Equal(data))
		})

		By("recreating pod should not impact file content", func() {
			Expect(e2eclient.Delete(ctx, pod, &client.DeleteOptions{GracePeriodSeconds: ptr.To(int64(1))})).To(Succeed())
			Expect(e2eclient.Create(ctx, pod)).To(Succeed())
		})

		By("verifying the mount point and filesystem (after recreation)", verifyMountAndFileSystem)

		var pvname string
		By("verifying lvm2 lv based on LogicalVolume referenced in PVC", func() {
			pvc := pvc.DeepCopy()
			Expect(e2eclient.Get(ctx, client.ObjectKeyFromObject(pvc), pvc)).To(Succeed())
			Expect(pvc.Spec.VolumeName).NotTo(BeEmpty())
			pvname = pvc.Spec.VolumeName

			lv := &topolvmv1.LogicalVolume{}
			Expect(e2eclient.Get(ctx, client.ObjectKey{Name: pvc.Spec.VolumeName}, lv)).To(Succeed())
			Expect(lv.UID).NotTo(BeEmpty())

			_, err := getLVInfo(string(lv.UID))
			DeferCleanup(ctx, func(ctx SpecContext) {
				By("verifying lv no longer exists at end of test")
				_, err := getLVInfo(string(lv.UID))
				Expect(err).To(ContainSubstring("not found"))
			})

			Expect(err).NotTo(HaveOccurred(), "LV info from host for LogicalVolume must be present")
		})

		By("verifying PersistentVolume is deleted when PVC is deleted", func() {
			Expect(e2eclient.Delete(ctx, pod, &client.DeleteOptions{GracePeriodSeconds: ptr.To(int64(1))})).To(Succeed())
			Expect(e2eclient.Delete(ctx, pvc)).To(Succeed())
			pv := &corev1.PersistentVolume{}
			Expect(e2eclient.Get(ctx, client.ObjectKey{Name: pvname}, pv)).Should(Satisfy(k8serrors.IsNotFound))
		})
	})
}

func testNamespace() *corev1.Namespace {
	ns := &corev1.Namespace{}
	ns.Name = "e2e-test-" + randomString(10)
	return ns
}

func randomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxyz")

	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

func getPodConsumingPVC(name string, claim *corev1.PersistentVolumeClaim) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "ubuntu",
				Image:   "ubuntu:20.04",
				Command: []string{"sleep", "infinity"},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "data",
					MountPath: "/data",
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: claim.GetName(),
					},
				},
			}},
		},
	}
}

func templatePVCForStorageClass(
	namespace *corev1.Namespace,
	class *storagev1.StorageClass,
	volumeMode corev1.PersistentVolumeMode,
	storageRequest resource.Quantity,
) *corev1.PersistentVolumeClaim {
	return getPVC(
		namespace.GetName(),
		class.GetName(),
		class.GetName(),
		volumeMode,
		storageRequest,
	)
}

func getPVC(name, namespace, storageClass string, volumeMode corev1.PersistentVolumeMode, storageRequest resource.Quantity) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeMode: &volumeMode,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageRequest,
				},
			},
			StorageClassName: &storageClass,
		},
	}
}

func templateStorageClass(name string, provisioner, fstype, deviceClass string) *storagev1.StorageClass {
	return &storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Provisioner: provisioner,
		Parameters: map[string]string{
			"csi.storage.k8s.io/fstype": fstype,
			topolvm.GetDeviceClassKey(): deviceClass,
		},
		MountOptions:         nil,
		AllowVolumeExpansion: nil,
		VolumeBindingMode:    ptr.To(storagev1.VolumeBindingWaitForFirstConsumer),
	}
}

// execAtLocal executes cmd.
func execAtLocal(cmd string, input []byte, args ...string) (stdout []byte, err error) {
	var stdoutBuf, stderrBuf bytes.Buffer
	command := exec.Command(cmd, args...)
	command.Stdout = &stdoutBuf
	command.Stderr = &stderrBuf

	if len(input) != 0 {
		command.Stdin = bytes.NewReader(input)
	}

	err = command.Run()
	stdout = stdoutBuf.Bytes()
	stderr := stderrBuf.Bytes()
	if err != nil {
		err = fmt.Errorf("%s failed. stdout=%s, stderr=%s, err=%w", cmd, stdout, stderr, err)
	}
	return
}

type ErrLVNotFound struct {
	lvName string
}

func (e ErrLVNotFound) Error() string {
	return fmt.Sprintf("lv_name ( %s ) not found", e.lvName)
}

type lvinfo struct {
	size     int
	poolName string
	vgName   string
}

func getLVInfo(lvName string) (*lvinfo, error) {
	stdout, err := execAtLocal("sudo", nil,
		"lvs", "--noheadings", "-o", "lv_size,pool_lv,vg_name",
		"--units", "b", "--nosuffix", "--separator", ":",
		"--select", "lv_name="+lvName)
	if err != nil {
		return nil, err
	}
	output := strings.TrimSpace(string(stdout))
	if output == "" {
		return nil, ErrLVNotFound{lvName: lvName}
	}
	if strings.Contains(output, "\n") {
		return nil, errors.New("found multiple lvs")
	}
	items := strings.Split(output, ":")
	if len(items) != 3 {
		return nil, fmt.Errorf("invalid format: %s", output)
	}
	size, err := strconv.Atoi(items[0])
	if err != nil {
		return nil, err
	}
	return &lvinfo{
		size:     size,
		poolName: items[1],
		vgName:   items[2],
	}, nil
}
