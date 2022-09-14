package iolimit

import v1 "k8s.io/api/core/v1"

const (
	BlkIOThrottleReadBPS   = "blkio.throttle.read_bps_device"
	BlkIOThrottleReadIOPS  = "blkio.throttle.read_iops_device"
	BlkIOThrottleWriteBPS  = "blkio.throttle.write_bps_device"
	BlkIOThrottleWriteIOPS = "blkio.throttle.write_iops_device"
	Cgroupv2BlkIOThrottle  = "io.max"
)

// DeviceIOSet key is device number
type DeviceIOSet map[string]*IOLimit

type PodBlkIO struct {
	PodUid      string
	PodQos      v1.PodQOSClass
	DeviceIOSet DeviceIOSet
}

type IOLimit struct {
	Rbps  uint64
	Riops uint64
	Wbps  uint64
	Wiops uint64
}

func (bd1 *IOLimit) Equal(bd2 *IOLimit) bool {
	if bd1 == bd2 {
		return true
	}
	if bd1 == nil || bd2 == nil {
		return false
	}
	if bd1.Riops != bd2.Riops {
		return false
	}
	if bd1.Rbps != bd2.Rbps {
		return false
	}
	if bd1.Wiops != bd2.Wiops {
		return false
	}
	if bd1.Wbps != bd2.Wbps {
		return false
	}
	return true
}

func GetSupportedIOThrottles() []string {
	return []string{BlkIOThrottleReadBPS, BlkIOThrottleReadIOPS, BlkIOThrottleWriteBPS, BlkIOThrottleWriteIOPS}
}
