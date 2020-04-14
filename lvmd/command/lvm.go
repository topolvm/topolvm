package command

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/cybozu-go/log"
)

const (
	lvm      = "/sbin/lvm"
	blockdev = "/sbin/blockdev"
	cowMin   = 50
	cowMax   = 300
)

// ErrNotFound is returned when a VG or LV is not found.
var ErrNotFound = errors.New("not found")

// CallLVM calls lvm sub-commands.
// cmd is a name of sub-command.
func CallLVM(cmd string, args ...string) error {
	args = append([]string{cmd}, args...)
	c := exec.Command(lvm, args...)
	log.Info("invoking LVM command", map[string]interface{}{
		"args": args,
	})
	c.Stderr = os.Stderr
	return c.Run()
}

// LVInfo is a map of lv attributes to values.
type LVInfo map[string]string

func parseOneLine(line string) LVInfo {
	ret := LVInfo{}
	line = strings.TrimSpace(line)
	for _, token := range strings.Split(line, " ") {
		if len(token) == 0 {
			continue
		}
		// assume token is "k=v"
		kv := strings.Split(token, "=")
		k, v := kv[0], kv[1]
		// k[5:] removes "LVM2_" prefix.
		k = strings.ToLower(k[5:])
		// assume v is 'some-value'
		v = strings.Trim(v, "'")
		ret[k] = v
	}
	return ret
}

// parseLines parses output from lvm.
func parseLines(output string) []LVInfo {
	ret := []LVInfo{}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		info := parseOneLine(line)
		ret = append(ret, info)
	}
	return ret
}

// parseOutput calls lvm family and parses output from it.
//
// cmd is a command name of lvm family.
// fields are comma separated field names.
// args is optional arguments for lvm command.
func parseOutput(cmd, fields string, args ...string) ([]LVInfo, error) {
	arg := []string{
		cmd, "-o", fields,
		"--noheadings", "--separator= ",
		"--units=b", "--nosuffix",
		"--unbuffered", "--nameprefixes",
	}
	arg = append(arg, args...)
	c := exec.Command(lvm, arg...)
	c.Stderr = os.Stderr
	stdout, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := c.Start(); err != nil {
		return nil, err
	}
	out, err := ioutil.ReadAll(stdout)
	if err != nil {
		return nil, err
	}
	if err := c.Wait(); err != nil {
		return nil, err
	}
	return parseLines(string(out)), nil
}

// VolumeGroup represents a volume group of linux lvm.
type VolumeGroup struct {
	name string
}

// Name returns the volume group name.
func (g *VolumeGroup) Name() string {
	return g.name
}

// Size returns the capacity of the volume group in bytes.
func (g *VolumeGroup) Size() (uint64, error) {
	infoList, err := parseOutput("vgs", "vg_size", g.name)
	if err != nil {
		return 0, err
	}

	if len(infoList) != 1 {
		return 0, errors.New("volume group not found: " + g.name)
	}

	info := infoList[0]
	vgSize, err := strconv.ParseUint(info["vg_size"], 10, 64)
	if err != nil {
		return 0, err
	}
	return vgSize, nil
}

// Free returns the free space of the volume group in bytes.
func (g *VolumeGroup) Free() (uint64, error) {
	infoList, err := parseOutput("vgs", "vg_free", g.name)
	if err != nil {
		return 0, err
	}

	if len(infoList) != 1 {
		return 0, errors.New("volume group not found: " + g.name)
	}

	info := infoList[0]
	vgFree, err := strconv.ParseUint(info["vg_free"], 10, 64)
	if err != nil {
		return 0, err
	}
	return vgFree, nil
}

// CreateVolumeGroup calls "vgcreate" to create a volume group.
// name is for creating volume name. device is path to a PV.
func CreateVolumeGroup(name, device string) (*VolumeGroup, error) {
	err := CallLVM("vgcreate", "-ff", "-y", name, device)
	if err != nil {
		return nil, err
	}
	return FindVolumeGroup(name)
}

// FindVolumeGroup finds a named volume group.
// name is volume group name to look up.
func FindVolumeGroup(name string) (*VolumeGroup, error) {
	groups, err := ListVolumeGroups()
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		if group.Name() == name {
			return group, nil
		}
	}
	return nil, ErrNotFound
}

// ListVolumeGroups lists all volume groups.
func ListVolumeGroups() ([]*VolumeGroup, error) {
	infoList, err := parseOutput("vgs", "vg_name")
	if err != nil {
		return nil, err
	}
	groups := []*VolumeGroup{}
	for _, info := range infoList {
		groups = append(groups, &VolumeGroup{info["vg_name"]})
	}
	return groups, nil
}

// FindVolume finds a named logical volume in this volume group.
func (g *VolumeGroup) FindVolume(name string) (*LogicalVolume, error) {
	volumes, err := g.ListVolumes()
	if err != nil {
		return nil, err
	}
	for _, volume := range volumes {
		if volume.Name() == name {
			return volume, nil
		}
	}
	return nil, ErrNotFound
}

// ListVolumes lists all logical volumes in this volume group.
func (g *VolumeGroup) ListVolumes() ([]*LogicalVolume, error) {
	infoList, err := parseOutput(
		"lvs",
		"lv_name,lv_path,lv_size,lv_kernel_major,lv_kernel_minor,origin,origin_size,pool_lv,thin_count,lv_tags",
		g.Name())
	if err != nil {
		return nil, err
	}
	var ret []*LogicalVolume
	lvNameSet := make(map[string]struct{})
	for _, info := range infoList {
		if len(info["thin_count"]) > 0 {
			continue
		}
		// Avoid listing duplicate LVs divided with segments
		if _, ok := lvNameSet[info["lv_name"]]; ok {
			continue
		}
		lvNameSet[info["lv_name"]] = struct{}{}
		size, err := strconv.ParseUint(info["lv_size"], 10, 64)
		if err != nil {
			return nil, err
		}
		var origin *string
		if len(info["origin"]) > 0 {
			originName := info["origin"]
			origin = &originName
		}
		var pool *string
		if len(info["pool_lv"]) > 0 {
			poolLv := info["pool_lv"]
			pool = &poolLv
		}
		if origin != nil && pool == nil {
			// this volume is a snapshot, but not a thin volume.
			size, err = strconv.ParseUint(info["origin_size"], 10, 64)
			if err != nil {
				return nil, err
			}
		}
		major, _ := strconv.ParseUint(info["lv_kernel_major"], 10, 32)
		minor, _ := strconv.ParseUint(info["lv_kernel_minor"], 10, 32)
		ret = append(ret, newLogicalVolume(
			info["lv_name"],
			info["lv_path"],
			g,
			size,
			origin,
			pool,
			uint32(major),
			uint32(minor),
			strings.Split(info["lv_tags"], ","),
		))
	}
	return ret, nil
}

// CreateVolume creates logical volume in this volume group.
// name is a name of creating volume. size is volume size in bytes. volTags is a
// list of tags to add to the volume.
func (g *VolumeGroup) CreateVolume(name string, size uint64, tags []string) (*LogicalVolume, error) {
	lvcreateArgs := []string{"-n", name, "-L", fmt.Sprintf("%vg", size>>30), "-W", "y", "-y"}
	for _, tag := range tags {
		lvcreateArgs = append(lvcreateArgs, "--addtag")
		lvcreateArgs = append(lvcreateArgs, tag)
	}
	lvcreateArgs = append(lvcreateArgs, g.Name())
	if err := CallLVM("lvcreate", lvcreateArgs...); err != nil {
		return nil, err
	}
	return g.FindVolume(name)
}

// FindPool finds a named thin pool in this volume group.
func (g *VolumeGroup) FindPool(name string) (*ThinPool, error) {
	pools, err := g.ListPools()
	if err != nil {
		return nil, err
	}
	for _, pool := range pools {
		if pool.Name() == name {
			return pool, nil
		}
	}
	return nil, fmt.Errorf("not found thin pool: %v", name)
}

// ListPools lists all thin pool volumes in this volume group.
func (g *VolumeGroup) ListPools() ([]*ThinPool, error) {
	infoList, err := parseOutput("lvs", "lv_name,lv_size,thin_count", g.Name())
	if err != nil {
		return nil, err
	}
	ret := []*ThinPool{}
	for _, info := range infoList {
		if len(info["thin_count"]) == 0 {
			continue
		}
		lvSize, err := strconv.ParseUint(info["lv_size"], 10, 64)
		if err != nil {
			return nil, err
		}
		ret = append(ret, newThinPool(info["lv_name"], g, lvSize))
	}
	return ret, nil
}

// CreatePool creates a pool for thin-provisioning volumes.
func (g *VolumeGroup) CreatePool(name string, size uint64) (*ThinPool, error) {
	if err := CallLVM("lvcreate", "-T", fmt.Sprintf("%v/%v", g.Name(), name),
		"--size", fmt.Sprintf("%vg", size>>30)); err != nil {
		return nil, err
	}
	return g.FindPool(name)
}

// ThinPool represents a lvm thin pool.
type ThinPool struct {
	fullname string
	name     string
	vg       *VolumeGroup
	size     uint64
}

func fullName(name string, vg *VolumeGroup) string {
	return fmt.Sprintf("%v/%v", vg.Name(), name)
}

func newThinPool(name string, vg *VolumeGroup, size uint64) *ThinPool {
	fullname := fullName(name, vg)
	return &ThinPool{
		fullname,
		name,
		vg,
		size,
	}
}

// Name returns thin pool name.
func (t *ThinPool) Name() string {
	return t.name
}

// FullName returns a VG prefixed name.
func (t *ThinPool) FullName() string {
	return t.fullname
}

// VG returns a volume group in which the thin pool is.
func (t *ThinPool) VG() *VolumeGroup {
	return t.vg
}

// Size returns a size of the thin pool.
func (t *ThinPool) Size() uint64 {
	return t.size
}

// Resize the thin pool capacity.
func (t *ThinPool) Resize(newSize uint64) error {
	if t.size == newSize {
		return nil
	}
	if err := CallLVM("lvresize", "-f", "-L", fmt.Sprintf("%vb", newSize), t.fullname); err != nil {
		return err
	}
	t.size = newSize
	return nil
}

// ListVolumes lists all volumes in this thin pool.
func (t *ThinPool) ListVolumes() ([]*LogicalVolume, error) {
	volumes, err := t.vg.ListVolumes()
	if err != nil {
		return nil, err
	}
	ret := []*LogicalVolume{}
	for _, volume := range volumes {
		if volume.pool != nil && *volume.pool == t.name {
			ret = append(ret, volume)
		}
	}
	return ret, nil
}

// CreateVolume creates a thin volume from this pool.
func (t *ThinPool) CreateVolume(name string, size uint64) (*LogicalVolume, error) {
	if err := CallLVM("lvcreate", "-T", t.fullname, "-n", name, "-V", fmt.Sprintf("%vg", size>>30)); err != nil {
		return nil, err
	}
	return t.vg.FindVolume(name)
}

// LogicalVolume represents a logical volume.
type LogicalVolume struct {
	fullname string
	// name is equivalent for LogicalVolume CRD UID
	name     string
	path     string
	vg       *VolumeGroup
	size     uint64
	origin   *string
	pool     *string
	devMajor uint32
	devMinor uint32
	tags     []string
}

func newLogicalVolume(name, path string, vg *VolumeGroup, size uint64, origin, pool *string, major, minor uint32, tags []string) *LogicalVolume {
	fullname := fullName(name, vg)
	return &LogicalVolume{
		fullname,
		name,
		path,
		vg,
		size,
		origin,
		pool,
		major,
		minor,
		tags,
	}
}

// Name returns a volume name.
func (l *LogicalVolume) Name() string {
	return l.name
}

// FullName returns a vg prefixed volume name.
func (l *LogicalVolume) FullName() string {
	return l.fullname
}

// Path returns a path to the logical volume.
func (l *LogicalVolume) Path() string {
	return l.path
}

// VG returns a volume group in which the volume is.
func (l *LogicalVolume) VG() *VolumeGroup {
	return l.vg
}

// Size returns a size of the volume.
func (l *LogicalVolume) Size() uint64 {
	return l.size
}

// IsSnapshot checks if the volume is snapshot or not.
func (l *LogicalVolume) IsSnapshot() bool {
	return l.origin != nil
}

// Origin returns logical volume instance if this is a snapshot, or nil if not.
func (l *LogicalVolume) Origin() (*LogicalVolume, error) {
	if l.origin == nil {
		return nil, nil
	}
	return l.vg.FindVolume(*l.origin)
}

// IsThin checks if the volume is thin volume or not.
func (l *LogicalVolume) IsThin() bool {
	return l.pool != nil
}

// Pool returns thin pool if this is a thin pool, or nil if not.
func (l *LogicalVolume) Pool() (*ThinPool, error) {
	if l.pool == nil {
		return nil, nil
	}
	return l.vg.FindPool(*l.pool)
}

// MajorNumber returns the device major number.
func (l *LogicalVolume) MajorNumber() uint32 {
	return l.devMajor
}

// MinorNumber returns the device minor number.
func (l *LogicalVolume) MinorNumber() uint32 {
	return l.devMinor
}

// Tags returns the tags member.
func (l *LogicalVolume) Tags() []string {
	return l.tags
}

// Snapshot takes a snapshot of this volume.
//
// If this is a thin-provisioning volume, snapshots can be
// created unconditionally.  Else, snapshots can be created
// only for non-snapshot volumes.
func (l *LogicalVolume) Snapshot(name string, cowSize uint64) (*LogicalVolume, error) {
	if l.pool == nil {
		if l.IsSnapshot() {
			return nil, fmt.Errorf("snapshot of snapshot")
		}
		var gbSize uint64
		if cowSize > 0 {
			gbSize = cowSize >> 30
		} else {
			gbSize = (l.size * 2 / 10) >> 30
			if gbSize > cowMax {
				gbSize = cowMax
			}
		}
		if gbSize < cowMin {
			gbSize = cowMin
		}
		if l.size < (gbSize << 30) {
			gbSize = (l.size >> 30) << 30
		}
		if err := CallLVM("lvcreate", "-s", "-n", name, "-L", fmt.Sprintf("%vg", gbSize), l.path); err != nil {
			return nil, err
		}

		time.Sleep(2 * time.Second)
		snapLV, err := l.vg.FindVolume(name)
		if err != nil {
			return nil, err
		}
		// without this, wrong data may read from the snapshot.
		if err := exec.Command(blockdev, "--flushbufs", snapLV.path).Run(); err != nil {
			return nil, err
		}
		return snapLV, nil
	}

	var lvcreateArgs []string
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		lvcreateArgs = []string{"-s", "-n", name, l.fullname}
	} else {
		lvcreateArgs = []string{"-s", "-k", "n", "-n", name, l.fullname}
	}
	if err := CallLVM("lvcreate", lvcreateArgs...); err != nil {
		return nil, err
	}
	return l.vg.FindVolume(name)
}

// Resize this volume.
// newSize is a new size of this volume in bytes.
func (l *LogicalVolume) Resize(newSize uint64) error {
	if l.size > newSize {
		return fmt.Errorf("volume cannot be shrunk")
	}
	if l.size == newSize {
		return nil
	}
	if err := CallLVM("lvresize", "-L", fmt.Sprintf("%vb", newSize), l.fullname); err != nil {
		return err
	}
	l.size = newSize
	return nil
}

// Remove this volume.
func (l *LogicalVolume) Remove() error {
	return CallLVM("lvremove", "-f", l.path)
}

// Rename this volume.
// This method also updates properties such as Name() or Path().
func (l *LogicalVolume) Rename(name string) error {
	if err := CallLVM("lvrename", l.vg.Name(), l.name, name); err != nil {
		return err
	}
	l.fullname = fullName(name, l.vg)
	l.name = name
	l.path = path.Join(path.Dir(l.path), l.name)
	return nil
}
