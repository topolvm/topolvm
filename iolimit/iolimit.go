package iolimit

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	libcontainercgroups "github.com/opencontainers/runc/libcontainer/cgroups"
	cgroupsystemd "github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	v1 "k8s.io/api/core/v1"
)

type CgroupDrivertype string

const (
	CgroupfsDriver CgroupDrivertype = "cgroupfs"
	SystemdDriver  CgroupDrivertype = "systemd"
	RootCgroup     string           = "/sys/fs/cgroup"
)

type CgroupName []string

var errTemplate = "the pod(uid %s)'s cgroup blkio path(%s) is not exist"

func exists(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, false
	}
	return info, true
}

// FileExists checks if a file exists and is not a directory
func fileExists(filepath string) bool {
	info, present := exists(filepath)
	return present && info.Mode().IsRegular()
}

// DirExists checks if a directory exists
func dirExists(path string) bool {
	info, present := exists(path)
	return present && info.IsDir()
}

func SetIOLimit(blkIO *PodBlkIO) error {
	blkPath := getPodBlkIOCgroupPath(blkIO)
	if !dirExists(blkPath) {
		return fmt.Errorf(errTemplate, blkIO.PodUid, blkPath)
	}
	if libcontainercgroups.IsCgroup2UnifiedMode() {
		ioMaxPath := path.Join(blkPath, Cgroupv2BlkIOThrottle)
		if !fileExists(ioMaxPath) {
			return fmt.Errorf(errTemplate, blkIO.PodUid, ioMaxPath)
		}
		for deviceNo, deviceIOLimit := range blkIO.DeviceIOSet {
			ioStr := getCG2IOLimitStr(deviceNo, deviceIOLimit)
			if err := os.WriteFile(ioMaxPath, []byte(ioStr), 0600); err != nil {
				return fmt.Errorf("failed to write ioStr(%s) to path(%s)", ioStr, ioMaxPath)
			}
		}
		return nil
	}
	ioLimitPath := getCG1IOLimitPaths(blkPath, blkIO.PodUid)
	for _, blkIOPath := range ioLimitPath {
		if !fileExists(blkIOPath) {
			return fmt.Errorf(errTemplate, blkIO.PodUid, blkIOPath)
		}
	}
	for deviceNo, iolt := range blkIO.DeviceIOSet {
		for _, throttle := range GetSupportedIOThrottles() {
			line := deviceNo
			switch throttle {
			case BlkIOThrottleReadBPS:
				if iolt.Rbps != 0 {
					line += " " + strconv.FormatUint(iolt.Rbps, 10)
				} else {
					line += " 0"
				}
			case BlkIOThrottleReadIOPS:
				if iolt.Riops != 0 {
					line += " " + strconv.FormatUint(iolt.Riops, 10)
				} else {
					line += " 0"
				}
			case BlkIOThrottleWriteBPS:
				if iolt.Wbps != 0 {
					line += " " + strconv.FormatUint(iolt.Wbps, 10)
				} else {
					line += " 0"
				}
			case BlkIOThrottleWriteIOPS:
				if iolt.Wiops != 0 {
					line += " " + strconv.FormatUint(iolt.Wiops, 10)
				} else {
					line += " 0"
				}
			default:
				return fmt.Errorf("unsupported throttle type %s", throttle)
			}
			if err := os.WriteFile(ioLimitPath[throttle], []byte(line), 0600); err != nil {
				return fmt.Errorf("failed to write ioStr(%s) to path(%s)", line, ioLimitPath[throttle])
			}
		}
	}
	return nil
}

func NewCgroupName(base CgroupName, components ...string) CgroupName {
	for _, component := range components {
		if strings.Contains(component, "/") || strings.Contains(component, "_") {
			panic(fmt.Errorf("invalid character in component [%q] of CgroupName", component))
		}
	}
	return append(append([]string{}, base...), components...)
}

func escapeSystemdCgroupName(part string) string {
	return strings.Replace(part, "-", "_", -1)
}

func (cgroupName CgroupName) toSystemd() string {
	if len(cgroupName) == 0 || (len(cgroupName) == 1 && cgroupName[0] == "") {
		return "/"
	}
	newparts := []string{}
	for _, part := range cgroupName {
		part = escapeSystemdCgroupName(part)
		newparts = append(newparts, part)
	}

	result, err := cgroupsystemd.ExpandSlice(strings.Join(newparts, "-") + ".slice")
	if err != nil {
		// Should never happen...
		panic(fmt.Errorf("error converting cgroup name [%v] to systemd format: %v", cgroupName, err))
	}
	return result
}

func (cgroupName CgroupName) toCgroupfs() string {
	return "/" + path.Join(cgroupName...)
}

func getPodBlkIOCgroupPath(blkIO *PodBlkIO) string {
	cgroupName := generatePodCgroupName(blkIO.PodQos, blkIO.PodUid)
	var cgroupPath string
	switch getCgroupDriverType() {
	case CgroupfsDriver:
		cgroupPath = cgroupName.toCgroupfs()
	case SystemdDriver:
		cgroupPath = cgroupName.toSystemd()
	}
	if libcontainercgroups.IsCgroup2UnifiedMode() {
		return path.Join(RootCgroup, cgroupPath)
	} else {
		return path.Join(RootCgroup, "blkio", cgroupPath)
	}
}

func generatePodCgroupName(qos v1.PodQOSClass, podUid string) CgroupName {
	cgroupName := CgroupName{"kubepods"}
	var qosClass string
	switch qos {
	case v1.PodQOSGuaranteed:
		return NewCgroupName(cgroupName, "pod"+string(podUid))
	case v1.PodQOSBurstable:
		qosClass = "burstable"
	case v1.PodQOSBestEffort:
		qosClass = "besteffort"
	}
	return NewCgroupName(cgroupName, qosClass, "pod"+string(podUid))
}

// check cgroup driver

func getCgroupDriverType() CgroupDrivertype {
	if dirExists(RootCgroup+"/kubepods.slice") || dirExists(RootCgroup+"/systemd/kubepods.slice") {
		return SystemdDriver
	}
	return CgroupfsDriver
}

func getCG1IOLimitPaths(blkPath, podUID string) map[string]string {
	ioPathMap := map[string]string{}
	ioPathMap[BlkIOThrottleReadBPS] = path.Join(blkPath, BlkIOThrottleReadBPS)
	ioPathMap[BlkIOThrottleReadIOPS] = path.Join(blkPath, BlkIOThrottleReadIOPS)
	ioPathMap[BlkIOThrottleWriteBPS] = path.Join(blkPath, BlkIOThrottleWriteBPS)
	ioPathMap[BlkIOThrottleWriteIOPS] = path.Join(blkPath, BlkIOThrottleWriteIOPS)
	return ioPathMap
}

func getCG2IOLimitStr(deviceNo string, iolt *IOLimit) string {
	line := deviceNo
	if iolt.Rbps != 0 {
		line += " rbps=" + strconv.FormatUint(iolt.Rbps, 10)
	} else {
		line += " rbps=max"
	}
	if iolt.Riops != 0 {
		line += " riops=" + strconv.FormatUint(iolt.Riops, 10)
	} else {
		line += " riops=max"
	}
	if iolt.Wbps != 0 {
		line += " wbps=" + strconv.FormatUint(iolt.Wbps, 10)
	} else {
		line += " wbps=max"
	}
	if iolt.Wiops != 0 {
		line += " wiops=" + strconv.FormatUint(iolt.Wiops, 10)
	} else {
		line += " wiops=max"
	}
	return line
}
