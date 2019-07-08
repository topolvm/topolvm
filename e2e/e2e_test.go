package e2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/cybozu-go/topolvm"
	topolvmv1 "github.com/cybozu-go/topolvm/topolvm-node/api/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = FDescribe("E2E test", func() {
	testNamespacePrefix := "e2etest-"
	var ns string
	BeforeEach(func() {
		ns = testNamespacePrefix + randomString(10)
		createNamespace(ns)
	})

	AfterEach(func() {
		kubectl("delete", "namespaces/"+ns)
	})

	It("should be mounted in specified path", func() {
		By("deploying Pod with PVC")
		podYAML := `apiVersion: v1
kind: Pod
metadata:
  name: ubuntu
  labels:
    app.kubernetes.io/name: ubuntu
spec:
  containers:
    - name: ubuntu
      image: quay.io/cybozu/ubuntu:18.04
      command: ["sleep", "infinity"]
      volumeMounts:
        - mountPath: /test1
          name: my-volume
  volumes:
    - name: my-volume
      persistentVolumeClaim:
        claimName: topo-pvc
`
		claimYAML := `kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: topo-pvc
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: topolvm-provisioner
`
		stdout, stderr, err := kubectlWithInput([]byte(claimYAML), "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput([]byte(podYAML), "apply", "-n", ns, "-f", "-")
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
		stdout, stderr, err = kubectlWithInput([]byte(podYAML), "apply", "-n", ns, "-f", "-")
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
		stdout, stderr, err = kubectlWithInput([]byte(podYAML), "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput([]byte(claimYAML), "delete", "-n", ns, "-f", "-")
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
		podYAML := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: ubuntu
  labels:
    app.kubernetes.io/name: ubuntu
spec:
  containers:
    - name: ubuntu
      image: quay.io/cybozu/ubuntu:18.04
      command: ["sleep", "infinity"]
      volumeDevices:
        - devicePath: %s
          name: my-volume
  volumes:
    - name: my-volume
      persistentVolumeClaim:
        claimName: topo-pvc
`, deviceFile)
		claimYAML := `kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: topo-pvc
spec:
  volumeMode: Block
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: topolvm-provisioner
`
		stdout, stderr, err := kubectlWithInput([]byte(claimYAML), "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput([]byte(podYAML), "apply", "-n", ns, "-f", "-")
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
		stdout, stderr, err = kubectlWithInput([]byte(podYAML), "apply", "-n", ns, "-f", "-")
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
		stdout, stderr, err = kubectlWithInput([]byte(podYAML), "delete", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectlWithInput([]byte(claimYAML), "delete", "-n", ns, "-f", "-")
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

		// Repeat applying a PVC to make sure that the volume is created on the node with the largest capacity in each loop.
		for i := 0; i < 3; i++ {
			By("getting the node with max capacity (loop: " + strconv.Itoa(i) + ")")
			stdout, stderr, err := kubectl("get", "nodes", "-o", "json")
			Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
			var nodes corev1.NodeList
			err = json.Unmarshal(stdout, &nodes)
			Expect(err).ShouldNot(HaveOccurred(), "stdout=%s", stdout)

			var maxCapNodes []string
			var maxCapacity int
			for _, node := range nodes.Items {
				if node.Name == "kind-control-plane" {
					continue
				}
				strCap, ok := node.Annotations[topolvm.CapacityKey]
				Expect(ok).To(Equal(true), "capacity is not annotated: "+node.Name)
				cap, err := strconv.Atoi(strCap)
				Expect(err).ShouldNot(HaveOccurred())
				fmt.Printf("%s: %d bytes\n", node.Name, cap)
				switch {
				case cap > maxCapacity:
					maxCapacity = cap
					maxCapNodes = []string{node.GetName()}
				case cap == maxCapacity:
					maxCapNodes = append(maxCapNodes, node.GetName())
				}
			}
			Expect(len(maxCapNodes)).To(Equal(3 - i))

			By("creating pvc")
			claimYAML := fmt.Sprintf(`kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: topo-pvc-%d
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: topolvm-provisioner-immediate
`, i)
			stdout, stderr, err = kubectlWithInput([]byte(claimYAML), "apply", "-n", ns, "-f", "-")
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
		By("creating pvc")
		claimYAML := `kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: topo-pvc
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: topolvm-provisioner-immediate
`
		stdout, stderr, err := kubectlWithInput([]byte(claimYAML), "apply", "-n", ns, "-f", "-")
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
		podYAML := `apiVersion: v1
kind: Pod
metadata:
  name: ubuntu
  labels:
    app.kubernetes.io/name: ubuntu
spec:
  containers:
    - name: ubuntu
      image: quay.io/cybozu/ubuntu:18.04
      command: ["sleep", "infinity"]
      volumeMounts:
        - mountPath: /test1
          name: my-volume
  volumes:
    - name: my-volume
      persistentVolumeClaim:
        claimName: topo-pvc
`

		stdout, stderr, err = kubectlWithInput([]byte(podYAML), "apply", "-n", ns, "-f", "-")
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
		stdout, stderr, err = kubectlWithInput([]byte(podYAML), "apply", "-n", ns, "-f", "-")
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
			- node1: 20 / 20 GB (targetNode)
			- node2:  8 / 20 GB
			- node3:  8 / 20 GB

			# 1st case: test for `prioritize`
			Try to create 8GB PVC. Then
			- node1: 20 / 20 GB -> `prioritize` 4 -> selected
			- node2:  8 / 20 GB -> `prioritize` 3
			- node3:  8 / 20 GB -> `prioritize` 3

			# 2nd case: test for `predicate` (1)
			Try to create 10GB PVC. Then
			- node1: 12 / 20 GB -> selected
			- node2:  8 / 20 GB -> filtered (insufficient capacity)
			- node3:  8 / 20 GB -> filtered (insufficient capacity)

			# 3rd case: test for `predicate` (2)
			Try to create 10GB PVC. Then it cause error.
			- node1:  2 / 20 GB -> filtered (insufficient capacity)
			- node2:  8 / 20 GB -> filtered (insufficient capacity)
			- node3:  8 / 20 GB -> filtered (insufficient capacity)
		*/
		By("initializing node capacity")
		claimYAML := `kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: topo-pvc-dummy-1
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 12Gi
  storageClassName: topolvm-provisioner-immediate
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: topo-pvc-dummy-2
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 12Gi
  storageClassName: topolvm-provisioner-immediate
`
		stdout, stderr, err := kubectlWithInput([]byte(claimYAML), "apply", "-n", ns, "-f", "-")
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
			if node.Name == "kind-control-plane" {
				continue
			}

			strCap, ok := node.Annotations[topolvm.CapacityKey]
			Expect(ok).To(Equal(true), "capacity is not annotated: "+node.Name)
			cap, err := strconv.Atoi(strCap)
			Expect(err).ShouldNot(HaveOccurred())

			fmt.Printf("%s: %d", node.Name, cap)
			if cap > maxCapacity {
				maxCapacity = cap
				targetNode = node.Name
			}
		}

		By("creating pvc")
		claimYAML = `kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: topo-pvc1
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 8Gi
  storageClassName: topolvm-provisioner
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: topo-pvc2
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: topolvm-provisioner
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: topo-pvc3
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: topolvm-provisioner
`

		stdout, stderr, err = kubectlWithInput([]byte(claimYAML), "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		podYAMLTmpl := `apiVersion: v1
kind: Pod
metadata:
  name: ubuntu%d
  labels:
    app.kubernetes.io/name: ubuntu
spec:
  containers:
    - name: ubuntu
      image: quay.io/cybozu/ubuntu:18.04
      command: ["sleep", "infinity"]
      volumeMounts:
        - mountPath: /test1
          name: my-volume
  volumes:
    - name: my-volume
      persistentVolumeClaim:
        claimName: topo-pvc%d
`
		var boundNode string

		By("confirming that claiming 8GB pv to the targetNode is successful")
		stdout, stderr, err = kubectlWithInput([]byte(fmt.Sprintf(podYAMLTmpl, 1, 1)), "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Eventually(func() error {
			boundNode, err = waitCreatingPodWithPVC("ubuntu1", ns)
			return err
		}).Should(Succeed())
		Expect(boundNode).To(Equal(targetNode), "bound: %s, target: %s", boundNode, targetNode)

		By("confirming that claiming 10GB pv to the targetNode is successful")
		stdout, stderr, err = kubectlWithInput([]byte(fmt.Sprintf(podYAMLTmpl, 2, 2)), "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Eventually(func() error {
			boundNode, err = waitCreatingPodWithPVC("ubuntu2", ns)
			return err
		}).Should(Succeed())
		Expect(boundNode).To(Equal(targetNode), "bound: %s, target: %s", boundNode, targetNode)

		By("confirming that claiming 10GB pv to the targetNode is unsuccessful")
		stdout, stderr, err = kubectlWithInput([]byte(fmt.Sprintf(podYAMLTmpl, 3, 3)), "apply", "-n", ns, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Eventually(func() error {
			// TODO
			return nil
		}).Should(Succeed())
	})
})

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
