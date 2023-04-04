package e2e

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
)

//go:embed testdata/publish/lv-template.yaml
var lvTemplateYAML string

//go:embed testdata/publish/pod-with-mount-option-pvc.yaml
var podWithMountOptionPVCYAML []byte

type cleanup struct {
	// key is volumeID, value is target path
	volumes map[string]string
}

func (c *cleanup) register(volumeID, targetPath string) {
	By("[cleanup] register")
	if c.volumes == nil {
		c.volumes = make(map[string]string)
	}
	c.volumes[volumeID] = targetPath
}

func (c *cleanup) unregister(volumeID, targetPath string) {
	By("[cleanup] unregister")
	if c.volumes != nil {
		delete(c.volumes, volumeID)
	}
}

func (c *cleanup) unpublishVolumes(nc csi.NodeClient) {
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
	cl := &cleanup{}
	var (
		nc   csi.NodeClient
		conn *grpc.ClientConn
	)

	nodeSocket := "/tmp/topolvm/worker1/plugins/" + topolvm.GetPluginName() + "/node/csi-topolvm.sock"
	if isDaemonsetLvmdEnvSet() {
		nodeSocket = "/var/lib/kubelet/plugins/" + topolvm.GetPluginName() + "/node/csi-topolvm.sock"
	}

	var cc CleanupContext

	BeforeEach(func() {
		cc = commonBeforeEach()

		dialer := &net.Dialer{}
		dialFunc := func(ctx context.Context, a string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", a)
		}
		var err error
		conn, err = grpc.Dial(nodeSocket, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialFunc))
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

	It("should publish filesystem with mount option", func() {
		By("creating a PVC and Pod")
		_, err := kubectlWithInput(podWithMountOptionPVCYAML, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			var pod corev1.Pod
			err := getObjects(&pod, "pod", "pause-mount-option")
			if err != nil {
				return fmt.Errorf("failed to get pod. err: %w", err)
			}

			if pod.Status.Phase != corev1.PodRunning {
				return errors.New("Pod is not running")
			}

			return nil
		}).Should(Succeed())

		By("check mount option")
		var pvc corev1.PersistentVolumeClaim
		err = getObjects(&pvc, "pvc", "topo-pvc-mount-option")
		Expect(err).ShouldNot(HaveOccurred())

		stdout, err := execAtLocal("cat", nil, "/proc/mounts")
		Expect(err).ShouldNot(HaveOccurred())

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
		_, err = kubectlWithInput(podWithMountOptionPVCYAML, "delete", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("validate mount options", func() {
		mountTargetPath := "/mnt/csi-node-test"

		By("creating a logical volume resource")
		nodeName := "topolvm-e2e-worker"
		if isDaemonsetLvmdEnvSet() {
			nodeName = getDaemonsetLvmdNodeName()
		}

		name := "csi-node-test-fs"
		lvYaml := []byte(fmt.Sprintf(lvTemplateYAML, topolvm.GetPluginName(), name, name, nodeName))
		_, err := kubectlWithInput(lvYaml, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		var volumeID string
		Eventually(func() error {
			var lv topolvmv1.LogicalVolume
			err := getObjects(&lv, "logicalvolumes", name)
			if err != nil {
				return fmt.Errorf("failed to get logical volume. err: %w", err)
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
		_, err = kubectl("delete", "logicalvolumes", "csi-node-test-fs")
		Expect(err).ShouldNot(HaveOccurred())
	})
}
