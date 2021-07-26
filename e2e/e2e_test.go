package e2e

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	corev1 "k8s.io/api/core/v1"
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

//go:embed testdata/e2e/ephemeral-volume-pod.yaml
var ephemeralVolumePodYAML []byte

func testE2E() {
	testNamespacePrefix := "e2etest-"
	var ns string
	var cc CleanupContext

	BeforeEach(func() {
		cc = commonBeforeEach()

		ns = testNamespacePrefix + randomString(10)
		createNamespace(ns)
	})

	AfterEach(func() {
		// When a test fails, I want to investigate the cause. So please don't remove the namespace!
		if !CurrentGinkgoTestDescription().Failed {
			kubectl("delete", "namespaces/"+ns)
		}

		commonAfterEach(cc)
	})

	It("should be mounted in specified path", func() {
		By("deploying Pod with PVC")
		claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 1, "topolvm-provisioner"))
		podYaml := []byte(fmt.Sprintf(podVolumeMountTemplateYAML, "ubuntu", "topo-pvc"))

		stdout, stderr, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that the specified device exists in the Pod")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pvc", "topo-pvc", "-n", ns)
			if err != nil {
				return fmt.Errorf("failed to create PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			stdout, stderr, err = kubectl("get", "pods", "ubuntu", "-n", ns)
			if err != nil {
				return fmt.Errorf("failed to create Pod. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "mountpoint", "-d", "/test1")
			if err != nil {
				return fmt.Errorf("failed to check mount point. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "grep", "/test1", "/proc/mounts")
			if err != nil {
				return err
			}
			fields := strings.Fields(string(stdout))
			if fields[2] != "xfs" {
				return errors.New("/test1 is not xfs")
			}
			return nil
		}).Should(Succeed())

		By("writing file under /test1")
		writePath := "/test1/bootstrap.log"
		stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "cp", "/var/log/bootstrap.log", writePath)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "sync")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "cat", writePath)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).ShouldNot(BeEmpty())

		By("deleting the Pod, then recreating it")
		stdout, stderr, err = kubectl("delete", "--now=true", "-n", ns, "pod/ubuntu")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that the file exists")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pvc", "topo-pvc", "-n", ns)
			if err != nil {
				return fmt.Errorf("failed to create PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			stdout, stderr, err = kubectl("get", "pods", "ubuntu", "-n", ns)
			if err != nil {
				return fmt.Errorf("failed to create Pod. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "cat", writePath)
			if err != nil {
				return fmt.Errorf("failed to cat. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			if len(strings.TrimSpace(string(stdout))) == 0 {
				return fmt.Errorf(writePath + " is empty")
			}
			return nil
		}).Should(Succeed())

		By("confirming that the lv correspond to LogicalVolume resource is registered in LVM")
		stdout, stderr, err = kubectl("get", "pvc", "-n", ns, "topo-pvc", "-o=template", "--template={{.spec.volumeName}}")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		volName := strings.TrimSpace(string(stdout))
		Eventually(func() error {
			return checkLVIsRegisteredInLVM(volName)
		}).Should(Succeed())

		By("deleting the Pod and PVC")
		stdout, stderr, err = kubectlWithInput(podYaml, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput(claimYAML, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that the PV is deleted")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pv", volName, "--ignore-not-found")
			if err != nil {
				return fmt.Errorf("failed to get pv/%s. stdout: %s, stderr: %s, err: %v", volName, stdout, stderr, err)
			}
			if len(strings.TrimSpace(string(stdout))) != 0 {
				return fmt.Errorf("target pv exists %s", volName)
			}
			return nil
		}).Should(Succeed())

		By("confirming that the lv correspond to LogicalVolume is deleted")
		Eventually(func() error {
			return checkLVIsDeletedInLVM(volName)
		}).Should(Succeed())
	})

	It("should create a block device for Pod", func() {
		deviceFile := "/dev/e2etest"

		By("deploying ubuntu Pod with PVC to mount a block device")
		podYAML := []byte(fmt.Sprintf(podVolumeDeviceTemplateYAML, deviceFile))
		claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Block", 1, "topolvm-provisioner"))

		stdout, stderr, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput(podYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that a block device exists in ubuntu pod")
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "-n", ns, "pvc", "topo-pvc", "--template={{.spec.volumeName}}")
			if err != nil {
				return fmt.Errorf("failed to get volume name of topo-pvc. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "test", "-b", deviceFile)
			if err != nil {
				podinfo, _, _ := kubectl("-n", ns, "describe", "pod", "ubuntu")
				return fmt.Errorf("failed to test. stdout: %s, stderr: %s, err: %v; ubuntu pod info stdout: %s", stdout, stderr, err, podinfo)
			}
			return nil
		}).Should(Succeed())

		By("writing data to a block device")
		// /etc/hostname contains "ubuntu"
		stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "dd", "if=/etc/hostname", "of="+deviceFile)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "sync")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "dd", "if="+deviceFile, "of=/dev/stdout", "bs=6", "count=1", "status=none")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(string(stdout)).Should(Equal("ubuntu"))

		By("deleting the Pod, then recreating it")
		stdout, stderr, err = kubectl("delete", "--now=true", "-n", ns, "pod/ubuntu")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput(podYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("reading data from a block device")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pvc", "topo-pvc", "-n", ns)
			if err != nil {
				return fmt.Errorf("failed to create PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			stdout, stderr, err = kubectl("get", "pods", "ubuntu", "-n", ns)
			if err != nil {
				return fmt.Errorf("failed to create Pod. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "dd", "if="+deviceFile, "of=/dev/stdout", "bs=6", "count=1", "status=none")
			if err != nil {
				return fmt.Errorf("failed to cat. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			if string(stdout) != "ubuntu" {
				return fmt.Errorf("expected: ubuntu, actual: %s", string(stdout))
			}
			return nil
		}).Should(Succeed())

		By("confirming that the lv correspond to LogicalVolume resource is registered in LVM")
		stdout, stderr, err = kubectl("get", "pvc", "-n", ns, "topo-pvc", "-o=template", "--template={{.spec.volumeName}}")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		volName := strings.TrimSpace(string(stdout))
		Eventually(func() error {
			return checkLVIsRegisteredInLVM(volName)
		}).Should(Succeed())

		By("deleting the Pod and PVC")
		stdout, stderr, err = kubectlWithInput(podYAML, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput(claimYAML, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that the PV is deleted")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pv", volName, "--ignore-not-found")
			if err != nil {
				return fmt.Errorf("failed to get pv/%s. stdout: %s, stderr: %s, err: %v", volName, stdout, stderr, err)
			}
			if len(strings.TrimSpace(string(stdout))) != 0 {
				return fmt.Errorf("target PV exists %s", volName)
			}
			return nil
		}).Should(Succeed())

		By("confirming that the lv correspond to LogicalVolume is deleted")
		Eventually(func() error {
			return checkLVIsDeletedInLVM(volName)
		}).Should(Succeed())
	})

	It("should choose a node with the largest capacity when volumeBindingMode == Immediate is specified", func() {
		if isStorageCapacity() {
			Skip(skipMessageForStorageCapacity + " and Storage Capacity Tracking doesn't check Storage Capacity when volumeBindingMode == Immediate is specified")
			return
		}

		num := 3
		if isDaemonsetLvmdEnvSet() {
			num = 0
		}

		// Repeat applying a PVC to make sure that the volume is created on the node with the largest capacity in each loop.
		for i := 0; i < num; i++ {
			By("getting the node with max capacity (loop: " + strconv.Itoa(i) + ")")
			var maxCapNodes []string
			Eventually(func() error {
				var maxCapacity int
				maxCapNodes = []string{}
				stdout, stderr, err := kubectl("get", "nodes", "-o", "json")
				if err != nil {
					return fmt.Errorf("kubectl get nodes error: stdout=%s, stderr=%s", stdout, stderr)
				}
				var nodes corev1.NodeList
				err = json.Unmarshal(stdout, &nodes)
				if err != nil {
					return fmt.Errorf("unmarshal error: stdout=%s", stdout)
				}
				for _, node := range nodes.Items {
					if node.Name == "topolvm-e2e-control-plane" {
						continue
					}
					strCap, ok := node.Annotations[topolvm.CapacityKeyPrefix+"ssd"]
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
				if len(maxCapNodes) != 3-i {
					return fmt.Errorf("unexpected number of maxCapNodes: expected: %d, actual: %d", 3-i, len(maxCapNodes))
				}
				return nil
			}).Should(Succeed())

			By("creating pvc")
			claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, fmt.Sprintf("topo-pvc-%d", i), "Filesystem", 1, "topolvm-provisioner-immediate"))
			stdout, stderr, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
			Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

			var volumeName string
			Eventually(func() error {
				stdout, stderr, err = kubectl("get", "-n", ns, "pvc", "topo-pvc-"+strconv.Itoa(i), "-o", "json")
				if err != nil {
					return fmt.Errorf("failed to get PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
				}

				var pvc corev1.PersistentVolumeClaim
				err = json.Unmarshal(stdout, &pvc)
				if err != nil {
					return fmt.Errorf("failed to unmarshal PVC. stdout: %s, err: %v", stdout, err)
				}

				if pvc.Spec.VolumeName == "" {
					return errors.New("pvc.Spec.VolumeName should not be empty")
				}

				volumeName = pvc.Spec.VolumeName
				return nil
			}).Should(Succeed())

			By("confirming that the logical volume was scheduled onto the node with max capacity")
			stdout, stderr, err = kubectl("get", "-n", "topolvm-system", "logicalvolumes", volumeName, "-o", "json")
			Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

			var lv topolvmv1.LogicalVolume
			err = json.Unmarshal(stdout, &lv)
			Expect(err).ShouldNot(HaveOccurred())

			target := lv.Spec.NodeName
			Expect(containString(maxCapNodes, target)).To(Equal(true), "maxCapNodes: %v, target: %s", maxCapNodes, target)
		}
	})

	It("should scheduled onto the correct node where PV exists (volumeBindingMode == Immediate)", func() {
		if isStorageCapacity() {
			Skip(skipMessageForStorageCapacity + " and Storage Capacity Tracking doesn't check Storage Capacity when volumeBindingMode == Immediate is specified")
			return
		}

		By("creating pvc")
		claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 1, "topolvm-provisioner-immediate"))
		stdout, stderr, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		var volumeName string
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "-n", ns, "pvc", "topo-pvc", "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var pvc corev1.PersistentVolumeClaim
			err = json.Unmarshal(stdout, &pvc)
			if err != nil {
				return fmt.Errorf("failed to unmarshal PVC. stdout: %s, err: %v", stdout, err)
			}

			if pvc.Spec.VolumeName == "" {
				return errors.New("pvc.Spec.VolumeName should not be empty")
			}

			volumeName = pvc.Spec.VolumeName
			return nil
		}).Should(Succeed())

		By("getting node name of which volume is created")
		stdout, stderr, err = kubectl("get", "-n", "topolvm-system", "logicalvolumes", volumeName, "-o", "json")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		var lv topolvmv1.LogicalVolume
		err = json.Unmarshal(stdout, &lv)
		Expect(err).ShouldNot(HaveOccurred())

		nodeName := lv.Spec.NodeName

		By("deploying ubuntu Pod with PVC")
		podYaml := []byte(fmt.Sprintf(podVolumeMountTemplateYAML, "ubuntu", "topo-pvc"))
		stdout, stderr, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that ubuntu pod is scheduled onto " + nodeName)
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "-n", ns, "pod", "ubuntu", "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to create pod. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var pod corev1.Pod
			err = json.Unmarshal(stdout, &pod)
			if err != nil {
				return fmt.Errorf("failed to unmarshal pod. stdout: %s, err: %v", stdout, err)
			}

			if pod.Spec.NodeName != nodeName {
				return fmt.Errorf("pod is not yet scheduled")
			}

			return nil
		}).Should(Succeed())

		By("deleting the Pod, then recreating it")
		stdout, stderr, err = kubectl("delete", "--now=true", "-n", ns, "pod/ubuntu")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that ubuntu pod is rescheduled onto " + nodeName)
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "-n", ns, "pod", "ubuntu", "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to create pod. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var pod corev1.Pod
			err = json.Unmarshal(stdout, &pod)
			if err != nil {
				return fmt.Errorf("failed to unmarshal pod. stdout: %s, err: %v", stdout, err)
			}

			if pod.Spec.NodeName != nodeName {
				return fmt.Errorf("pod is not yet scheduled")
			}

			return nil
		}).Should(Succeed())
	})

	It("should schedule pods and volumes according to topolvm-scheduler", func() {
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
		// Skip because this test requires multiple nodes but there is just one node in daemonset lvmd test environment.
		skipIfDaemonsetLvmd()
		By("initializing node capacity")
		stdout, stderr, err := kubectlWithInput(nodeCapacityPVCYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "-n", ns, "pvc", "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var pvcList corev1.PersistentVolumeClaimList
			err = json.Unmarshal(stdout, &pvcList)
			if err != nil {
				return fmt.Errorf("failed to unmarshal PVC. stdout: %s, err: %v", stdout, err)
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
		stdout, stderr, err = kubectl("get", "node", "-o", "json")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		var nodeList corev1.NodeList
		err = json.Unmarshal(stdout, &nodeList)
		Expect(err).ShouldNot(HaveOccurred())

		var targetNode string
		var maxCapacity int
		for _, node := range nodeList.Items {
			if node.Name == "topolvm-e2e-control-plane" {
				continue
			}

			strCap, ok := node.Annotations[topolvm.CapacityKeyPrefix+"ssd"]
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
		stdout, stderr, err = kubectlWithInput(nodeCapacityPVC2YAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		var boundNode string
		By("confirming that claiming 8GB pv to the targetNode is successful")
		stdout, stderr, err = kubectlWithInput([]byte(fmt.Sprintf(podVolumeMountTemplateYAML, "ubuntu1", "topo-pvc1")), "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Eventually(func() error {
			boundNode, err = waitCreatingPodWithPVC("ubuntu1", ns)
			return err
		}).Should(Succeed())
		Expect(boundNode).To(Equal(targetNode), "bound: %s, target: %s", boundNode, targetNode)

		By("confirming that claiming 6GB pv to the targetNode is successful")
		stdout, stderr, err = kubectlWithInput([]byte(fmt.Sprintf(podVolumeMountTemplateYAML, "ubuntu2", "topo-pvc2")), "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Eventually(func() error {
			boundNode, err = waitCreatingPodWithPVC("ubuntu2", ns)
			return err
		}).Should(Succeed())
		Expect(boundNode).To(Equal(targetNode), "bound: %s, target: %s", boundNode, targetNode)

		By("confirming that claiming 8GB pv to the targetNode is unsuccessful")
		stdout, stderr, err = kubectlWithInput([]byte(fmt.Sprintf(podVolumeMountTemplateYAML, "ubuntu3", "topo-pvc3")), "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		time.Sleep(15 * time.Second)

		stdout, stderr, err = kubectl("get", "-n", ns, "pod", "ubuntu3", "-o", "json")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		var pod corev1.Pod
		err = json.Unmarshal(stdout, &pod)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s", stdout)
		Expect(pod.Spec.NodeName).To(Equal(""))
	})

	It("should mount inline ephemeral volumes backed by LVMs to the pod and delete LVMs when pod is deleted", func() {
		By("reading current count of LVMs")
		baseLvmCount, err := countLVMs()
		Expect(err).ShouldNot(HaveOccurred())

		By("deploying Pod with a TopoLVM inline ephemeral volume")
		stdout, stderr, err := kubectlWithInput(ephemeralVolumePodYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that the specified mountpoints exist in the Pod")
		Eventually(func() error {
			err := verifyMountExists(ns, "ubuntu", "/test1")
			if err != nil {
				return err
			}

			err = verifyMountExists(ns, "ubuntu", "/test2")
			if err != nil {
				return err
			}
			return nil
		}).Should(Succeed())

		// 2086912 is the number of 1k blocks to expect for a xfs volume
		// formatted from a raw 2 Gi block device
		verifyMountProperties(ns, "ubuntu", "/test1", "xfs", 2086912)

		// 999320 is the number of 1k blocks to expect for an ext4 volume
		// formatted from a raw 1 Gi block device
		verifyMountProperties(ns, "ubuntu", "/test2", "ext4", 999320)

		By("writing file under /test1")
		writePath := "/test1/bootstrap.log"
		stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "cp", "/var/log/bootstrap.log", writePath)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "sync")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "cat", writePath)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).ShouldNot(BeEmpty())

		By("confirming the mounted dir permission is 2777")
		stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "stat", "/test1", "-c", "'%a'")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal("'2777'"))

		By("confirming two LVMs were created")
		postCreateLvmCount, err := countLVMs()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(postCreateLvmCount).To(Equal(baseLvmCount + 2))

		By("deleting the Pod")
		stdout, stderr, err = kubectlWithInput(ephemeralVolumePodYAML, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("verifying that the two LVMs were removed")
		postDeleteLvmCount, err := countLVMs()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(postDeleteLvmCount).To(Equal(baseLvmCount))
	})

	It("should resize filesystem", func() {
		By("deploying Pod with PVC")
		claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 1, "topolvm-provisioner"))
		podYaml := []byte(fmt.Sprintf(podVolumeMountTemplateYAML, "ubuntu", "topo-pvc"))

		stdout, stderr, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that the specified device is mounted in the Pod")
		Eventually(func() error {
			return verifyMountExists(ns, "ubuntu", "/test1")
		}).Should(Succeed())

		By("resizing PVC online")
		claimYAML = []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 2, "topolvm-provisioner"))
		stdout, stderr, err = kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that the specified device is resized in the Pod")
		timeout := time.Minute * 5
		Eventually(func() error {
			stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "df", "--output=size", "/test1")
			if err != nil {
				return fmt.Errorf("failed to get volume size. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			dfFields := strings.Fields(string(stdout))
			volSize, err := strconv.Atoi(dfFields[1])
			if err != nil {
				return fmt.Errorf("failed to convert volume size string. stdout: %s, err: %v", stdout, err)
			}
			if volSize != 2086912 {
				return fmt.Errorf("failed to match volume size. actual: %d, expected: %d", volSize, 2086912)
			}
			return nil
		}, timeout).Should(Succeed())

		By("deleting Pod for offline resizing")
		stdout, stderr, err = kubectlWithInput(podYaml, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("resizing PVC offline")
		claimYAML = []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 3, "topolvm-provisioner"))
		stdout, stderr, err = kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("deploying Pod")
		stdout, stderr, err = kubectlWithInput(podYaml, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that the specified device is resized in the Pod")
		Eventually(func() error {
			stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "df", "--output=size", "/test1")
			if err != nil {
				return fmt.Errorf("failed to get volume size. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			dfFields := strings.Fields((string(stdout)))
			volSize, err := strconv.Atoi(dfFields[1])
			if err != nil {
				return fmt.Errorf("failed to convert volume size string. stdout: %s, err: %v", stdout, err)
			}
			if volSize != 3135488 {
				return fmt.Errorf("failed to match volume size. actual: %d, expected: %d", volSize, 3135488)
			}
			return nil
		}, timeout).Should(Succeed())

		By("deleting topolvm-node Pods to clear /dev/topolvm/*")
		stdout, stderr, err = kubectl("delete", "-n", ns, "pod", "-l=app.kubernetes.io/name=topolvm-node")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("resizing PVC")
		claimYAML = []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 4, "topolvm-provisioner"))
		stdout, stderr, err = kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that the specified device is resized in the Pod")
		Eventually(func() error {
			stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "df", "--output=size", "/test1")
			if err != nil {
				return fmt.Errorf("failed to get volume size. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			dfFields := strings.Fields(string(stdout))
			volSize, err := strconv.Atoi(dfFields[1])
			if err != nil {
				return fmt.Errorf("failed to convert volume size string. stdout: %s, err: %v", stdout, err)
			}
			if volSize != 4184064 {
				return fmt.Errorf("failed to match volume size. actual: %d, expected: %d", volSize, 4184064)
			}
			return nil
		}, timeout).Should(Succeed())

		By("confirming that no failure event has occurred")
		fieldSelector := "involvedObject.kind=PersistentVolumeClaim," +
			"involvedObject.name=topo-pvc," +
			"reason=VolumeResizeFailed"
		stdout, stderr, err = kubectl("get", "-n", ns, "events", "-o", "json", "--field-selector="+fieldSelector)
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		var events corev1.EventList
		err = json.Unmarshal(stdout, &events)
		Expect(err).NotTo(HaveOccurred(), "stdout=%s", stdout)
		Expect(events.Items).To(BeEmpty())

		By("resizing PVC over vg capacity")
		claimYAML = []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Filesystem", 100, "topolvm-provisioner"))
		stdout, stderr, err = kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that a failure event occurs")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "-n", ns, "events", "-o", "json", "--field-selector="+fieldSelector)
			if err != nil {
				return fmt.Errorf("failed to get event. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var events corev1.EventList
			err = json.Unmarshal(stdout, &events)
			if err != nil {
				return fmt.Errorf("failed to unmarshal events. stdout: %s, err: %v", stdout, err)
			}

			if len(events.Items) == 0 {
				return errors.New("failure event not found")
			}
			return nil
		}).Should(Succeed())

		By("deleting the Pod and PVC")
		stdout, stderr, err = kubectlWithInput(podYaml, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput(claimYAML, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	})

	It("should resize a block device", func() {
		By("deploying Pod with PVC")
		deviceFile := "/dev/e2etest"
		podYAML := []byte(fmt.Sprintf(podVolumeDeviceTemplateYAML, deviceFile))
		claimYAML := []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Block", 1, "topolvm-provisioner"))

		stdout, stderr, err := kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput(podYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that a block device exists in ubuntu pod")
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "-n", ns, "pvc", "topo-pvc", "--template={{.spec.volumeName}}")
			if err != nil {
				return fmt.Errorf("failed to get volume name of topo-pvc. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "test", "-b", deviceFile)
			if err != nil {
				return fmt.Errorf("failed to test. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			return nil
		}).Should(Succeed())

		By("resizing PVC")
		claimYAML = []byte(fmt.Sprintf(pvcTemplateYAML, "topo-pvc", "Block", 2, "topolvm-provisioner"))
		stdout, stderr, err = kubectlWithInput(claimYAML, "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that the specified device is resized in the Pod")
		timeout := time.Minute * 5
		Eventually(func() error {
			stdout, stderr, err = kubectl("exec", "-n", ns, "ubuntu", "--", "blockdev", "--getsize64", deviceFile)
			if err != nil {
				return fmt.Errorf("failed to get volume size. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			volSize, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
			if err != nil {
				return fmt.Errorf("failed to convert volume size string. stdout: %s, err: %v", stdout, err)
			}
			if volSize != 2147483648 {
				return fmt.Errorf("failed to match volume size. actual: %d, expected: %d", volSize, 2147483648)
			}
			return nil
		}, timeout).Should(Succeed())

		By("deleting the Pod and PVC")
		stdout, stderr, err = kubectlWithInput(podYAML, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput(claimYAML, "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	})
}

func verifyMountExists(ns string, pod string, mount string) error {
	stdout, stderr, err := kubectl("exec", "-n", ns, pod, "--", "mountpoint", "-d", mount)
	if err != nil {
		return fmt.Errorf("failed to check mount point. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
	}
	return nil
}

func verifyMountProperties(ns string, pod string, mount string, fsType string, size int) {
	By(fmt.Sprintf("verifying that %s is mounted as type %s", mount, fsType))

	stdout, stderr, err := kubectl("exec", "-n", ns, pod, "grep", mount, "/proc/mounts")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	mountFields := strings.Fields(string(stdout))
	Expect(mountFields[2]).To(Equal(fsType))

	By(fmt.Sprintf("verifying that the volume mounted at %s has the correct size", mount))
	stdout, stderr, err = kubectl("exec", "-n", ns, pod, "--", "df", "--output=size", mount)
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

	dfFields := strings.Fields(string(stdout))
	volSize, err := strconv.Atoi(dfFields[1])
	Expect(err).ShouldNot(HaveOccurred())
	Expect(volSize).To(Equal(size))
}

func waitCreatingDefaultSA(ns string) error {
	stdout, stderr, err := kubectl("get", "sa", "-n", ns, "default")
	if err != nil {
		return fmt.Errorf("default sa is not found. stdout=%s, stderr=%s, err=%v", stdout, stderr, err)
	}
	return nil
}

func waitCreatingPodWithPVC(podName, ns string) (string, error) {
	stdout, stderr, err := kubectl("get", "-n", ns, "pod", podName, "-o", "json")
	if err != nil {
		return "", fmt.Errorf("failed to create pod. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
	}

	var pod corev1.Pod
	err = json.Unmarshal(stdout, &pod)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal pod. stdout: %s, err: %v", stdout, err)
	}

	if pod.Spec.NodeName == "" {
		return "", fmt.Errorf("pod is not yet scheduled")
	}

	return pod.Spec.NodeName, nil
}

func checkLVIsRegisteredInLVM(volName string) error {
	stdout, stderr, err := kubectl("get", "logicalvolume", "-n", "topolvm-system", volName, "-o=template", "--template={{.metadata.uid}}")
	if err != nil {
		return fmt.Errorf("err=%v, stdout=%s, stderr=%s", err, stdout, stderr)
	}
	lvName := strings.TrimSpace(string(stdout))
	stdout, err = exec.Command("sudo", "lvdisplay", "--select", "lv_name="+lvName).Output()
	if err != nil {
		return fmt.Errorf("err=%v, stdout=%s", err, stdout)
	}
	if strings.TrimSpace(string(stdout)) == "" {
		return fmt.Errorf("lv_name ( %s ) not found", lvName)
	}
	return nil
}

func checkLVIsDeletedInLVM(volName string) error {
	stdout, err := exec.Command("sudo", "lvdisplay", "--select", "lv_name="+volName).Output()
	if err != nil {
		return fmt.Errorf("failed to lvdisplay. stdout: %s, err: %v", stdout, err)
	}
	if len(strings.TrimSpace(string(stdout))) != 0 {
		return fmt.Errorf("target LV exists %s", volName)
	}
	return nil
}

func countLVMs() (int, error) {
	stdout, err := exec.Command("sudo", "lvs", "-o", "lv_name", "--noheadings").Output()
	if err != nil {
		return -1, fmt.Errorf("failed to lvs. stdout %s, err %v", stdout, err)
	}
	return bytes.Count(stdout, []byte("\n")), nil
}

func getNodeAnnotationMapWithPrefix(prefix string) (map[string]map[string]string, error) {
	stdout, stderr, err := kubectl("get", "node", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("stdout=%sr stderr=%s, err=%v", stdout, stderr, err)
	}

	var nodeList corev1.NodeList
	err = json.Unmarshal(stdout, &nodeList)
	if err != nil {
		return nil, err
	}

	capacities := make(map[string]map[string]string)
	for _, node := range nodeList.Items {
		if node.Name == "topolvm-e2e-control-plane" {
			continue
		}

		capacities[node.Name] = make(map[string]string)
		for k, v := range node.Annotations {
			if !strings.HasPrefix(k, prefix) {
				continue
			}
			capacities[node.Name][k] = v
		}
	}
	return capacities, nil
}
