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

// skipSpecs holds regexp strings that are used to skip tests.
// It is intended to be setup by init function on each test file if necessary.
var skipSpecs []string

// They are initialized in BeforeSuite, it should be used only in Ginkgo nodes.
var kubectlPath string
var nonControlPlaneNodeCount int

func TestMtest(t *testing.T) {
	if os.Getenv("E2ETEST") == "" {
		t.Skip("Run under e2e/")
	}

	RegisterFailHandler(Fail)

	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultEventuallyTimeout(time.Minute)

	suiteConfig, _ := GinkgoConfiguration()
	suiteConfig.SkipStrings = append(suiteConfig.SkipStrings, skipSpecs...)

	RunSpecs(t, "Test on sanity", suiteConfig)
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

func waitKindnet(g Gomega) {
	var nodes corev1.NodeList
	err := getObjects(&nodes, "node")
	g.Expect(err).ShouldNot(HaveOccurred())
	var ds appsv1.DaemonSet
	err = getObjects(&ds, "ds", "-n", "kube-system", "kindnet")
	if err != ErrObjectNotFound {
		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(ds.Status.NumberReady).To(BeEquivalentTo(len(nodes.Items)))
	}
}

func isStorageCapacity() bool {
	return os.Getenv("STORAGE_CAPACITY") == "true"
}

func skipIfStorageCapacity(reason ...string) {
	if isStorageCapacity() {
		msg := "skip because current environment is storage capacity"
		if len(reason) > 0 {
			msg += ": " + reason[0]
		}
		Skip(msg)
	}
}

func skipIfSingleNode() {
	if nonControlPlaneNodeCount == 0 {
		Skip("This test requires multiple nodes")
	}
}

//go:embed testdata/pause-pod.yaml
var pausePodYAML []byte

var _ = BeforeSuite(func() {
	By("Getting kubectl binary")
	kubectlPath = os.Getenv("KUBECTL")
	Expect(kubectlPath).ShouldNot(BeEmpty())
	fmt.Println("This test uses a kubectl at " + kubectlPath)

	By("Getting node count")
	var nodes corev1.NodeList
	err := getObjects(&nodes, "nodes", "-l=node-role.kubernetes.io/control-plane!=")
	Expect(err).Should(SatisfyAny(Not(HaveOccurred()), BeIdenticalTo(ErrObjectNotFound)))
	nonControlPlaneNodeCount = len(nodes.Items)

	By("Waiting for kindnet to get ready if necessary")
	// Because kindnet will crash. we need to confirm its readiness twice.
	Eventually(waitKindnet).Should(Succeed())
	time.Sleep(5 * time.Second)
	Eventually(waitKindnet).Should(Succeed())

	SetDefaultEventuallyTimeout(5 * time.Minute)

	By("Waiting for mutating webhook to get ready")
	Eventually(func() error {
		_, err := kubectlWithInput(pausePodYAML, "apply", "-f", "-")
		if err != nil {
			return err
		}
		return nil
	}).Should(Succeed())
	_, err = kubectlWithInput(pausePodYAML, "delete", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred())
})

var _ = Describe("TopoLVM", func() {
	Context("scheduling", testScheduling)
	Context("metrics", testMetrics)
	Context("mount option", testMountOption)
	Context("ReadWriteOncePod", testReadWriteOncePod)
	Context("e2e", testE2E)
	Context("multiple-vg", testMultipleVolumeGroups)
	Context("lvcreate-options", testLVCreateOptions)
	Context("thin-provisioning", testThinProvisioning)
	Context("thin-snapshot-restore", testSnapRestore)
	Context("thin-volume-cloning", testPVCClone)
	Context("logical-volume", testLogicalVolume)
	Context("node delete", testNodeDelete)
	Context("CSI sanity", testSanity)
})
