package e2e

import (
	"os"
	"path"

	"github.com/kubernetes-csi/csi-test/v5/pkg/sanity"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	appsv1 "k8s.io/api/apps/v1"
)

func testSanity() {
	BeforeEach(func() {
		_, err := kubectl("delete", "nodes", "topolvm-e2e-worker2", "--ignore-not-found")
		Expect(err).ShouldNot(HaveOccurred())
		_, err = kubectl("delete", "nodes", "topolvm-e2e-worker3", "--ignore-not-found")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func(g Gomega) {
			var ds appsv1.DaemonSet
			err := getObjects(&ds, "ds", "-n", "topolvm-system", "topolvm-node")
			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(ds.Status.NumberAvailable).To(BeEquivalentTo(1))
		}).Should(Succeed())
	})

	baseDir := "/tmp/topolvm/worker1/plugins/" + topolvm.GetPluginName() + "/"
	if isDaemonsetLvmdEnvSet() {
		baseDir = "/var/lib/kubelet/plugins/" + topolvm.GetPluginName() + "/"
	}

	tc := sanity.NewTestConfig()
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

	if os.Getenv("SANITY_TEST_WITH_THIN_DEVICECLASS") == "true" {
		// csi.storage.k8s.io/fstype=xfs,topolvm.(io|cybozu.com)/device-class=thin
		volParams := make(map[string]string)
		volParams["csi.storage.k8s.io/fstype"] = "xfs"
		volParams[topolvm.GetDeviceClassKey()] = "thin"
		tc.TestVolumeParameters = volParams
	}
	sanity.GinkgoTest(&tc)
}
