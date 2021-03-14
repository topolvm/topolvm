package e2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/kubernetes-csi/csi-test/v4/pkg/sanity"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
)

var binDir string

func TestMtest(t *testing.T) {
	if os.Getenv("E2ETEST") == "" {
		t.Skip("Run under e2e/")
	}
	rand.Seed(time.Now().UnixNano())

	RegisterFailHandler(Fail)

	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultEventuallyTimeout(time.Minute)

	RunSpecs(t, "Test on sanity")
}

func createNamespace(ns string) {
	stdout, stderr, err := kubectl("create", "namespace", ns)
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	Eventually(func() error {
		return waitCreatingDefaultSA(ns)
	}).Should(Succeed())
	fmt.Fprintln(os.Stderr, "created namespace: "+ns)
}

func randomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxyz")

	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

func waitKindnet() error {
	stdout, stderr, err := kubectl("-n=kube-system", "get", "ds/kindnet", "-o", "json")
	if err != nil {
		return errors.New(string(stderr))
	}

	var ds appsv1.DaemonSet
	err = json.Unmarshal(stdout, &ds)
	if err != nil {
		return err
	}

	if ds.Status.NumberReady != 4 {
		return fmt.Errorf("numberReady is not 4: %d", ds.Status.NumberReady)
	}
	return nil
}

func isLvmdEnv() bool {
	return os.Getenv("LVMD") != ""
}

var _ = BeforeSuite(func() {
	By("Getting the directory path which contains some binaries")
	binDir = os.Getenv("BINDIR")
	Expect(binDir).ShouldNot(BeEmpty())
	fmt.Println("This test uses the binaries under " + binDir)

	if !isLvmdEnv() {
		By("Waiting for kindnet to get ready")
		// Because kindnet will crash. we need to confirm its readiness twice.
		Eventually(waitKindnet).Should(Succeed())
		time.Sleep(5 * time.Second)
		Eventually(waitKindnet).Should(Succeed())
	}

	SetDefaultEventuallyTimeout(5 * time.Minute)

	By("Waiting for mutating webhook to get ready")
	podYAML := `apiVersion: v1
kind: Pod
metadata:
  name: ubuntu
  labels:
    app.kubernetes.io/name: ubuntu
spec:
  containers:
    - name: ubuntu
      image: quay.io/cybozu/ubuntu:20.04
      command: ["/usr/local/bin/pause"]
`
	Eventually(func() error {
		_, stderr, err := kubectlWithInput([]byte(podYAML), "apply", "-f", "-")
		if err != nil {
			return errors.New(string(stderr))
		}
		return nil
	}).Should(Succeed())
	stdout, stderr, err := kubectlWithInput([]byte(podYAML), "delete", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
})

var _ = Describe("TopoLVM", func() {
	Context("hook", testHook)
	Context("topolvm-node", testNode)
	Context("scheduler", testScheduler)
	Context("metrics", testMetrics)
	Context("publish", testPublishVolume)
	Context("e2e", testE2E)
	Context("multiple-vg", testMultipleVolumeGroups)
	if !isLvmdEnv() {
		Context("cleanup", testCleanup)
	}
	Context("CSI sanity", func() {
		baseDir := "/var/lib/kubelet/plugins/topolvm.cybozu.com/"
		if !isLvmdEnv() {
			baseDir = "/tmp/topolvm/worker1/plugins/topolvm.cybozu.com/"
			It("should add node selector to node DaemonSet for CSI test", func() {
				_, _, err := kubectl("delete", "nodes", "topolvm-e2e-worker2")
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(func() error {
					var ds appsv1.DaemonSet
					stdout, _, err := kubectl("get", "-n", "topolvm-system", "ds", "node", "-o", "json")
					if err != nil {
						return err
					}
					err = json.Unmarshal(stdout, &ds)
					if err != nil {
						return err
					}
					if ds.Status.NumberAvailable != 1 {
						return errors.New("node daemonset is not ready")
					}
					return nil
				}).Should(Succeed())
			})
		}

		tc := sanity.NewTestConfig()
		tc.Address = baseDir + "/node/csi-topolvm.sock"
		tc.ControllerAddress = baseDir + "/controller/csi-topolvm.sock"
		tc.TargetPath = baseDir + "/node/mountdir"
		tc.StagingPath = baseDir + "/node/stagingdir"
		tc.TestVolumeSize = 1073741824
		tc.IDGen = &sanity.DefaultIDGenerator{}
		sanity.GinkgoTest(&tc)
	})
})
