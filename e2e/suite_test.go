package e2e

import (
	_ "embed"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var kubectlPath string
var skipMessageForStorageCapacity string = "skip because current environment is storage capacity"

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
	_, err := kubectl("create", "namespace", ns)
	Expect(err).ShouldNot(HaveOccurred())
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
	var ds appsv1.DaemonSet
	err := getObjects(&ds, "ds", "-n", "kube-system", "kindnet")
	if err != nil {
		return err
	}

	if ds.Status.NumberReady != 4 {
		return fmt.Errorf("numberReady is not 4: %d", ds.Status.NumberReady)
	}
	return nil
}

func isDaemonsetLvmdEnvSet() bool {
	return os.Getenv("DAEMONSET_LVMD") != ""
}

func isStorageCapacity() bool {
	return os.Getenv("STORAGE_CAPACITY") == "true"
}

func skipIfDaemonsetLvmd() {
	if isDaemonsetLvmdEnvSet() {
		Skip("skip because current environment is daemonset lvmd")
	}
}

func getDaemonsetLvmdNodeName() string {
	var nodes corev1.NodeList
	err := getObjects(&nodes, "nodes")
	Expect(err).ShouldNot(HaveOccurred())
	Expect(nodes.Items).Should(HaveLen(1))
	return nodes.Items[0].Name
}

//go:embed testdata/pause-pod.yaml
var pausePodYAML []byte

var _ = BeforeSuite(func() {
	By("Getting kubectl binary")
	kubectlPath = os.Getenv("KUBECTL")
	Expect(kubectlPath).ShouldNot(BeEmpty())
	fmt.Println("This test uses a kubectl at " + kubectlPath)

	if !isDaemonsetLvmdEnvSet() {
		By("Waiting for kindnet to get ready")
		// Because kindnet will crash. we need to confirm its readiness twice.
		Eventually(waitKindnet).Should(Succeed())
		time.Sleep(5 * time.Second)
		Eventually(waitKindnet).Should(Succeed())
	}
	SetDefaultEventuallyTimeout(5 * time.Minute)

	By("Waiting for mutating webhook to get ready")
	Eventually(func() error {
		_, err := kubectlWithInput(pausePodYAML, "apply", "-f", "-")
		if err != nil {
			return err
		}
		return nil
	}).Should(Succeed())
	_, err := kubectlWithInput(pausePodYAML, "delete", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred())
})

var _ = Describe("TopoLVM", func() {
	if os.Getenv("SANITY_TEST_WITH_THIN_DEVICECLASS") != "true" {
		Context("topolvm-node", testNode)
		Context("topolvm-controller", testController)
		Context("scheduling", testScheduling)
		Context("metrics", testMetrics)
		Context("publish", testPublishVolume)
		Context("ReadWriteOncePod", testReadWriteOncePod)
		Context("e2e", testE2E)
		Context("multiple-vg", testMultipleVolumeGroups)
		Context("lvcreate-options", testLVCreateOptions)
		Context("thin-provisioning", testThinProvisioning)
		Context("thin-snapshot-restore", testSnapRestore)
		Context("thin-volume-cloning", testPVCClone)
		Context("logical-volume", testLogicalVolume)
		Context("cleanup", testCleanup)
	}
	Context("CSI sanity", testSanity)
})
