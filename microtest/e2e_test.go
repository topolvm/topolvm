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
	})

	AfterEach(func() {
		kubectl("delete", "namespace", testNamespace)
	})

	It("should be mounted in specified path", func() {
		By("initialize LogicalVolume CRD")
		stdout, stderr, err := kubectl("create", "namespace", "topolvm-system")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		stdout, stderr, err = kubectl("apply", "-f", "../topolvm-node/config/crd/bases/topolvm.cybozu.com_logicalvolumes.yaml")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("initialize topolvm services")
		stdout, stderr, err = kubectl("apply", "-f", "./csi.yml")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("deploying Pod with PVC")
		yml := `
kind: PersistentVolumeClaim
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
			stdout, stderr, err = kubectl("exec", "-n", testNamespace, "ubuntu", "--", "mountpoint", "-d", "/test1")
			if err != nil {
				return fmt.Errorf("failed. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			return nil
		}).Should(Succeed())
	})
})
