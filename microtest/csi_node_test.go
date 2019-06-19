package microtest

import (
	"context"
	"net"

	"github.com/cybozu-go/topolvm/csi"
	"github.com/cybozu-go/topolvm/lvmd"
	"github.com/cybozu-go/topolvm/lvmd/command"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

var _ = Describe("CSI Node test", func() {
	csiNodeSocket := "/var/snap/microk8s/common/var/lib/kubelet/plugins/topolvm.cybozu.com/node/csi-topolvm.sock"

	It("should worked NodePublishVolume successfully", func() {
		stdout, stderr, err := kubectl("get ", "nodes", "--template={{(index .items 0).metadata.name}}")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		// lvcreate
		volumeID := "vol-test"
		vgName := "vol-test"
		loop, err := lvmd.MakeLoopbackVG(vgName)
		Expect(err).ShouldNot(HaveOccurred())
		defer lvmd.CleanLoopbackVG(loop, vgName)
		_, err = command.FindVolumeGroup(vgName)
		Expect(err).ShouldNot(HaveOccurred())
		err = command.CallLVM("lvcreate", "-n", volumeID, "-L", "1G", vgName)
		Expect(err).ShouldNot(HaveOccurred())

		dialer := &net.Dialer{}
		dialFunc := func(ctx context.Context, a string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", a)
		}
		conn, err := grpc.Dial(csiNodeSocket, grpc.WithInsecure(), grpc.WithContextDialer(dialFunc))
		Expect(err).ShouldNot(HaveOccurred())
		defer conn.Close()

		By("Creating Filesystem volume")
		stdVolCap := &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{FsType: "btrfs"},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		}

		req := &NodePublishVolumeRequest{
			PublishContext:   map[string]string{},
			TargetPath:       "/test/path",
			VolumeCapability: stdVolCap,
			VolumeId:         "vol-test",
		}
		ns := csi.NewNodeService(string(stdout), conn)
		resp, err := ns.NodePublishVolume(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resp).ShouldNot(BeNil())

		By("Creating Filesystem volume again")
		// TODO
	})
})
