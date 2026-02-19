package e2e

import (
	"path"

	"github.com/kubernetes-csi/csi-test/v5/pkg/sanity"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
)

func init() {
	// TopoLVM claims having capabilities for snapshot and clone
	// but they are not implemented for thick volumes.
	// Note: specify as narrow conditions as possible to avoid matching other than expected.
	skipSpecs = append(skipSpecs,
		"Thick LVM.*CreateVolume.*source snapshot",
		"Thick LVM.*CreateVolume.*source volume",
		"Thick LVM.*CreateSnapshot",
		"Thick LVM.*DeleteSnapshot",
	)
}

func testSanity() {
	var tc, thinTC sanity.TestConfig

	BeforeEach(func() {
		_, err := kubectl("delete", "nodes", "topolvm-e2e-worker2", "--ignore-not-found")
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectl("delete", "nodes", "topolvm-e2e-worker3", "--ignore-not-found")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func(g Gomega) int {
			var nodePods corev1.PodList
			err := getObjects(&nodePods, "pod", "-n", "topolvm-system", "-l", "app.kubernetes.io/component=node")
			g.Expect(err).ShouldNot(HaveOccurred())
			return len(nodePods.Items)
		}).Should(Equal(1))

		Eventually(func(g Gomega) {
			var pods corev1.PodList
			err := getObjects(&pods, "pod", "-n", "topolvm-system", "-l", "app.kubernetes.io/component=controller")
			g.Expect(err).ShouldNot(HaveOccurred())
			for _, pod := range pods.Items {
				g.Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
				g.Expect(pod.Spec.NodeName).NotTo(BeElementOf("topolvm-e2e-worker2", "topolvm-e2e-worker3"))
			}
		}).Should(Succeed())

		baseDir := "/var/lib/kubelet/plugins/" + topolvm.GetPluginName() + "/"
		if nonControlPlaneNodeCount > 0 {
			baseDir = "/tmp/topolvm/worker1/plugins/" + topolvm.GetPluginName() + "/"
		}

		tc = sanity.NewTestConfig()
		tc.Address = path.Join(baseDir, "/node/csi-topolvm.sock")
		tc.ControllerAddress = path.Join(baseDir, "/controller/csi-topolvm.sock")
		tc.TargetPath = path.Join(baseDir, "/node/mountdir")
		tc.StagingPath = path.Join(baseDir, "/node/stagingdir")
		tc.TestVolumeSize = 1073741824
		tc.IDGen = &sanity.DefaultIDGenerator{}
		tc.CheckPath = func(path string) (sanity.PathKind, error) {
			_, err := kubectl("exec", "-n", "topolvm-system", "daemonset/topolvm-node", "--", "test", "-f", path)
			if err == nil {
				return sanity.PathIsFile, nil
			}
			_, err = kubectl("exec", "-n", "topolvm-system", "daemonset/topolvm-node", "--", "test", "-d", path)
			if err == nil {
				return sanity.PathIsDir, nil
			}
			_, err = kubectl("exec", "-n", "topolvm-system", "daemonset/topolvm-node", "--", "test", "!", "-e", path)
			if err == nil {
				return sanity.PathIsNotFound, nil
			}
			_, err = kubectl("exec", "-n", "topolvm-system", "daemonset/topolvm-node", "--", "test", "-e", path)
			return sanity.PathIsOther, err
		}

		thinTC = tc
		// csi.storage.k8s.io/fstype=xfs,topolvm.(io|cybozu.com)/device-class=thin
		thinTC.TestVolumeParameters = map[string]string{
			"csi.storage.k8s.io/fstype": "xfs",
			topolvm.GetDeviceClassKey(): "thin",
		}
	})

	Context("Thick LVM", func() {
		sanity.GinkgoTest(&tc)
	})
	Context("Thin LVM", func() {
		sanity.GinkgoTest(&thinTC)
	})
}
