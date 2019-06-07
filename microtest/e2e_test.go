package microtest

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("E2E test", func() {
	testNamespace := "e2e-test"

	BeforeEach(func() {
		kubectl("delete", "namespace", testNamespace)
		stdout, stderr, err := kubectl("create", "namespace", testNamespace)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Eventually(func() error {
			return waitCreatingDefaultSA(testNamespace)
		}).Should(Succeed())
	})

	AfterEach(func() {
		//kubectl("delete", "namespace", testNamespace)
	})

	It("should be mounted in specified path", func() {
		By("initialize LogicalVolume CRD")
		stdout, stderr, err := kubectl("create", "namespace", "topolvm-system")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		//defer kubectl("delete", "namespace", "topolvm-system")

		Eventually(func() error {
			return waitCreatingDefaultSA("topolvm-system")
		}).Should(Succeed())

		stdout, stderr, err = kubectl("apply", "-f", "../topolvm-node/config/crd/bases/topolvm.cybozu.com_logicalvolumes.yaml")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("initialize topolvm services")
		stdout, stderr, err = kubectl("apply", "-f", "./csi.yml")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("deploying Pod with PVC")
		yml := `kind: PersistentVolumeClaim
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
---
apiVersion: v1
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
		stdout, stderr, err = kubectlWithInput([]byte(yml), "apply", "-n", testNamespace, "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("confirming that the specified device exists in the Pod")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pvc", "topo-pvc", "-n", testNamespace)
			if err != nil {
				return fmt.Errorf("failed to create PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			stdout, stderr, err = kubectl("get", "pods", "ubuntu", "-n", testNamespace)
			if err != nil {
				return fmt.Errorf("failed to create Pod. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			stdout, stderr, err = kubectl("exec", "-n", testNamespace, "ubuntu", "--", "mountpoint", "-d", "/test1")
			if err != nil {
				return fmt.Errorf("failed to check mount point. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
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
