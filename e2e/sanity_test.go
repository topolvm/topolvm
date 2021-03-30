package e2e

import (
	"encoding/json"
	"errors"

	"github.com/kubernetes-csi/csi-test/v4/pkg/sanity"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
)

func testSanity() {
	baseDir := "/tmp/topolvm/worker1/plugins/topolvm.cybozu.com/"
	if isDaemonsetLvmdEnvSet() {
		baseDir = "/var/lib/kubelet/plugins/topolvm.cybozu.com/"
	}

	It("should add node selector to node DaemonSet for CSI test", func() {
		// skip test when using minikube because it doesn't need to delete a worker.
		skipIfDaemonsetLvmd()
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

	tc := sanity.NewTestConfig()
	tc.Address = baseDir + "/node/csi-topolvm.sock"
	tc.ControllerAddress = baseDir + "/controller/csi-topolvm.sock"
	tc.TargetPath = baseDir + "/node/mountdir"
	tc.StagingPath = baseDir + "/node/stagingdir"
	tc.TestVolumeSize = 1073741824
	tc.IDGen = &sanity.DefaultIDGenerator{}
	sanity.GinkgoTest(&tc)
}
