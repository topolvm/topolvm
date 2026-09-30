package capacitytemplate

import (
	"context"
	"fmt"
	"math"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	gibibyte           int64 = 1 << 30
	lvmOverheadPerPV   int64 = 8 << 20
	logicalSectorBytes int64 = 512
	raidLevel0               = "raid0"
)

// RemoteLVMHandler calculates capacity for NodeGroup remoteVolumes that are
// prepared into the shared remote_data_vg TopoLVM volume group.
type RemoteLVMHandler struct{}

type parsedRemoteLVMVolume struct {
	name      string
	mode      string
	raidLevel string
	sizeGiB   int64
	diskCount int64
}

// CalculateCapacity calculates the conservative fresh-node remote LVM capacity
// represented by a NodeGroup. It mirrors k8s-nodegroups GetRemoteLVMCapacity:
// cloud disk sizes are rounded up to GiB, all member disks contribute capacity,
// and 8 MiB is deducted for each physical volume produced after RAID assembly.
func (RemoteLVMHandler) CalculateCapacity(
	_ context.Context,
	nodeGroup *unstructured.Unstructured,
	spareGB int64,
) (*resource.Quantity, error) {
	if nodeGroup == nil {
		return nil, fmt.Errorf("nodegroup must not be nil")
	}
	if spareGB < 0 || spareGB > math.MaxInt64/gibibyte {
		return nil, fmt.Errorf("spare-gb is outside the supported range")
	}

	rawVolumes, found, err := unstructured.NestedSlice(nodeGroup.Object, "spec", "storage", "remoteVolumes")
	if err != nil {
		return nil, fmt.Errorf("spec.storage.remoteVolumes must be an array: %w", err)
	}
	if !found || len(rawVolumes) == 0 {
		return nil, nil
	}

	seenNames := make(map[string]struct{}, len(rawVolumes))
	var capacityBytes int64
	var physicalVolumeCount int64
	for i, raw := range rawVolumes {
		volume, err := parseRemoteLVMVolume(raw, i)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seenNames[volume.name]; duplicate {
			return nil, fmt.Errorf("spec.storage.remoteVolumes contains duplicate name %q", volume.name)
		}
		seenNames[volume.name] = struct{}{}

		if volume.mode != "lvm" {
			continue
		}
		if volume.sizeGiB > math.MaxInt64/gibibyte/volume.diskCount {
			return nil, fmt.Errorf("spec.storage.remoteVolumes[%d] rounded capacity exceeds the supported range", i)
		}
		volumeBytes := volume.sizeGiB * gibibyte * volume.diskCount
		if capacityBytes > math.MaxInt64-volumeBytes {
			return nil, fmt.Errorf("spec.storage.remoteVolumes LVM capacity exceeds the supported range")
		}
		capacityBytes += volumeBytes

		volumePVCount := int64(1)
		if volume.raidLevel == "none" {
			volumePVCount = volume.diskCount
		}
		if physicalVolumeCount > math.MaxInt64-volumePVCount {
			return nil, fmt.Errorf("LVM physical volume count exceeds the supported range")
		}
		physicalVolumeCount += volumePVCount
	}

	return usableRemoteLVMCapacity(capacityBytes, physicalVolumeCount, spareGB)
}

func parseRemoteLVMVolume(raw interface{}, index int) (parsedRemoteLVMVolume, error) {
	entry, ok := raw.(map[string]interface{})
	if !ok {
		return parsedRemoteLVMVolume{}, fmt.Errorf("spec.storage.remoteVolumes[%d] must be an object", index)
	}

	name, ok := entry["name"].(string)
	if !ok || name == "" {
		return parsedRemoteLVMVolume{}, fmt.Errorf("spec.storage.remoteVolumes[%d].name must be a non-empty string", index)
	}

	mode, ok := entry["provisioningMode"].(string)
	if !ok || mode == "" {
		return parsedRemoteLVMVolume{}, fmt.Errorf("spec.storage.remoteVolumes[%d].provisioningMode must be a string", index)
	}
	switch mode {
	case "static", "zfs", "lvm":
	default:
		return parsedRemoteLVMVolume{}, fmt.Errorf(
			"spec.storage.remoteVolumes[%d].provisioningMode is invalid: %q", index, mode)
	}

	size, ok := entry["size"].(string)
	if !ok || size == "" {
		return parsedRemoteLVMVolume{}, fmt.Errorf("spec.storage.remoteVolumes[%d].size must be a quantity string", index)
	}
	quantity, err := resource.ParseQuantity(size)
	if err != nil {
		return parsedRemoteLVMVolume{}, fmt.Errorf("spec.storage.remoteVolumes[%d].size is invalid: %w", index, err)
	}
	if quantity.Sign() <= 0 {
		return parsedRemoteLVMVolume{}, fmt.Errorf("spec.storage.remoteVolumes[%d].size must be positive", index)
	}
	maxQuantity := resource.NewQuantity(math.MaxInt64, resource.DecimalSI)
	if quantity.Cmp(*maxQuantity) > 0 {
		return parsedRemoteLVMVolume{}, fmt.Errorf("spec.storage.remoteVolumes[%d].size exceeds the supported range", index)
	}

	count, err := remoteLVMDiskCount(entry, index)
	if err != nil {
		return parsedRemoteLVMVolume{}, err
	}
	level, err := remoteLVMRaidLevel(entry, index)
	if err != nil {
		return parsedRemoteLVMVolume{}, err
	}

	// Quantity.Value rounds a positive fractional byte up, matching NGC.
	sizeBytes := quantity.Value()
	return parsedRemoteLVMVolume{
		name:      name,
		mode:      mode,
		raidLevel: level,
		sizeGiB:   int64(1) + (sizeBytes-1)/gibibyte,
		diskCount: count,
	}, nil
}

func usableRemoteLVMCapacity(capacityBytes, physicalVolumeCount, spareGB int64) (*resource.Quantity, error) {
	if physicalVolumeCount == 0 {
		return nil, nil
	}
	if physicalVolumeCount > math.MaxInt64/lvmOverheadPerPV {
		return nil, fmt.Errorf("LVM metadata overhead exceeds the supported range")
	}
	overheadBytes := physicalVolumeCount * lvmOverheadPerPV
	reserveBytes := spareGB * gibibyte
	if overheadBytes > capacityBytes || reserveBytes > capacityBytes-overheadBytes {
		return nil, fmt.Errorf("calculated capacity is not positive after LVM overhead and spare-gb reserve")
	}
	capacityBytes -= overheadBytes + reserveBytes
	capacityBytes -= capacityBytes % logicalSectorBytes
	if capacityBytes <= 0 {
		return nil, fmt.Errorf("calculated capacity is not positive after rounding")
	}
	return resource.NewQuantity(capacityBytes, resource.BinarySI), nil
}

func remoteLVMDiskCount(entry map[string]interface{}, index int) (int64, error) {
	raw, found := entry["diskCount"]
	if !found {
		return 1, nil
	}

	var count int64
	switch value := raw.(type) {
	case int64:
		count = value
	case int32:
		count = int64(value)
	case int:
		count = int64(value)
	case float64:
		if value != math.Trunc(value) || value > math.MaxInt64 || value < math.MinInt64 {
			return 0, fmt.Errorf("spec.storage.remoteVolumes[%d].diskCount must be an integer", index)
		}
		count = int64(value)
	default:
		return 0, fmt.Errorf("spec.storage.remoteVolumes[%d].diskCount must be an integer", index)
	}
	if count == 0 {
		return 1, nil
	}
	if count < 0 {
		return 0, fmt.Errorf("spec.storage.remoteVolumes[%d].diskCount must not be negative", index)
	}
	return count, nil
}

func remoteLVMRaidLevel(entry map[string]interface{}, index int) (string, error) {
	raw, found := entry["raidLevel"]
	if !found {
		return raidLevel0, nil
	}
	level, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("spec.storage.remoteVolumes[%d].raidLevel must be a string", index)
	}
	if level == "" {
		return raidLevel0, nil
	}
	if level != raidLevel0 && level != "none" {
		return "", fmt.Errorf("spec.storage.remoteVolumes[%d].raidLevel is invalid: %q", index, level)
	}
	return level, nil
}
