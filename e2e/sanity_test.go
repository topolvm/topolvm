package e2e

import (
	"encoding/json"
	"errors"
	"path"
	"time"

	"github.com/kubernetes-csi/csi-test/v4/pkg/sanity"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	appsv1 "k8s.io/api/apps/v1"
)

func testSanity() {
	baseDir := "/tmp/topolvm/worker1/plugins/topolvm.cybozu.com/"
	if isDaemonsetLvmdEnvSet() {
		baseDir = "/var/lib/kubelet/plugins/topolvm.cybozu.com/"
	}

	It("should add node selector to node DaemonSet for CSI test", func() {
		// Skip deleting node because there is just one node in daemonset lvmd test environment.
		skipIfDaemonsetLvmd()
		_, _, err := kubectl("delete", "nodes", "topolvm-e2e-worker2")
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(func() error {
			var ds appsv1.DaemonSet
			stdout, _, err := kubectl("get", "-n", "topolvm-system", "ds", "topolvm-node", "-o", "json")
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
	tc.Address = path.Join(baseDir, "/node/csi-topolvm.sock")
	tc.ControllerAddress = path.Join(baseDir, "/controller/csi-topolvm.sock")
	tc.TargetPath = path.Join(baseDir, "/node/mountdir")
	tc.StagingPath = path.Join(baseDir, "/node/stagingdir")
	tc.TestVolumeSize = 1073741824
	tc.IDGen = &sanity.DefaultIDGenerator{}
	tc.DialOptions = append(tc.DialOptions, grpc.WithTimeout(5*time.Minute))
	tc.CheckPath = func(path string) (sanity.PathKind, error) {
		_, _, err := kubectl("exec", "-n", "topolvm-system", "daemonset/topolvm-node", "--", "test", "-f", path)
		if err == nil {
			return sanity.PathIsFile, nil
		}
		_, _, err = kubectl("exec", "-n", "topolvm-system", "daemonset/topolvm-node", "--", "test", "-d", path)
		if err == nil {
			return sanity.PathIsDir, nil
		}
		_, _, err = kubectl("exec", "-n", "topolvm-system", "daemonset/topolvm-node", "--", "test", "!", "-e", path)
		if err == nil {
			return sanity.PathIsNotFound, nil
		}
		_, _, err = kubectl("exec", "-n", "topolvm-system", "daemonset/topolvm-node", "--", "test", "-e", path)
		return sanity.PathIsOther, err
	}
	sanity.GinkgoTest(&tc)
}
