package e2e

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/csi"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

//go:embed testdata/publish/lv-template.yaml
var lvTemplateYAML string

//go:embed testdata/publish/pod-with-mount-option-pvc.yaml
var podWithMountOptionPVCYAML []byte

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

func testPublishVolume() {
	var (
		cl   cleanup
		nc   csi.NodeClient
		conn *grpc.ClientConn
	)

	nodeSocket := "/tmp/topolvm/worker1/plugins/topolvm.cybozu.com/node/csi-topolvm.sock"
	if isDaemonsetLvmdEnvSet() {
		nodeSocket = "/var/lib/kubelet/plugins/topolvm.cybozu.com/node/csi-topolvm.sock"
	}

	var cc CleanupContext

	BeforeEach(func() {
		cc = commonBeforeEach()

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

		commonAfterEach(cc)
	})

	It("should publish filesystem", func() {
		mountTargetPath := "/mnt/csi-node-test"

		nodeName := "topolvm-e2e-worker"
		if isDaemonsetLvmdEnvSet() {
			nodeName = getDaemonsetLvmdNodeName()
		}

		By("creating a logical volume resource")
		name := "csi-node-test-fs"
		lvYaml := []byte(fmt.Sprintf(lvTemplateYAML, name, name, nodeName))
		_, _, err := kubectlWithInput(lvYaml, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		var volumeID string
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "logicalvolume", name, "-o", "yaml")
			if err != nil {
				return fmt.Errorf("failed to get logical volume. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var lv topolvmv1.LogicalVolume
			err = yaml.Unmarshal(stdout, &lv)
			if err != nil {
				return err
			}

			if len(lv.Status.VolumeID) == 0 {
				return errors.New("VolumeID is not set")
			}

			if lv.Labels == nil {
				return errors.New("logical volume label is nil")
			}

			if lv.Labels[topolvm.CreatedbyLabelKey] != topolvm.CreatedbyLabelValue {
				return fmt.Errorf("logical volume label %q is not eqaual %q", topolvm.CreatedbyLabelKey, topolvm.CreatedbyLabelValue)
			}

			volumeID = lv.Status.VolumeID
			return nil
		}).Should(Succeed())

		cl.register(volumeID, mountTargetPath)

		By("creating Filesystem volume")
		mountVolCap := &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{FsType: "xfs"},
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

		By("cleaning logicalvolume")
		stdout, stderr, err := kubectl("delete", "logicalvolume", "csi-node-test-fs")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	})

	It("should be worked NodePublishVolume successfully to create a block device", func() {
		deviceTargetPath := "/dev/csi-node-test"

		By("creating a logical volume resource")
		nodeName := "topolvm-e2e-worker"
		if isDaemonsetLvmdEnvSet() {
			nodeName = getDaemonsetLvmdNodeName()
		}

		name := "csi-node-test-block"
		lvYaml := []byte(fmt.Sprintf(lvTemplateYAML, name, name, nodeName))
		_, _, err := kubectlWithInput(lvYaml, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		var volumeID string
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "logicalvolume", name, "-o", "yaml")
			if err != nil {
				return fmt.Errorf("failed to get logical volume. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var lv topolvmv1.LogicalVolume
			err = yaml.Unmarshal(stdout, &lv)
			if err != nil {
				return err
			}

			if len(lv.Status.VolumeID) == 0 {
				return errors.New("VolumeID is not set")
			}
			volumeID = lv.Status.VolumeID
			return nil
		}).Should(Succeed())

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

		By("cleaning logicalvolume")
		stdout, stderr, err := kubectl("delete", "logicalvolume", "csi-node-test-block")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	})

	It("should publish filesystem with mount option", func() {
		By("creating a PVC and Pod")
		_, _, err := kubectlWithInput(podWithMountOptionPVCYAML, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "pod", "ubuntu-mount-option", "-o", "yaml")
			if err != nil {
				return fmt.Errorf("failed to get pod. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var pod corev1.Pod
			err = yaml.Unmarshal(stdout, &pod)
			if err != nil {
				return err
			}

			if pod.Status.Phase != corev1.PodRunning {
				return errors.New("Pod is not running")
			}

			return nil
		}).Should(Succeed())

		By("check mount option")
		stdout, stderr, err := kubectl("get", "pvc", "topo-pvc-mount-option", "-o", "yaml")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		var pvc corev1.PersistentVolumeClaim
		err = yaml.Unmarshal(stdout, &pvc)
		Expect(err).ShouldNot(HaveOccurred())

		stdout, stderr, err = execAtLocal("cat", nil, "/proc/mounts")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		var isExistingOption bool
		lines := strings.Split(string(stdout), "\n")
		for _, line := range lines {
			if strings.Contains(line, pvc.Spec.VolumeName) {
				fields := strings.Split(line, " ")
				Expect(len(fields)).To(Equal(6))
				options := strings.Split(fields[3], ",")
				for _, option := range options {
					if option == "debug" {
						isExistingOption = true
					}
				}
			}
		}
		Expect(isExistingOption).Should(BeTrue())

		By("cleaning pvc/pod")
		stdout, stderr, err = kubectlWithInput(podWithMountOptionPVCYAML, "delete", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	})

	It("validate mount options", func() {
		mountTargetPath := "/mnt/csi-node-test"

		By("creating a logical volume resource")
		nodeName := "topolvm-e2e-worker"
		if isDaemonsetLvmdEnvSet() {
			nodeName = getDaemonsetLvmdNodeName()
		}

		name := "csi-node-test-fs"
		lvYaml := []byte(fmt.Sprintf(lvTemplateYAML, name, name, nodeName))
		_, _, err := kubectlWithInput(lvYaml, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		var volumeID string
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "logicalvolume", name, "-o", "yaml")
			if err != nil {
				return fmt.Errorf("failed to get logical volume. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var lv topolvmv1.LogicalVolume
			err = yaml.Unmarshal(stdout, &lv)
			if err != nil {
				return err
			}

			if len(lv.Status.VolumeID) == 0 {
				return errors.New("VolumeID is not set")
			}
			volumeID = lv.Status.VolumeID
			return nil
		}).Should(Succeed())

		cl.register(volumeID, mountTargetPath)

		By("mount option \"rw\" is specified even though read only mode is specified")
		mountVolCap := &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{
					FsType:     "xfs",
					MountFlags: []string{"rw"},
				},
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
			Readonly:         true,
		}
		_, err = nc.NodePublishVolume(context.Background(), req)
		Expect(err).Should(HaveOccurred())

		By("cleaning logicalvolume")
		stdout, stderr, err := kubectl("delete", "logicalvolume", "csi-node-test-fs")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	})
}
