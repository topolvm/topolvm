package e2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/kubernetes-csi/csi-test/pkg/sanity"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
)

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

var _ = BeforeSuite(func() {
	By("Waiting for mutating webhook to get ready")
	// Because kindnet will crash. we need to confirm its readiness twice.
	Eventually(waitKindnet).Should(Succeed())
	time.Sleep(5 * time.Second)
	Eventually(waitKindnet).Should(Succeed())

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
	Context("CSI sanity", func() {
		sanity.GinkgoTest(&sanity.Config{
			Address:           "/tmp/topolvm/worker1/plugins/topolvm.cybozu.com/node/csi-topolvm.sock",
			ControllerAddress: "/tmp/topolvm/worker1/plugins/topolvm.cybozu.com/controller/csi-topolvm.sock",
			TargetPath:        "/tmp/topolvm/worker1/plugins/topolvm.cybozu.com/node/mountdir",
			StagingPath:       "/tmp/topolvm/worker1/plugins/topolvm.cybozu.com/node/stagingdir",
			TestVolumeSize:    1073741824,
		})

		It("should remove node selector from node DaemonSet after CSI test", func() {
			var ds appsv1.DaemonSet
			stdout, _, err := kubectl("get", "-n", "topolvm-system", "ds", "node", "-o", "json")
			Expect(err).ShouldNot(HaveOccurred())
			err = json.Unmarshal(stdout, &ds)
			Expect(err).ShouldNot(HaveOccurred())
			ds.Spec.Template.Spec.NodeSelector = nil
			data, _ := json.Marshal(ds)
			_, _, err = kubectlWithInput(data, "apply", "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				var ds appsv1.DaemonSet
				stdout, _, err := kubectl("get", "-n", "topolvm-system", "ds", "node", "-o", "json")
				if err != nil {
					return nil
				}
				err = json.Unmarshal(stdout, &ds)
				if err != nil {
					return nil
				}
				if ds.Status.NumberAvailable != 3 {
					return errors.New("node daemonset is not ready")
				}
				return nil
			})
		})
	})
	Context("hook", testHook)
	Context("lvmetrics", testLvmetrics)
	Context("scheduler", testScheduler)
	Context("metrics", testMetrics)
	Context("publish", testPublishVolume)
	Context("e2e", testE2E)
	Context("cleanup", testCleanup)
})
