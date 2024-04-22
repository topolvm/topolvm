package e2e

import (
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/cmd/topolvm-controller/app"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

//go:embed testdata/e2e/pvc-template.yaml
var pvcTemplateYAML string

//go:embed testdata/e2e/pod-volume-mount-template.yaml
var podVolumeMountTemplateYAML string

//go:embed testdata/e2e/pod-volume-device-template.yaml
var podVolumeDeviceTemplateYAML string

//go:embed testdata/e2e/node-capacity-pvc.yaml
var nodeCapacityPVCYAML []byte

//go:embed testdata/e2e/node-capacity-pvc2.yaml
var nodeCapacityPVC2YAML []byte

//go:embed testdata/e2e/generic-ephemeral-volume.yaml
var e2eGenericEphemeralVolumeYAML []byte

func testE2E() {
	testNamespacePrefix := "e2etest-"
	var ns string
	var cc CleanupContext

	BeforeEach(func() {
		cc = commonBeforeEach()

		ns = testNamespacePrefix + randomString()
		createNamespace(ns)
	})

	AfterEach(func() {
		// When a test fails, I want to investigate the cause. So please don't remove the namespace!
		if !CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
			_, err := kubectl("delete", "namespaces/"+ns)
			Expect(err).ShouldNot(HaveOccurred())
		}

		commonAfterEach(cc)
	})

	DescribeTable("should be mounted in specified path",
		func(fs string, storageClass string) {
			By(fmt.Sprintf("deploying Pod with PVC based on StorageClass: %s", storageClass))
			claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 1024, storageClass))
			podYaml := []byte(fmt.Sprintf(podVolumeMountTemplateYAML, "ubuntu", "topo-pvc"))

			_, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())
			_, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			By("confirming that the specified device exists in the Pod")
			Eventually(func() error {
				_, err = kubectl("exec", "-n", ns, "ubuntu", "--", "mountpoint", "-d", "/test1")
				if err != nil {
					return fmt.Errorf("failed to check mount point. err: %w", err)
				}

				stdout, err := kubectl("exec", "-n", ns, "ubuntu", "grep", "/test1", "/proc/mounts")
				if err != nil {
					return err
				}
				fields := strings.Fields(string(stdout))
				if fields[2] != fs {
					return fmt.Errorf("/test1 is not %s", fs)
				}
				return nil
			}).Should(Succeed())

			By("writing file under /test1")
			writePath := "/test1/bootstrap.log"
			_, err = kubectl("exec", "-n", ns, "ubuntu", "--", "cp", "/var/log/bootstrap.log", writePath)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = kubectl("exec", "-n", ns, "ubuntu", "--", "sync")
			Expect(err).ShouldNot(HaveOccurred())
			stdout, err := kubectl("exec", "-n", ns, "ubuntu", "--", "cat", writePath)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(strings.TrimSpace(string(stdout))).ShouldNot(BeEmpty())

			By("deleting the Pod, then recreating it")
			_, err = kubectl("delete", "--now=true", "-n", ns, "pod/ubuntu")
			Expect(err).ShouldNot(HaveOccurred())
			_, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			By("confirming that the file exists")
			Eventually(func() error {
				stdout, err = kubectl("exec", "-n", ns, "ubuntu", "--", "cat", writePath)
				if err != nil {
					return fmt.Errorf("failed to cat. err: %w", err)
				}
				if len(strings.TrimSpace(string(stdout))) == 0 {
					return fmt.Errorf(writePath + " is empty")
				}
				return nil
			}).Should(Succeed())

			By("confirming that the lv correspond to LogicalVolume resource is registered in LVM")
			var pvc corev1.PersistentVolumeClaim
			err = getObjects(&pvc, "pvc", "-n", ns, "topo-pvc")
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(func() error {
				return checkLVIsRegisteredInLVM(pvc.Spec.VolumeName)
			}).Should(Succeed())

			By("deleting the Pod and PVC")
			_, err = kubectlWithInput(podYaml, "delete", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())
			_, err = kubectlWithInput(claimYAML, "delete", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			By("confirming that the PV is deleted")
			Eventually(func() error {
				var pv corev1.PersistentVolume
				err := getObjects(&pv, "pv", volName)
				switch {
				case errors.Is(err, ErrObjectNotFound):
					return nil
				case err != nil:
					return fmt.Errorf("failed to get pv/%s. err: %w", volName, err)
				default:
					return fmt.Errorf("target pv exists %s", volName)
				}
			}).Should(Succeed())

			By("confirming that the lv correspond to LogicalVolume is deleted")
			Eventually(func() error {
				return checkLVIsDeletedInLVM(volName)
			}).Should(Succeed())
		},
		Entry("xfs", "xfs", "topolvm-provisioner"),
		Entry("btrfs", "btrfs", "topolvm-provisioner-btrfs"),
	)

	DescribeTable("should be mounted in specified path but changed by the minimum allocation default",
		func(fs string, storageClass string, minimumAllocationDefault string) {
			By(fmt.Sprintf("deploying Pod with PVC based on StorageClass: %s", storageClass))
			claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 100, storageClass))
			podYaml := []byte(fmt.Sprintf(podVolumeMountTemplateYAML, "ubuntu", "topo-pvc"))

			_, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())
			_, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			By("confirming that the specified device exists in the Pod")
			Eventually(func() error {
				_, err = kubectl("exec", "-n", ns, "ubuntu", "--", "mountpoint", "-d", "/test1")
				if err != nil {
					return fmt.Errorf("failed to check mount point. err: %w", err)
				}

				stdout, err := kubectl("exec", "-n", ns, "ubuntu", "grep", "/test1", "/proc/mounts")
				if err != nil {
					return err
				}
				fields := strings.Fields(string(stdout))
				if fields[2] != fs {
					return fmt.Errorf("/test1 is not %s", fs)
				}
				return nil
			}).Should(Succeed())

			By("writing file under /test1")
			writePath := "/test1/bootstrap.log"
			_, err = kubectl("exec", "-n", ns, "ubuntu", "--", "cp", "/var/log/bootstrap.log", writePath)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = kubectl("exec", "-n", ns, "ubuntu", "--", "sync")
			Expect(err).ShouldNot(HaveOccurred())
			stdout, err := kubectl("exec", "-n", ns, "ubuntu", "--", "cat", writePath)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(strings.TrimSpace(string(stdout))).ShouldNot(BeEmpty())

			By("deleting the Pod, then recreating it")
			_, err = kubectl("delete", "--now=true", "-n", ns, "pod/ubuntu")
			Expect(err).ShouldNot(HaveOccurred())
			_, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			By("confirming that the file exists")
			Eventually(func() error {
				stdout, err = kubectl("exec", "-n", ns, "ubuntu", "--", "cat", writePath)
				if err != nil {
					return fmt.Errorf("failed to cat. err: %w", err)
				}
				if len(strings.TrimSpace(string(stdout))) == 0 {
					return fmt.Errorf(writePath + " is empty")
				}
				return nil
			}).Should(Succeed())

			By("confirming that the lv correspond to LogicalVolume resource is registered in LVM")
			var pvc corev1.PersistentVolumeClaim
			err = getObjects(&pvc, "pvc", "-n", ns, "topo-pvc")
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(func() error {
				return checkLVIsRegisteredInLVM(pvc.Spec.VolumeName)
			}).Should(Succeed())

			By("confirming that the lv was successfully sized to the minimum size of the StorageClass")
			Expect(pvc.Status.Capacity.Storage().Cmp(resource.MustParse(minimumAllocationDefault))).
				Should(Equal(0),
					"expected: %s as minimum capacity, actual: %s",
					minimumAllocationDefault,
					pvc.Status.Capacity.Storage().String(),
				)

			By("deleting the Pod and PVC")
			_, err = kubectlWithInput(podYaml, "delete", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())
			_, err = kubectlWithInput(claimYAML, "delete", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			By("confirming that the PV is deleted")
			Eventually(func() error {
				var pv corev1.PersistentVolume
				err := getObjects(&pv, "pv", volName)
				switch {
				case errors.Is(err, ErrObjectNotFound):
					return nil
				case err != nil:
					return fmt.Errorf("failed to get pv/%s. err: %w", volName, err)
				default:
					return fmt.Errorf("target pv exists %s", volName)
				}
			}).Should(Succeed())

			By("confirming that the lv correspond to LogicalVolume is deleted")
			Eventually(func() error {
				return checkLVIsDeletedInLVM(volName)
			}).Should(Succeed())
		},
		Entry("xfs", "xfs", "topolvm-provisioner", app.DefaultMinimumAllocationSizeXFS),
		Entry("btrfs", "btrfs", "topolvm-provisioner-btrfs", app.DefaultMinimumAllocationSizeBtrfs),
	)

	It("should use generic ephemeral volumes", func() {
		By("deploying a Pod with a generic ephemeral volume")
		_, err := kubectlWithInput(e2eGenericEphemeralVolumeYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming the Pod is deployed")
		Eventually(func() error {
			var pod corev1.Pod
			err := getObjects(&pod, "pod", "-n", ns, "pause")
			if err != nil {
				return fmt.Errorf("failed to get Pod. err: %w", err)
			}
			if pod.Status.Phase != corev1.PodRunning {
				return errors.New("Pod is not running")
			}
			return nil
		}).Should(Succeed())

		By("confirming the PVC is bound")
		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err := getObjects(&pvc, "pvc", "-n", ns, "pause-generic-ephemeral-volume1")
			if err != nil {
				return fmt.Errorf("failed to get PVC. err: %w", err)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				return errors.New("PVC is not bound")
			}
			return nil
		}).Should(Succeed())

		By("deleting the Pod with a generic ephemeral volume")
		_, err = kubectlWithInput(e2eGenericEphemeralVolumeYAML, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming the Pod is deleted")
		Eventually(func() error {
			var pod corev1.Pod
			err := getObjects(&pod, "pod", "-n", ns, "pause")
			switch {
			case err == ErrObjectNotFound:
				return nil
			case err != nil:
				return err
			default:
				return errors.New("the Pod exists")
			}
		}).Should(Succeed())

		By("confirming the PVC is deleted")
		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err := getObjects(&pvc, "pvc", "-n", ns, "pause-generic-ephemeral-volume1")
			switch {
			case err == ErrObjectNotFound:
				return nil
			case err != nil:
				return err
			default:
				return errors.New("the PVC exists")
			}
		}).Should(Succeed())
	})

	It("should create a block device for Pod", func() {
		deviceFile := "/dev/e2etest"

		By("deploying ubuntu Pod with PVC to mount a block device")
		podYAML := []byte(fmt.Sprintf(podVolumeDeviceTemplateYAML, deviceFile))

		claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Block", 1024, "topolvm-provisioner"))

		_, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectlWithInput(podYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming that a block device exists in ubuntu pod")
		Eventually(func() error {
			_, err = kubectl("exec", "-n", ns, "ubuntu", "--", "test", "-b", deviceFile)
			if err != nil {
				podinfo, _ := kubectl("-n", ns, "describe", "pod", "ubuntu")
				return fmt.Errorf("failed to test. ubuntu pod info output: %s; err: %w", podinfo, err)
			}
			return nil
		}).Should(Succeed())

		By("writing data to a block device")
		// /etc/hostname contains "ubuntu"
		_, err = kubectl("exec", "-n", ns, "ubuntu", "--", "dd", "if=/etc/hostname", "of="+deviceFile)
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectl("exec", "-n", ns, "ubuntu", "--", "sync")
		Expect(err).ShouldNot(HaveOccurred())
		stdout, err := kubectl("exec", "-n", ns, "ubuntu", "--",
			"dd", "if="+deviceFile, "of=/dev/stdout", "bs=6", "count=1", "status=none")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(stdout)).Should(Equal("ubuntu"))

		By("deleting the Pod, then recreating it")
		_, err = kubectl("delete", "--now=true", "-n", ns, "pod/ubuntu")
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectlWithInput(podYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("reading data from a block device")
		Eventually(func() error {
			stdout, err = kubectl("exec", "-n", ns, "ubuntu", "--",
				"dd", "if="+deviceFile, "of=/dev/stdout", "bs=6", "count=1", "status=none")
			if err != nil {
				return fmt.Errorf("failed to cat. err: %w", err)
			}
			if string(stdout) != "ubuntu" {
				return fmt.Errorf("expected: ubuntu, actual: %s", string(stdout))
			}
			return nil
		}).Should(Succeed())

		By("confirming that the lv correspond to LogicalVolume resource is registered in LVM")
		var pvc corev1.PersistentVolumeClaim
		err = getObjects(&pvc, "pvc", "-n", ns, "topo-pvc")
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(func() error {
			return checkLVIsRegisteredInLVM(pvc.Spec.VolumeName)
		}).Should(Succeed())

		By("deleting the Pod and PVC")
		_, err = kubectlWithInput(podYAML, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectlWithInput(claimYAML, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming that the PV is deleted")
		Eventually(func() error {
			var pv corev1.PersistentVolume
			err := getObjects(&pv, "pv", volName)
			switch {
			case err == ErrObjectNotFound:
				return nil
			case err != nil:
				return fmt.Errorf("failed to get pv/%s. err: %w", volName, err)
			default:
				return fmt.Errorf("target PV exists %s", volName)
			}
		}).Should(Succeed())

		By("confirming that the lv correspond to LogicalVolume is deleted")
		Eventually(func() error {
			return checkLVIsDeletedInLVM(volName)
		}).Should(Succeed())
	})

	It("should create a block device for Pod but changed by the minimum allocation default for block storage", func() {
		deviceFile := "/dev/e2etest"

		By("deploying ubuntu Pod with PVC to mount a block device")
		podYAML := []byte(fmt.Sprintf(podVolumeDeviceTemplateYAML, deviceFile))

		claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Block", 1, "topolvm-provisioner"))

		_, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectlWithInput(podYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming that a block device exists in ubuntu pod")
		Eventually(func() error {
			_, err = kubectl("exec", "-n", ns, "ubuntu", "--", "test", "-b", deviceFile)
			if err != nil {
				podinfo, _ := kubectl("-n", ns, "describe", "pod", "ubuntu")
				return fmt.Errorf("failed to test. ubuntu pod info output: %s; err: %w", podinfo, err)
			}
			return nil
		}).Should(Succeed())

		By("writing data to a block device")
		// /etc/hostname contains "ubuntu"
		_, err = kubectl("exec", "-n", ns, "ubuntu", "--", "dd", "if=/etc/hostname", "of="+deviceFile)
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectl("exec", "-n", ns, "ubuntu", "--", "sync")
		Expect(err).ShouldNot(HaveOccurred())
		stdout, err := kubectl("exec", "-n", ns, "ubuntu", "--",
			"dd", "if="+deviceFile, "of=/dev/stdout", "bs=6", "count=1", "status=none")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(stdout)).Should(Equal("ubuntu"))

		By("deleting the Pod, then recreating it")
		_, err = kubectl("delete", "--now=true", "-n", ns, "pod/ubuntu")
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectlWithInput(podYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("reading data from a block device")
		Eventually(func() error {
			stdout, err = kubectl("exec", "-n", ns, "ubuntu", "--",
				"dd", "if="+deviceFile, "of=/dev/stdout", "bs=6", "count=1", "status=none")
			if err != nil {
				return fmt.Errorf("failed to cat. err: %w", err)
			}
			if string(stdout) != "ubuntu" {
				return fmt.Errorf("expected: ubuntu, actual: %s", string(stdout))
			}
			return nil
		}).Should(Succeed())

		By("confirming that the lv correspond to LogicalVolume resource is registered in LVM")
		var pvc corev1.PersistentVolumeClaim
		err = getObjects(&pvc, "pvc", "-n", ns, "topo-pvc")
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(func() error {
			return checkLVIsRegisteredInLVM(pvc.Spec.VolumeName)
		}).Should(Succeed())

		By("confirming that the lv was successfully sized to the minimum size of the StorageClass")
		Expect(pvc.Status.Capacity.Storage().Cmp(resource.MustParse(app.DefaultMinimumAllocationSizeBlock))).
			Should(Equal(0),
				"expected: %s as minimum capacity, actual: %s",
				app.DefaultMinimumAllocationSizeBlock, pvc.Status.Capacity.Storage().String())

		By("deleting the Pod and PVC")
		_, err = kubectlWithInput(podYAML, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectlWithInput(claimYAML, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming that the PV is deleted")
		Eventually(func() error {
			var pv corev1.PersistentVolume
			err := getObjects(&pv, "pv", volName)
			switch {
			case err == ErrObjectNotFound:
				return nil
			case err != nil:
				return fmt.Errorf("failed to get pv/%s. err: %w", volName, err)
			default:
				return fmt.Errorf("target PV exists %s", volName)
			}
		}).Should(Succeed())

		By("confirming that the lv correspond to LogicalVolume is deleted")
		Eventually(func() error {
			return checkLVIsDeletedInLVM(volName)
		}).Should(Succeed())
	})

	It("should choose a node with the largest capacity when volumeBindingMode == Immediate is specified", func() {
		skipIfStorageCapacity(
			"Storage Capacity Tracking doesn't check Storage Capacity when volumeBindingMode == Immediate is specified",
		)

		// Repeat applying a PVC to make sure that the volume is created on the node with the largest capacity in each loop.
		for i := 0; i < nonControlPlaneNodeCount; i++ {
			By("getting the node with max capacity (loop: " + strconv.Itoa(i) + ")")
			var maxCapNodes []string
			Eventually(func() error {
				var maxCapacity int
				maxCapNodes = []string{}
				var nodes corev1.NodeList
				err := getObjects(&nodes, "nodes")
				if err != nil {
					return fmt.Errorf("kubectl get nodes error: %w", err)
				}
				for _, node := range nodes.Items {
					if node.Name == "topolvm-e2e-control-plane" {
						continue
					}
					strCap, ok := node.Annotations[topolvm.GetCapacityKeyPrefix()+"dc1"]
					if !ok {
						return fmt.Errorf("capacity is not annotated: %s", node.Name)
					}
					capacity, err := strconv.Atoi(strCap)
					if err != nil {
						return err
					}
					fmt.Printf("%s: %d bytes\n", node.Name, capacity)
					switch {
					case capacity > maxCapacity:
						maxCapacity = capacity
						maxCapNodes = []string{node.GetName()}
					case capacity == maxCapacity:
						maxCapNodes = append(maxCapNodes, node.GetName())
					}
				}
				if len(maxCapNodes) != nonControlPlaneNodeCount-i {
					return fmt.Errorf("unexpected number of maxCapNodes: expected: %d, actual: %d",
						nonControlPlaneNodeCount-i, len(maxCapNodes))
				}
				return nil
			}).Should(Succeed())

			By("creating pvc")
			claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML,
				fmt.Sprintf("topo-pvc-%d", i), "Filesystem", 1024, "topolvm-provisioner-immediate"))
			_, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			var volumeName string
			Eventually(func() error {
				var pvc corev1.PersistentVolumeClaim
				err := getObjects(&pvc, "pvc", "-n", ns, "topo-pvc-"+strconv.Itoa(i))
				if err != nil {
					return fmt.Errorf("failed to get PVC. err: %w", err)
				}

				if pvc.Spec.VolumeName == "" {
					return errors.New("pvc.Spec.VolumeName should not be empty")
				}

				volumeName = pvc.Spec.VolumeName
				return nil
			}).Should(Succeed())

			By("confirming that the logical volume was scheduled onto the node with max capacity")
			var lv topolvmv1.LogicalVolume
			err = getObjects(&lv, "logicalvolumes", volumeName)
			Expect(err).ShouldNot(HaveOccurred())

			target := lv.Spec.NodeName
			Expect(target).To(BeElementOf(maxCapNodes), "maxCapNodes: %v, target: %s", maxCapNodes, target)
		}
	})

	It("should scheduled onto the correct node where PV exists (volumeBindingMode == Immediate)", func() {
		By("creating pvc")
		claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 1024, "topolvm-provisioner-immediate"))
		_, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		var volumeName string
		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err := getObjects(&pvc, "pvc", "-n", ns, "topo-pvc")
			if err != nil {
				return fmt.Errorf("failed to get PVC. err: %w", err)
			}

			if pvc.Spec.VolumeName == "" {
				return errors.New("pvc.Spec.VolumeName should not be empty")
			}

			volumeName = pvc.Spec.VolumeName
			return nil
		}).Should(Succeed())

		By("getting node name of which volume is created")
		var lv topolvmv1.LogicalVolume
		err = getObjects(&lv, "logicalvolumes", volumeName)
		Expect(err).ShouldNot(HaveOccurred())

		nodeName := lv.Spec.NodeName

		By("deploying ubuntu Pod with PVC")
		podYaml := []byte(fmt.Sprintf(podVolumeMountTemplateYAML, "ubuntu", "topo-pvc"))
		_, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming that ubuntu pod is scheduled onto " + nodeName)
		Eventually(func() error {
			var pod corev1.Pod
			err := getObjects(&pod, "pod", "-n", ns, "ubuntu")
			if err != nil {
				return fmt.Errorf("failed to create pod. err: %w", err)
			}

			if pod.Spec.NodeName != nodeName {
				return fmt.Errorf("pod is not yet scheduled")
			}

			return nil
		}).Should(Succeed())

		By("deleting the Pod, then recreating it")
		_, err = kubectl("delete", "--now=true", "-n", ns, "pod/ubuntu")
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming that ubuntu pod is rescheduled onto " + nodeName)
		Eventually(func() error {
			var pod corev1.Pod
			err := getObjects(&pod, "pod", "-n", ns, "ubuntu")
			if err != nil {
				return fmt.Errorf("failed to create pod. err: %w", err)
			}

			if pod.Spec.NodeName != nodeName {
				return fmt.Errorf("pod is not yet scheduled")
			}

			return nil
		}).Should(Succeed())
	})

	It("should schedule pods and volumes according to topolvm-scheduler", func() {
		skipIfStorageCapacity()
		skipIfSingleNode()

		/*
			Check the operation of topolvm-scheduler in multi-node(3-node) environment.
			As preparation, set the capacity of each node as follows.
			- node1: 18 / 18 GiB (targetNode)
			- node2:  4 / 18 GiB
			- node3:  4 / 18 GiB

			# 1st case: test for `prioritize`
			Try to create 8GiB PVC. Then
			- node1: 18 / 18 GiB -> `prioritize` 4 -> selected
			- node2:  4 / 18 GiB -> `prioritize` 2
			- node3:  4 / 18 GiB -> `prioritize` 2

			# 2nd case: test for `predicate` (1)
			Try to create 6GiB PVC. Then
			- node1: 10 / 18 GiB -> selected
			- node2:  4 / 18 GiB -> filtered (insufficient capacity)
			- node3:  4 / 18 GiB -> filtered (insufficient capacity)

			# 3rd case: test for `predicate` (2)
			Try to create 8GiB PVC. Then it cause error.
			- node1:  4 / 18 GiB -> filtered (insufficient capacity)
			- node2:  4 / 18 GiB -> filtered (insufficient capacity)
			- node3:  4 / 18 GiB -> filtered (insufficient capacity)
		*/
		By("initializing node capacity")
		_, err := kubectlWithInput(nodeCapacityPVCYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			var pvcList corev1.PersistentVolumeClaimList
			err := getObjects(&pvcList, "pvc", "-n", ns)
			if err != nil {
				return fmt.Errorf("failed to get PVC. err: %w", err)
			}

			if len(pvcList.Items) != 2 {
				return fmt.Errorf("the length of PVC list should be 2")
			}

			for _, pvc := range pvcList.Items {
				if pvc.Spec.VolumeName == "" {
					return errors.New("pvc.Spec.VolumeName should not be empty")
				}
			}
			return nil
		}).Should(Succeed())

		By("selecting a targetNode")
		var nodeList corev1.NodeList
		err = getObjects(&nodeList, "node")
		Expect(err).ShouldNot(HaveOccurred())

		var targetNode string
		var maxCapacity int
		for _, node := range nodeList.Items {
			if node.Name == "topolvm-e2e-control-plane" {
				continue
			}

			strCap, ok := node.Annotations[topolvm.GetCapacityKeyPrefix()+"dc1"]
			Expect(ok).To(Equal(true), "capacity is not annotated: "+node.Name)
			capacity, err := strconv.Atoi(strCap)
			Expect(err).ShouldNot(HaveOccurred())

			fmt.Printf("%s: %d\n", node.Name, capacity)
			if capacity > maxCapacity {
				maxCapacity = capacity
				targetNode = node.Name
			}
		}

		By("creating pvc")
		_, err = kubectlWithInput(nodeCapacityPVC2YAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		var boundNode string
		By("confirming that claiming 8GB pv to the targetNode is successful")
		podYaml := []byte(fmt.Sprintf(podVolumeMountTemplateYAML, "ubuntu1", "topo-pvc1"))
		_, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(func() error {
			boundNode, err = waitCreatingPodWithPVC("ubuntu1", ns)
			return err
		}).Should(Succeed())
		Expect(boundNode).To(Equal(targetNode), "bound: %s, target: %s", boundNode, targetNode)

		By("confirming that claiming 6GB pv to the targetNode is successful")
		podYaml = []byte(fmt.Sprintf(podVolumeMountTemplateYAML, "ubuntu2", "topo-pvc2"))
		_, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(func() error {
			boundNode, err = waitCreatingPodWithPVC("ubuntu2", ns)
			return err
		}).Should(Succeed())
		Expect(boundNode).To(Equal(targetNode), "bound: %s, target: %s", boundNode, targetNode)

		By("confirming that claiming 8GB pv to the targetNode is unsuccessful")
		podYaml = []byte(fmt.Sprintf(podVolumeMountTemplateYAML, "ubuntu3", "topo-pvc3"))
		_, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		time.Sleep(15 * time.Second)

		var pod corev1.Pod
		err = getObjects(&pod, "pod", "-n", ns, "ubuntu3")
		Expect(pod.Spec.NodeName).To(Equal(""))
	})

	DescribeTable("should resize filesystem",
		// expectedSizes is the expected size of the volume after a resize.
		// there are 3 steps of resizing: online, offline, and after deleting pods.
		// thus the expected sizes should be 3 elements.
		func(storageClass string, expectedSizes []int) {
			By(fmt.Sprintf("deploying Pod with PVC based on StorageClass: %s", storageClass))
			claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 1024, storageClass))
			podYaml := []byte(fmt.Sprintf(podVolumeMountTemplateYAML, "ubuntu", "topo-pvc"))

			_, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())
			_, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			By("confirming that the specified device is mounted in the Pod")
			Eventually(func() error {
				return verifyMountExists(ns, "ubuntu", "/test1")
			}).Should(Succeed())

			By("resizing PVC online")
			claimYAML = []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 2*1024, storageClass))
			_, err = kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			By("confirming that the specified device is resized in the Pod")
			timeout := time.Minute * 5
			Eventually(func() error {
				stdout, err := kubectl("exec", "-n", ns, "ubuntu", "--", "df", "--output=size", "/test1")
				if err != nil {
					return fmt.Errorf("failed to get volume size. err: %w", err)
				}
				dfFields := strings.Fields(string(stdout))
				volSize, err := strconv.Atoi(dfFields[1])
				if err != nil {
					return fmt.Errorf("failed to convert volume size string. data: %s, err: %w", stdout, err)
				}
				if volSize != expectedSizes[0] {
					return fmt.Errorf("failed to match volume size. actual: %d, expected: %d", volSize, expectedSizes[0])
				}
				return nil
			}, timeout).Should(Succeed())

			By("deleting Pod for offline resizing")
			_, err = kubectlWithInput(podYaml, "delete", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			By("resizing PVC offline")
			claimYAML = []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 3*1024, storageClass))
			_, err = kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			By("deploying Pod")
			_, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			By("confirming that the specified device is resized in the Pod")
			Eventually(func() error {
				stdout, err := kubectl("exec", "-n", ns, "ubuntu", "--", "df", "--output=size", "/test1")
				if err != nil {
					return fmt.Errorf("failed to get volume size. err: %w", err)
				}
				dfFields := strings.Fields(string(stdout))
				volSize, err := strconv.Atoi(dfFields[1])
				if err != nil {
					return fmt.Errorf("failed to convert volume size string. data: %s, err: %w", stdout, err)
				}
				if volSize != expectedSizes[1] {
					return fmt.Errorf("failed to match volume size. actual: %d, expected: %d", volSize, expectedSizes[1])
				}
				return nil
			}, timeout).Should(Succeed())

			By("deleting topolvm-node Pods to clear /dev/topolvm/*")
			_, err = kubectl("delete", "-n", ns, "pod", "-l=app.kubernetes.io/component=node,app.kubernetes.io/name=topolvm")
			Expect(err).ShouldNot(HaveOccurred())

			By("resizing PVC")
			claimYAML = []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 4*1024, storageClass))
			_, err = kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			By("confirming that the specified device is resized in the Pod")
			Eventually(func() error {
				stdout, err := kubectl("exec", "-n", ns, "ubuntu", "--", "df", "--output=size", "/test1")
				if err != nil {
					return fmt.Errorf("failed to get volume size. err: %w", err)
				}
				dfFields := strings.Fields(string(stdout))
				volSize, err := strconv.Atoi(dfFields[1])
				if err != nil {
					return fmt.Errorf("failed to convert volume size string. data: %s, err: %w", stdout, err)
				}
				if volSize != expectedSizes[2] {
					return fmt.Errorf("failed to match volume size. actual: %d, expected: %d", volSize, expectedSizes[2])
				}
				return nil
			}, timeout).Should(Succeed())

			By("confirming that no failure event has occurred")
			fieldSelector := "involvedObject.kind=PersistentVolumeClaim," +
				"involvedObject.name=topo-pvc," +
				"reason=VolumeResizeFailed"
			var events corev1.EventList
			err = getObjects(&events, "events", "-n", ns, "--field-selector="+fieldSelector)
			Expect(err).To(BeEquivalentTo(ErrObjectNotFound))

			By("resizing PVC over vg capacity")
			claimYAML = []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 100*1024, storageClass))
			_, err = kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			By("confirming that a failure event occurs")
			Eventually(func() error {
				var events corev1.EventList
				err := getObjects(&events, "events", "-n", ns, "--field-selector="+fieldSelector)
				switch {
				case errors.Is(err, ErrObjectNotFound):
					return errors.New("failure event not found")
				case err != nil:
					return fmt.Errorf("failed to get event. err: %w", err)
				default:
					return nil
				}
			}).Should(Succeed())

			By("deleting the Pod and PVC")
			_, err = kubectlWithInput(podYaml, "delete", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())
			_, err = kubectlWithInput(claimYAML, "delete", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())
		},
		Entry("xfs", "topolvm-provisioner", []int{
			2*1024*1024 - 10240, // 2MB - 10KiB XFS overhead
			3*1024*1024 - 10240, // 3MB - 10KiB XFS overhead
			4*1024*1024 - 10240, // 4MB - 10KiB XFS overhead
		}),
		Entry("btrfs", "topolvm-provisioner-btrfs", []int{
			2 * 1024 * 1024, // 2MB
			3 * 1024 * 1024, // 3MB
			4 * 1024 * 1024, // 4MB
		}),
	)

	It("should resize a block device", func() {
		By("deploying Pod with PVC")
		deviceFile := "/dev/e2etest"
		podYAML := []byte(fmt.Sprintf(podVolumeDeviceTemplateYAML, deviceFile))
		claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Block", 1024, "topolvm-provisioner"))

		_, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectlWithInput(podYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming that a block device exists in ubuntu pod")
		Eventually(func() error {
			_, err = kubectl("exec", "-n", ns, "ubuntu", "--", "test", "-b", deviceFile)
			if err != nil {
				return fmt.Errorf("failed to test. err: %w", err)
			}
			return nil
		}).Should(Succeed())

		By("resizing PVC")
		claimYAML = []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Block", 2048, "topolvm-provisioner"))
		_, err = kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming that the specified device is resized in the Pod")
		timeout := time.Minute * 5
		Eventually(func() error {
			stdout, err := kubectl("exec", "-n", ns, "ubuntu", "--", "blockdev", "--getsize64", deviceFile)
			if err != nil {
				return fmt.Errorf("failed to get volume size. err: %w", err)
			}
			volSize, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
			if err != nil {
				return fmt.Errorf("failed to convert volume size string. data: %s, err: %w", stdout, err)
			}
			if volSize != 2147483648 {
				return fmt.Errorf("failed to match volume size. actual: %d, expected: %d", volSize, 2147483648)
			}
			return nil
		}, timeout).Should(Succeed())

		By("deleting the Pod and PVC")
		_, err = kubectlWithInput(podYAML, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectlWithInput(claimYAML, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("should delete a pod when the pvc is deleted", func() {
		By("deploying a pod and PVC")
		claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 1024, "topolvm-provisioner"))
		podYaml := []byte(fmt.Sprintf(podVolumeMountTemplateYAML, "ubuntu", "topo-pvc"))

		_, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("deleting the PVC")
		_, err = kubectlWithInput(claimYAML, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming the pod is deleted")
		Eventually(func() error {
			var pod corev1.Pod
			err := getObjects(&pod, "pod", "-n", ns, "ubuntu")
			if err == ErrObjectNotFound {
				return nil
			}
			return errors.New("the pod exists")
		}).Should(Succeed())
	})
}

func verifyMountExists(ns string, pod string, mount string) error {
	_, err := kubectl("exec", "-n", ns, pod, "--", "mountpoint", "-d", mount)
	if err != nil {
		return fmt.Errorf("failed to check mount point. err: %w", err)
	}
	return nil
}

func waitCreatingDefaultSA(ns string) error {
	var sa corev1.ServiceAccount
	err := getObjects(&sa, "sa", "-n", ns, "default")
	if err != nil {
		return fmt.Errorf("default sa is not found. err=%w", err)
	}
	return nil
}

func waitCreatingPodWithPVC(podName, ns string) (string, error) {
	var pod corev1.Pod
	err := getObjects(&pod, "pod", "-n", ns, podName)
	if err != nil {
		return "", fmt.Errorf("failed to create pod. err: %w", err)
	}

	if pod.Spec.NodeName == "" {
		return "", fmt.Errorf("pod is not yet scheduled")
	}

	return pod.Spec.NodeName, nil
}
