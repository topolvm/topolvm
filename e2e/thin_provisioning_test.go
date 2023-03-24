package e2e

import (
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	corev1 "k8s.io/api/core/v1"
)

//go:embed testdata/thin_provisioning/pod-template.yaml
var thinPodTemplateYAML string

//go:embed testdata/thin_provisioning/pvc-template.yaml
var thinPVCTemplateYAML string

func testThinProvisioning() {
	testNamespacePrefix := "thinptest-"
	var ns string
	var cc CleanupContext
	BeforeEach(func() {
		cc = commonBeforeEach()
		ns = testNamespacePrefix + randomString(10)
		createNamespace(ns)
	})

	AfterEach(func() {
		// When a test fails, I want to investigate the cause. So please don't remove the namespace!
		if !CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
			kubectl("delete", "namespaces/"+ns)
		}
		commonAfterEach(cc)
	})

	It("should thin provision a PV", func() {
		By("deploying Pod with PVC")

		nodeName := "topolvm-e2e-worker"
		if isDaemonsetLvmdEnvSet() {
			nodeName = getDaemonsetLvmdNodeName()
		}

		thinPvcYAML := []byte(fmt.Sprintf(thinPVCTemplateYAML, "thinvol", "1"))
		_, err := kubectlWithInput(thinPvcYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		thinPodYAML := []byte(fmt.Sprintf(thinPodTemplateYAML, "thinpod", "thinvol", topolvm.GetTopologyNodeKey(), nodeName))
		_, err = kubectlWithInput(thinPodYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming that the lv was created in the thin volume group and pool")
		var volumeName string

		Eventually(func() error {
			volumeName, err = getVolumeNameofPVC("thinvol", ns)
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

		By("deleting the Pod and PVC")
		_, err = kubectlWithInput(thinPodYAML, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectlWithInput(thinPvcYAML, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming that the PV is deleted")
		Eventually(func() error {
			var pv corev1.PersistentVolume
			err := getObjects(&pv, "pv", volumeName)
			switch {
			case err == ErrObjectNotFound:
				return nil
			case err != nil:
				return fmt.Errorf("failed to get pv/%s. err: %v", volumeName, err)
			default:
				return fmt.Errorf("target pv exists %s", volumeName)
			}
		}).Should(Succeed())

		By("confirming that the lv correspond to LogicalVolume is deleted")
		Eventually(func() error {
			return checkLVIsDeletedInLVM(volumeName)
		}).Should(Succeed())
	})

	It("should overprovision thin PVCs", func() {
		By("deploying multiple PVCS with total size < thinpoolsize * overprovisioning")
		// The actual thinpool size is 4 GB . With an overprovisioning limit of 5, it should allow
		// PVCs totalling upto 20 GB for each node
		nodeName := "topolvm-e2e-worker2"
		if isDaemonsetLvmdEnvSet() {
			nodeName = getDaemonsetLvmdNodeName()
		}
		for i := 0; i < 5; i++ {
			num := strconv.Itoa(i)
			thinPvcYAML := []byte(fmt.Sprintf(thinPVCTemplateYAML, "thinvol"+num, "3"))
			_, err := kubectlWithInput(thinPvcYAML, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			thinPodYAML := []byte(fmt.Sprintf(thinPodTemplateYAML, "thinpod"+num, "thinvol"+num, topolvm.GetTopologyNodeKey(), nodeName))
			_, err = kubectlWithInput(thinPodYAML, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

		}

		By("confirming that the volumes have been created in the thinpool")

		for i := 0; i < 5; i++ {
			var volumeName string
			var err error

			num := strconv.Itoa(i)
			Eventually(func() error {
				volumeName, err = getVolumeNameofPVC("thinvol"+num, ns)
				return err
			}).Should(Succeed())

			var lv *thinlvinfo
			Eventually(func() error {
				lv, err = getThinLVInfo(volumeName)
				return err
			}).Should(Succeed())

			vgName := "node2-myvg4"
			if isDaemonsetLvmdEnvSet() {
				vgName = "node-myvg5"
			}
			Expect(vgName).Should(Equal(lv.vgName))

			poolName := "pool0"
			Expect(poolName).Should(Equal(lv.poolName))
		}

		By("deleting the Pods and PVCs")

		for i := 0; i < 5; i++ {
			num := strconv.Itoa(i)
			_, err := kubectl("delete", "-n", ns, "pod", "thinpod"+num)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = kubectl("delete", "-n", ns, "pvc", "thinvol"+num)
			Expect(err).ShouldNot(HaveOccurred())

			By("confirming the Pod is deleted")
			Eventually(func() error {
				var pod corev1.Pod
				err := getObjects(&pod, "pod", "-n", ns, "thinpod"+num)
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
				err := getObjects(&pvc, "pvc", "-n", ns, "thinvol"+num)
				switch {
				case err == ErrObjectNotFound:
					return nil
				case err != nil:
					return err
				default:
					return errors.New("the PVC exists")
				}
			}).Should(Succeed())
		}
	})

	It("should check overprovision limits", func() {
		By("Deploying a PVC to use up the available thinpoolsize * overprovisioning")

		nodeName := "topolvm-e2e-worker3"
		if isDaemonsetLvmdEnvSet() {
			nodeName = getDaemonsetLvmdNodeName()
		}

		thinPvcYAML := []byte(fmt.Sprintf(thinPVCTemplateYAML, "thinvol", "18"))
		_, err := kubectlWithInput(thinPvcYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		thinPodYAML := []byte(fmt.Sprintf(thinPodTemplateYAML, "thinpod", "thinvol", topolvm.GetTopologyNodeKey(), nodeName))
		_, err = kubectlWithInput(thinPodYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		var volumeName string
		Eventually(func() error {
			volumeName, err = getVolumeNameofPVC("thinvol", ns)
			return err
		}).Should(Succeed())

		var lv *thinlvinfo
		Eventually(func() error {
			lv, err = getThinLVInfo(volumeName)
			return err
		}).Should(Succeed())

		vgName := "node3-myvg4"
		if isDaemonsetLvmdEnvSet() {
			vgName = "node-myvg5"
		}
		Expect(vgName).Should(Equal(lv.vgName))

		poolName := "pool0"
		Expect(poolName).Should(Equal(lv.poolName))

		By("Failing to deploying a PVC when total size > thinpoolsize * overprovisioning")
		thinPvcYAML = []byte(fmt.Sprintf(thinPVCTemplateYAML, "thinvol2", "5"))
		_, err = kubectlWithInput(thinPvcYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		thinPodYAML = []byte(fmt.Sprintf(thinPodTemplateYAML, "thinpod2", "thinvol2", topolvm.GetTopologyNodeKey(), nodeName))
		_, err = kubectlWithInput(thinPodYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err = getObjects(&pvc, "pvc", "-n", ns, "thinvol2")
			if err != nil {
				return fmt.Errorf("failed to get PVC. err: %v", err)
			}
			if pvc.Status.Phase == corev1.ClaimBound {
				return fmt.Errorf("PVC should not be bound")
			}
			return nil
		}).Should(Succeed())

		By("Deleting the pods and pvcs")
		_, err = kubectl("delete", "-n", ns, "pod", "thinpod")
		Expect(err).ShouldNot(HaveOccurred())

		_, err = kubectl("delete", "-n", ns, "pod", "thinpod2")
		Expect(err).ShouldNot(HaveOccurred())

		_, err = kubectl("delete", "-n", ns, "pvc", "thinvol")
		Expect(err).ShouldNot(HaveOccurred())

		_, err = kubectl("delete", "-n", ns, "pvc", "thinvol2")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming the Pods are deleted")
		Eventually(func() error {
			var pod corev1.Pod
			err := getObjects(&pod, "pod", "-n", ns, "thinpod")
			switch {
			case err == ErrObjectNotFound:
				return nil
			case err != nil:
				return err
			default:
				return errors.New("the Pod exists")
			}
		}).Should(Succeed())

		Eventually(func() error {
			var pod corev1.Pod
			err := getObjects(&pod, "pod", "-n", ns, "thinpod2")
			switch {
			case err == ErrObjectNotFound:
				return nil
			case err != nil:
				return err
			default:
				return errors.New("the Pod exists")
			}
		}).Should(Succeed())

		By("confirming the PVCs are deleted")
		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err := getObjects(&pvc, "pvc", "-n", ns, "thinvol")
			switch {
			case err == ErrObjectNotFound:
				return nil
			case err != nil:
				return err
			default:
				return errors.New("the PVC exists")
			}
		}).Should(Succeed())

		Eventually(func() error {
			var pvc corev1.PersistentVolumeClaim
			err := getObjects(&pvc, "pvc", "-n", ns, "thinvol2")
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
}

type thinlvinfo struct {
	lvName   string
	poolName string
	vgName   string
}

func getThinLVInfo(volName string) (*thinlvinfo, error) {
	var lv topolvmv1.LogicalVolume
	err := getObjects(&lv, "logicalvolume", volName)
	if err != nil {
		return nil, err
	}

	lvName := string(lv.UID)
	stdout, err := execAtLocal("sudo", nil, "lvs", "--noheadings", "-o", "lv_name,pool_lv,vg_name", "--select", "lv_name="+lvName)
	if err != nil {
		return nil, err
	}
	output := strings.TrimSpace(string(stdout))
	if output == "" {
		return nil, fmt.Errorf("lv_name ( %s ) not found", lvName)
	}
	lines := strings.Split(output, "\n")
	if len(lines) != 1 {
		return nil, errors.New("found multiple lvs")
	}
	items := strings.Fields(strings.TrimSpace(lines[0]))
	if len(items) < 3 {
		return nil, fmt.Errorf("invalid format: %s", lines[0])
	}
	return &thinlvinfo{lvName: items[0], poolName: items[1], vgName: items[2]}, nil
}

func getVolumeNameofPVC(pvcName, ns string) (volName string, err error) {
	var pvc corev1.PersistentVolumeClaim
	err = getObjects(&pvc, "pvc", "-n", ns, pvcName)
	if err != nil {
		return "", fmt.Errorf("failed to get PVC. err: %v", err)
	}

	if pvc.Status.Phase != corev1.ClaimBound {
		return "", errors.New("pvc status is not bound")
	}
	if pvc.Spec.VolumeName == "" {
		return "", errors.New("pvc.Spec.VolumeName should not be empty")
	}

	volumeName := pvc.Spec.VolumeName
	return volumeName, nil
}
