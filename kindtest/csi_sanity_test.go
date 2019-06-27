package kindtest

import (
	"context"
	"fmt"
	"net"

	"github.com/cybozu-go/topolvm/csi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

type cleanup struct {
	// key is volumeID, value is target path
	volumes map[string]string
}

func (c cleanup) register(volumeID, targetPath string) {
	By("[cleanup] register")
	if c.volumes == nil {
		c.volumes = make(map[string]string)
	}
	c.volumes[volumeID] = targetPath
}

func (c cleanup) unregister(volumeID, targetPath string) {
	By("[cleanup] unregister")
	if c.volumes != nil {
		delete(c.volumes, volumeID)
	}
}

func (c cleanup) unpublishVolumes(ns csi.NodeServer) {
	By("[cleanup] unpublishVolumes")
	for volumeID, targetPath := range c.volumes {
		req := &csi.NodeUnpublishVolumeRequest{
			VolumeId:   volumeID,
			TargetPath: targetPath,
		}
		_, err := ns.NodeUnpublishVolume(context.Background(), req)
		if err != nil {
			fmt.Printf("failed to unpublish volume: %v", req)
		}
	}
}

var _ = Describe("CSI sanity test", func() {
	var (
		cl   cleanup
		conn *grpc.ClientConn
		ns   csi.NodeServer
	)
	lvmdSocket := "/tmp/topolvm/lvmd.sock"

	BeforeEach(func() {
		dialer := &net.Dialer{}
		dialFunc := func(ctx context.Context, a string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", a)
		}
		var err error
		conn, err = grpc.Dial(lvmdSocket, grpc.WithInsecure(), grpc.WithContextDialer(dialFunc))
		Expect(err).ShouldNot(HaveOccurred())

		stdout, stderr, err := kubectl("get", "nodes", "--template={{(index .items 0).metadata.name}}")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		ns = csi.NewNodeService(string(stdout), conn)
	})

	AfterEach(func() {
		cl.unpublishVolumes(ns)
		conn.Close()
	})

	It("should be worked NodePublishVolume successfully to create fs", func() {
		volumeID := "csi-node-test-fs"
		mountTargetPath := "/mnt/csi-node-test"
		cl.register(volumeID, mountTargetPath)

		By("creating Filesystem volume")
		mountVolCap := &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{FsType: "btrfs"},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		}

		req := &csi.NodePublishVolumeRequest{
			PublishContext:   map[string]string{},
			TargetPath:       mountTargetPath,
			VolumeCapability: mountVolCap,
			VolumeId:         volumeID,
		}
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

		cl.unregister(volumeID, mountTargetPath)
	})

	It("should be worked NodePublishVolume successfully to create block", func() {
		deviceTargetPath := "/dev/csi-node-test"
		volumeID := "csi-node-test-block"
		cl.register(volumeID, deviceTargetPath)

		By("creating raw block volume")
		blockVolCap := &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Block{
				Block: &csi.VolumeCapability_BlockVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		}

		req := &csi.NodePublishVolumeRequest{
			PublishContext:   map[string]string{},
			TargetPath:       deviceTargetPath,
			VolumeCapability: blockVolCap,
			VolumeId:         volumeID,
		}
		resp, err := ns.NodePublishVolume(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resp).ShouldNot(BeNil())

		By("creating raw block volume again to check idempotency")
		resp, err = ns.NodePublishVolume(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resp).ShouldNot(BeNil())

		By("creating volume on same target path, but requested volume and existing one are different")
		_, err = ns.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
			PublishContext:   map[string]string{},
			TargetPath:       deviceTargetPath,
			VolumeCapability: blockVolCap,
			VolumeId:         volumeID + "-invalid",
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

		cl.unregister(volumeID, deviceTargetPath)
	})
})
