package e2e

import (
	"context"
	"fmt"
	"net"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-test/pkg/sanity"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

var _ = Describe("CSI sanity test", func() {
	sanity.GinkgoTest(&sanity.Config{
		Address:           "/tmp/topolvm/worker1/plugins/topolvm.cybozu.com/node/csi-topolvm.sock",
		ControllerAddress: "/tmp/topolvm/worker1/plugins/topolvm.cybozu.com/controller/csi-topolvm.sock",
		TargetPath:        "/tmp/topolvm/worker1/plugins/topolvm.cybozu.com/node/mountdir",
		StagingPath:       "/tmp/topolvm/worker1/plugins/topolvm.cybozu.com/node/stagingdir",
		TestVolumeSize:    1073741824,
	})
})

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

func (c cleanup) unpublishVolumes(nc csi.NodeClient) {
	By("[cleanup] unpublishVolumes")
	for volumeID, targetPath := range c.volumes {
		req := &csi.NodeUnpublishVolumeRequest{
			VolumeId:   volumeID,
			TargetPath: targetPath,
		}
		_, err := nc.NodeUnpublishVolume(context.Background(), req)
		if err != nil {
			fmt.Printf("failed to unpublish volume: %v", req)
		}
	}
	c.volumes = nil
}

var _ = Describe("Additional CSI sanity test", func() {
	var (
		cl   cleanup
		nc   csi.NodeClient
		conn *grpc.ClientConn
	)
	nodeSocket := "/tmp/topolvm/worker1/plugins/topolvm.cybozu.com/node/csi-topolvm.sock"

	BeforeEach(func() {
		dialer := &net.Dialer{}
		dialFunc := func(ctx context.Context, a string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", a)
		}
		var err error
		conn, err = grpc.Dial(nodeSocket, grpc.WithInsecure(), grpc.WithContextDialer(dialFunc))
		Expect(err).ShouldNot(HaveOccurred())

		nc = csi.NewNodeClient(conn)
	})

	AfterEach(func() {
		cl.unpublishVolumes(nc)
		if conn != nil {
			conn.Close()
			conn = nil
		}
	})

	It("should publish filesystem", func() {
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
		resp, err := nc.NodePublishVolume(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resp).ShouldNot(BeNil())

		By("publishing Filesystem volume again to check idempotency")
		resp, err = nc.NodePublishVolume(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resp).ShouldNot(BeNil())

		By("publishing volume on same target path, but requested volume and existing one are different")
		_, err = nc.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
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

		By("unpublishing the volume")
		unpubReq := csi.NodeUnpublishVolumeRequest{
			VolumeId:   volumeID,
			TargetPath: mountTargetPath,
		}
		unpubResp, err := nc.NodeUnpublishVolume(context.Background(), &unpubReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(unpubResp).ShouldNot(BeNil())

		By("unpublishing the volume again to check idempotency")
		unpubResp, err = nc.NodeUnpublishVolume(context.Background(), &unpubReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(unpubResp).ShouldNot(BeNil())

		cl.unregister(volumeID, mountTargetPath)
	})

	It("should be worked NodePublishVolume successfully to create a block device", func() {
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
		resp, err := nc.NodePublishVolume(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resp).ShouldNot(BeNil())

		By("creating raw block volume again to check idempotency")
		resp, err = nc.NodePublishVolume(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(resp).ShouldNot(BeNil())

		By("creating volume on the same target path, but requested volume and existing one are different")
		_, err = nc.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
			PublishContext:   map[string]string{},
			TargetPath:       deviceTargetPath,
			VolumeCapability: blockVolCap,
			VolumeId:         volumeID + "-invalid",
		})
		Expect(err).Should(HaveOccurred())

		By("unpublishing the volume")
		unpubReq := csi.NodeUnpublishVolumeRequest{
			VolumeId:   volumeID,
			TargetPath: deviceTargetPath,
		}
		unpubResp, err := nc.NodeUnpublishVolume(context.Background(), &unpubReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(unpubResp).ShouldNot(BeNil())

		By("deleting the volume again to check idempotency")
		unpubResp, err = nc.NodeUnpublishVolume(context.Background(), &unpubReq)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(unpubResp).ShouldNot(BeNil())

		cl.unregister(volumeID, deviceTargetPath)
	})
})
