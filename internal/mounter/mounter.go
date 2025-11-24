package mounter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/internal/filesystem"
	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	mountutil "k8s.io/mount-utils"
	utilexec "k8s.io/utils/exec"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var mountLogger = ctrl.Log.WithName("driver").WithName("mounter")

type LVMount struct {
	nodeName  string
	client    client.Client
	vgClient  proto.VGServiceClient
	lvService proto.LVServiceClient
	mounter   mountutil.SafeFormatAndMount
}

type MountResponse struct {
	DevicePath     string
	MountPath      string
	FSType         string
	MountOptions   []string
	AlreadyMounted bool
	Success        bool
	Message        string
}

func NewLVMount(client client.Client, vgService proto.VGServiceClient, lvService proto.LVServiceClient) *LVMount {
	return &LVMount{
		client:    client,
		vgClient:  vgService,
		lvService: lvService,
		mounter: mountutil.SafeFormatAndMount{
			Interface: mountutil.New(""),
			Exec:      utilexec.New(),
		},
	}
}

// Mount performs an idempotent mount of the LV snapshot.
func (m *LVMount) Mount(ctx context.Context, k8sLV *v1.LogicalVolume, mountOpts []string) (*MountResponse, error) {
	resp := &MountResponse{}
	lv, err := m.fetchLogicalVolume(ctx, k8sLV)
	if err != nil {
		return resp, err
	}
	devicePath := lv.GetPath()

	mountPath := getMountPathFromLV(lv.GetName())

	fsType, err := filesystem.DetectFilesystem(devicePath)
	if err != nil {
		return resp, fmt.Errorf("failed to detect filesystem for LV %s: %v", k8sLV.Name, err)
	}

	// Default to ext4 if no filesystem is detected
	if fsType == "" {
		fsType, err = m.getFSTypeFromPV(k8sLV)
		if err != nil {
			return resp, fmt.Errorf("failed to get FSType from StorageClass: %v", err)
		}
	}
	if fsType == "" {
		return m.fail(resp, "failed to detect filesystem for LV snapshot")
	}

	resp.DevicePath = devicePath
	resp.MountPath = mountPath
	resp.FSType = fsType
	resp.MountOptions = mountOpts

	mountLogger.Info("Mounting LV snapshot",
		"volume", k8sLV.Name,
		"device", devicePath,
		"mountPath", mountPath,
		"fsType", fsType,
		"options", mountOpts,
	)

	if err := m.prepareMountDir(mountPath); err != nil && !os.IsExist(err) {
		return m.fail(resp, fmt.Sprintf("failed to create mount directory: %v", err))
	}

	alreadyMounted, err := m.checkAlreadyMounted(mountPath)
	if err != nil {
		return m.fail(resp, fmt.Sprintf("failed to check mount point: %v", err))
	}
	if alreadyMounted {
		resp.AlreadyMounted = true
		resp.Success = true
		resp.Message = "LV snapshot already mounted"
		mountLogger.Info("Already mounted", "target", mountPath)
		return resp, nil
	}
	if err := m.doMount(devicePath, mountPath, fsType, mountOpts); err != nil {
		return m.fail(resp, fmt.Sprintf("failed to mount %s: %v", devicePath, err))
	}

	resp.Success = true
	resp.Message = "Snapshot mounted successfully"
	mountLogger.Info("Mounted successfully", "device", devicePath, "target", mountPath)
	return resp, nil
}

func (m *LVMount) getFSTypeFromPV(k8sLv *v1.LogicalVolume) (string, error) {
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: k8sLv.Spec.Name,
		},
	}
	if err := m.client.Get(context.TODO(), client.ObjectKeyFromObject(pv), pv); err != nil {
		return "", err
	}
	if pv.Spec.CSI == nil {
		return "", fmt.Errorf("PV %s has no CSI spec", pv.Name)
	}
	return pv.Spec.CSI.FSType, nil
}

// BindMount performs an idempotent bind mount of the LV.
// This is useful for restore operations where the LV is already mounted by the pod.
// It automatically finds the current mount point of the device and creates a bind mount.
func (m *LVMount) BindMount(ctx context.Context, k8sLV *v1.LogicalVolume) (*MountResponse, error) {
	resp := &MountResponse{}

	lv, err := m.fetchLogicalVolume(ctx, k8sLV)
	if err != nil {
		return resp, err
	}

	devicePath := lv.GetPath()

	// Find where the device is currently mounted
	sourceMountPath, err := m.findMountPoint(devicePath)
	if err != nil {
		return m.fail(resp, fmt.Sprintf("failed to find mount point for device %s: %v", devicePath, err))
	}

	if sourceMountPath == "" {
		return m.fail(resp, fmt.Sprintf("device %s is not currently mounted", devicePath))
	}

	mountPath := getMountPathFromLV(lv.GetName())

	// For bind mount, the source is the existing mount point
	// Use read-write mount for restore operations (no "ro" flag)
	mountOptions := []string{"bind"}

	resp.DevicePath = devicePath
	resp.MountPath = mountPath
	resp.FSType = "" // FSType is not needed for bind mounts
	resp.MountOptions = mountOptions

	mountLogger.Info("Bind mounting LV for restore",
		"volume", k8sLV.Name,
		"device", devicePath,
		"source", sourceMountPath,
		"mountPath", mountPath,
		"options", mountOptions,
	)

	if err := m.prepareMountDir(mountPath); err != nil && !os.IsExist(err) {
		return m.fail(resp, fmt.Sprintf("failed to create mount directory: %v", err))
	}

	alreadyMounted, err := m.checkAlreadyMounted(mountPath)
	if err != nil {
		return m.fail(resp, fmt.Sprintf("failed to check mount point: %v", err))
	}
	if alreadyMounted {
		resp.AlreadyMounted = true
		resp.Success = true
		resp.Message = "LV already bind mounted"
		mountLogger.Info("Already bind mounted", "target", mountPath)
		return resp, nil
	}

	// Perform bind mount
	if err := m.doMount(sourceMountPath, mountPath, "", mountOptions); err != nil {
		return m.fail(resp, fmt.Sprintf("failed to bind mount %s to %s: %v", sourceMountPath, mountPath, err))
	}

	resp.Success = true
	resp.Message = "Bind mount successful"
	mountLogger.Info("Bind mounted successfully", "source", sourceMountPath, "target", mountPath)
	return resp, nil
}

func (m *LVMount) fetchLogicalVolume(ctx context.Context, k8sLV *v1.LogicalVolume) (*proto.LogicalVolume, error) {
	lv, err := m.getLogicalVolume(ctx, k8sLV.Spec.DeviceClass, k8sLV.Status.VolumeID)
	if err != nil {
		return nil, err
	}
	if lv == nil {
		return nil, fmt.Errorf("failed to find the logical volume for k8s LV %s", k8sLV.Name)
	}
	return lv, nil
}

func getMountPathFromLV(lvName string) string {
	baseDir := os.Getenv("ONLINE_SNAPSHOT_DIR")
	return filepath.Join(baseDir, lvName)
}

func (m *LVMount) prepareMountDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func (m *LVMount) checkAlreadyMounted(target string) (bool, error) {
	isMnt, err := m.mounter.IsMountPoint(target)
	if err != nil {
		return true, err
	}
	return isMnt, nil
}

func (m *LVMount) doMount(device, target, fsType string, options []string) error {
	return m.mounter.FormatAndMount(device, target, fsType, options)
}

func (m *LVMount) doFormatAndMount(device, target, fsType string, options []string) error {
	// Check if device is already formatted
	existingFormat, err := filesystem.DetectFilesystem(device)
	if err != nil {
		return fmt.Errorf("failed to detect filesystem: %v", err)
	}

	// If device is not formatted, format it first
	if existingFormat == "" {
		mountLogger.Info("Device is unformatted, formatting now",
			"device", device,
			"fsType", fsType,
		)

		// Format the device using mkfs
		// We need to format without the read-only option
		var args []string
		if fsType == "ext4" || fsType == "ext3" {
			args = []string{
				"-F",  // Force flag
				"-m0", // Zero blocks reserved for super-user
				device,
			}
		} else if fsType == "xfs" {
			args = []string{
				"-f", // force flag
				device,
			}
		} else {
			args = []string{device}
		}

		output, err := m.mounter.Exec.Command("mkfs."+fsType, args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to format device %s as %s: %v, output: %s", device, fsType, err, string(output))
		}

		mountLogger.Info("Device formatted successfully",
			"device", device,
			"fsType", fsType,
		)
	}

	// Now mount with the requested options (including ro if specified)
	return m.mounter.Mount(device, target, fsType, options)
}

func (m *LVMount) fail(resp *MountResponse, msg string) (*MountResponse, error) {
	resp.Success = false
	resp.Message = msg
	return resp, fmt.Errorf(msg)
}

// findMountPoint finds the mount point for a given device by parsing /proc/mounts
func (m *LVMount) findMountPoint(devicePath string) (string, error) {
	// Read /proc/mounts to find where the device is mounted
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return "", fmt.Errorf("failed to read /proc/mounts: %v", err)
	}

	// Parse the mount information
	// Format: <device> <mount point> <fs type> <options> <dump> <pass>
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// Check if this is our device
		if fields[0] == devicePath {
			return fields[1], nil
		}
	}

	return "", nil
}

func (m *LVMount) getLogicalVolume(ctx context.Context, deviceClass, volumeID string) (*proto.LogicalVolume, error) {
	listResp, err := m.vgClient.GetLVList(ctx, &proto.GetLVListRequest{DeviceClass: deviceClass})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list LV: %v", err)
	}
	return m.findVolumeByID(listResp, volumeID), nil
}

func (m *LVMount) findVolumeByID(listResp *proto.GetLVListResponse, name string) *proto.LogicalVolume {
	for _, v := range listResp.Volumes {
		if v.Name == name {
			return v
		}
	}
	return nil
}

// Unmount performs an idempotent unmount of the LV snapshot.
func (m *LVMount) Unmount(ctx context.Context, k8sLV *v1.LogicalVolume) error {
	lv, err := m.fetchLogicalVolume(ctx, k8sLV)
	if err != nil {
		// If the LV doesn't exist, consider it already unmounted
		if status.Code(err) == codes.NotFound {
			mountLogger.Info("LV not found, skipping unmount", "volume", k8sLV.Name)
			return nil
		}
		return fmt.Errorf("failed to fetch logical volume for unmount: %v", err)
	}

	mountPath := getMountPathFromLV(lv.GetName())

	mountLogger.Info("Unmounting LV snapshot",
		"volume", k8sLV.Name,
		"mountPath", mountPath,
	)

	// Check if it's mounted
	isMounted, err := m.mounter.IsMountPoint(mountPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Mount point doesn't exist, consider it already unmounted
			mountLogger.Info("Mount path doesn't exist, skipping unmount", "mountPath", mountPath)
			return nil
		}
		return fmt.Errorf("failed to check mount point: %v", err)
	}

	if !isMounted {
		mountLogger.Info("LV not mounted, skipping unmount", "mountPath", mountPath)
		return nil
	}

	// Perform the unmount
	if err := m.mounter.Unmount(mountPath); err != nil {
		return fmt.Errorf("failed to unmount %s: %v", mountPath, err)
	}

	mountLogger.Info("Unmounted successfully", "mountPath", mountPath)

	// Optionally remove the mount directory
	if err := os.Remove(mountPath); err != nil && !os.IsNotExist(err) {
		mountLogger.Info("Warning: failed to remove mount directory", "mountPath", mountPath, "error", err)
		// Don't return error here as unmount was successful
	}

	return nil
}
