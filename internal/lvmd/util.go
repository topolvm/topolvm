package lvmd

import "math"

func calcThinPoolFreeBytes(overProvisionRatio float64, tpuSizeBytes, tpuVirtualBytes uint64) uint64 {
	virtualPoolSize := uint64(math.Floor(overProvisionRatio * float64(tpuSizeBytes)))
	if virtualPoolSize <= tpuVirtualBytes {
		return 0
	}
	return virtualPoolSize - tpuVirtualBytes
}
