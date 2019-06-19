package microtest

import (
	"context"
	"net"

	"github.com/cybozu-go/topolvm/csi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

var _ = Describe("CSI Node test", func() {
	lvmdSocket := "/tmp/topolvm/lvmd.sock"

	It("should be worked NodePublishVolume successfully to create fs", func() {
		volumeID := "csi-node-test-fs"
		dialer := &net.Dialer{}
		dialFunc := func(ctx context.Context, a string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", a)
		}
		conn, err := grpc.Dial(lvmdSocket, grpc.WithInsecure(), grpc.WithContextDialer(dialFunc))
		Expect(err).ShouldNot(HaveOccurred())
		defer conn.Close()

		By("creating Filesystem volume")
		mountVolCap := &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{FsType: "btrfs"},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		}

		stdout, stderr, err := kubectl("get", "nodes", "--template={{(index .items 0).metadata.name}}")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		mountTargetPath := "/mnt/csi-node-test"
		req := &csi.NodePublishVolumeRequest{
			PublishContext:   map[string]string{},
			TargetPath:       mountTargetPath,
			VolumeCapability: mountVolCap,
			VolumeId:         volumeID,
		}
		ns := csi.NewNodeService(string(stdout), conn)
		resp, err := ns.NodePublishVolume(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resp).ShouldNot(BeNil())

		By("creating Filesystem volume again to check idempotency")
		resp, err = ns.NodePublishVolume(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resp).ShouldNot(BeNil())

		By("creating volume on same target path, but requested volume and existing one are different")
		_, err = ns.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
			PublishContext: map[string]string{},
			TargetPath:     mountTargetPath,
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{FsType: "ext4"},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
			VolumeId: volumeID,
		})
		Expect(err).Should(HaveOccurred())

		By("deleting the volume")
		unpubReq := csi.NodeUnpublishVolumeRequest{
			VolumeId:   volumeID,
			TargetPath: mountTargetPath,
		}
		unpubResp, err := ns.NodeUnpublishVolume(context.Background(), &unpubReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(unpubResp).ShouldNot(BeNil())

		By("deleting the volume again to check idempotency")
		unpubResp, err = ns.NodeUnpublishVolume(context.Background(), &unpubReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(unpubResp).ShouldNot(BeNil())
	})

	It("should be worked NodePublishVolume successfully to create block", func() {
		volumeID := "csi-node-test-block"
		dialer := &net.Dialer{}
		dialFunc := func(ctx context.Context, a string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", a)
		}
		conn, err := grpc.Dial(lvmdSocket, grpc.WithInsecure(), grpc.WithContextDialer(dialFunc))
		Expect(err).ShouldNot(HaveOccurred())
		defer conn.Close()

		By("creating raw block volume")
		blockVolCap := &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Block{
				Block: &csi.VolumeCapability_BlockVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		}

		stdout, stderr, err := kubectl("get", "nodes", "--template={{(index .items 0).metadata.name}}")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		deviceTargetPath := "/dev/csi-node-test"
		req := &csi.NodePublishVolumeRequest{
			PublishContext:   map[string]string{},
			TargetPath:       deviceTargetPath,
			VolumeCapability: blockVolCap,
			VolumeId:         volumeID,
		}
		ns := csi.NewNodeService(string(stdout), conn)
		resp, err := ns.NodePublishVolume(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resp).ShouldNot(BeNil())

		By("creating raw block volume again to check idempotency")
		resp, err = ns.NodePublishVolume(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resp).ShouldNot(BeNil())

		By("creating volume on same target path, but requested volume and existing one are different")
		_, err = ns.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
			PublishContext: map[string]string{},
			TargetPath:     "/dev/csi-node-test-invalid",
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Block{
					Block: &csi.VolumeCapability_BlockVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
			VolumeId: volumeID,
		})
		Expect(err).Should(HaveOccurred())

		By("deleting the volume")
		unpubReq := csi.NodeUnpublishVolumeRequest{
			VolumeId:   volumeID,
			TargetPath: deviceTargetPath,
		}
		unpubResp, err := ns.NodeUnpublishVolume(context.Background(), &unpubReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(unpubResp).ShouldNot(BeNil())

		By("deleting the volume again to check idempotency")
		unpubResp, err = ns.NodeUnpublishVolume(context.Background(), &unpubReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(unpubResp).ShouldNot(BeNil())
	})
})
