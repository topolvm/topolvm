package kindtest

import (
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("E2E test", func() {
	testNamespacePrefix := "e2e-test"

	It("should be mounted in specified path", func() {
		ns := testNamespacePrefix + randomString(10)
		createNamespace(ns)

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
			return nil
		}).Should(Succeed())

		By("confirming that the lv is formatted as xfs")
		stdout, stderr, err = kubectl("get", "pvc", "-n", ns, "topo-pvc", "-o=template", "--template={{.spec.volumeName}}")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		volName := strings.TrimSpace(string(stdout))
		stdout, stderr, err = kubectl("get", "logicalvolume", "-n", "topolvm-system", volName, "-o=template", "--template={{.metadata.uid}}")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, err = exec.Command("sudo", "xfs_info", "/dev/myvg/"+string(stdout)).CombinedOutput()
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s", string(stdout))

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
		volName = strings.TrimSpace(string(stdout))
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
		ns := testNamespacePrefix + randomString(10)
		deviceFile := "/dev/e2etest"
		createNamespace(ns)

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
})

func waitCreatingDefaultSA(ns string) error {
	stdout, stderr, err := kubectl("get", "sa", "-n", ns, "default")
	if err != nil {
		return fmt.Errorf("default sa is not found. stdout=%s, stderr=%s, err=%v", stdout, stderr, err)
	}
	return nil
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
