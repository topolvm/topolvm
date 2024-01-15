package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	nsenter = "/usr/bin/nsenter"
	lvm     = "/sbin/lvm"
)

var Containerized bool = false

// ErrNotFound is returned when a VG or LV is not found.
var ErrNotFound = errors.New("not found")

// wrapExecCommand calls cmd with args but wrapped to run
// on the host
func wrapExecCommand(cmd string, args ...string) *exec.Cmd {
	if Containerized {
		args = append([]string{"-m", "-u", "-i", "-n", "-p", "-t", "1", cmd}, args...)
		cmd = nsenter
	}
	c := exec.Command(cmd, args...)
	return c
}

// callLVM calls lvm sub-commands.
// cmd is a name of sub-command.
func callLVM(ctx context.Context, cmd string, args ...string) error {
	_, err := callLVMWithStdout(ctx, cmd, args...)
	return err
}

// callLVMWithStdout calls lvm sub-commands and returns stdout.
// cmd is a name of sub-command.
func callLVMWithStdout(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	args = append([]string{cmd}, args...)

	c := wrapExecCommand(lvm, args...)
	c.Env = os.Environ()
	c.Env = append(c.Env, "LC_ALL=C")
	c.Stdout = &stdout
	c.Stderr = &stderr

	log.FromContext(ctx).Info("invoking LVM command", "args", args)
	err := c.Run()

	log.FromContext(ctx).Info("result of LVM command", "stdout", stdout.Bytes(), "stderr", stderr.Bytes())

	return stdout.Bytes(), err
}

// LVInfo is a map of lv attributes to values.
type LVInfo map[string]string

// VolumeGroup represents a volume group of linux lvm.
type VolumeGroup struct {
	state vg
	lvs   []lv
}

func (g *VolumeGroup) Update(ctx context.Context) error {
	vgs, lvs, err := getLVMState(ctx)
	if err != nil {
		return err
	}
	for _, vg := range vgs {
		if vg.name == g.Name() {
			g.state = vg
			break
		}
	}

	g.lvs = filter_lv(g.Name(), lvs)
	return nil
}

// Name returns the volume group name.
func (g *VolumeGroup) Name() string {
	return g.state.name
}

// Size returns the capacity of the volume group in bytes.
func (g *VolumeGroup) Size() (uint64, error) {
	return g.state.size, nil
}

// Free returns the free space of the volume group in bytes.
func (g *VolumeGroup) Free() (uint64, error) {
	return g.state.free, nil
}

// CreateVolumeGroup calls "vgcreate" to create a volume group.
// name is for creating volume name. device is path to a PV.
func CreateVolumeGroup(ctx context.Context, name, device string) (*VolumeGroup, error) {
	err := callLVM(ctx, "vgcreate", "-ff", "-y", name, device)
	if err != nil {
		return nil, err
	}
	return FindVolumeGroup(ctx, name)
}

// FindVolumeGroup finds a named volume group.
// name is volume group name to look up.
func FindVolumeGroup(ctx context.Context, name string) (*VolumeGroup, error) {
	groups, err := ListVolumeGroups(ctx)
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

func SearchVolumeGroupList(vgs []*VolumeGroup, name string) (*VolumeGroup, error) {
	for _, vg := range vgs {
		if vg.state.name == name {
			return vg, nil
		}
	}
	return nil, ErrNotFound
}

func filter_lv(vg_name string, lvs []lv) []lv {
	var filtered []lv
	for _, l := range lvs {
		if l.vgName == vg_name {
			filtered = append(filtered, l)
		}
	}
	return filtered
}

// ListVolumeGroups lists all volume groups.
func ListVolumeGroups(ctx context.Context) ([]*VolumeGroup, error) {
	vgs, lvs, err := getLVMState(ctx)
	if err != nil {
		return nil, err
	}

	groups := []*VolumeGroup{}
	for _, vg := range vgs {
		groups = append(groups, &VolumeGroup{vg, filter_lv(vg.name, lvs)})
	}
	return groups, nil
}

// FindVolume finds a named logical volume in this volume group.
func (g *VolumeGroup) FindVolume(name string) (*LogicalVolume, error) {
	for _, volume := range g.ListVolumes() {
		if volume.Name() == name {
			return volume, nil
		}
	}
	return nil, ErrNotFound
}

// ListVolumes lists all logical volumes in this volume group.
func (g *VolumeGroup) ListVolumes() []*LogicalVolume {
	var ret []*LogicalVolume

	for i, lv := range g.lvs {
		if !lv.isThinPool() {
			size := lv.size

			var origin *string
			if len(lv.origin) > 0 {
				origin = &g.lvs[i].origin
			}

			var pool *string
			if len(lv.poolLV) > 0 {
				pool = &g.lvs[i].poolLV
			}

			if origin != nil && pool == nil {
				// this volume is a snapshot, but not a thin volume.
				size = lv.originSize
			}

			ret = append(ret, newLogicalVolume(
				lv.name,
				lv.path,
				g,
				size,
				origin,
				pool,
				uint32(lv.major),
				uint32(lv.minor),
				lv.tags,
			))
		}
	}
	return ret
}

// CreateVolume creates logical volume in this volume group.
// name is a name of creating volume. size is volume size in bytes. volTags is a
// list of tags to add to the volume.
// lvcreateOptions are additional arguments to pass to lvcreate.
func (g *VolumeGroup) CreateVolume(ctx context.Context, name string, size uint64, tags []string, stripe uint, stripeSize string,
	lvcreateOptions []string) (*LogicalVolume, error) {
	lvcreateArgs := []string{"-n", name, "-L", fmt.Sprintf("%vb", size), "-W", "y", "-y"}
	for _, tag := range tags {
		lvcreateArgs = append(lvcreateArgs, "--addtag")
		lvcreateArgs = append(lvcreateArgs, tag)
	}
	if stripe != 0 {
		lvcreateArgs = append(lvcreateArgs, "-i", fmt.Sprintf("%d", stripe))

		if stripeSize != "" {
			lvcreateArgs = append(lvcreateArgs, "-I", stripeSize)
		}
	}
	lvcreateArgs = append(lvcreateArgs, lvcreateOptions...)
	lvcreateArgs = append(lvcreateArgs, g.Name())

	if err := callLVM(ctx, "lvcreate", lvcreateArgs...); err != nil {
		return nil, err
	}
	if err := g.Update(ctx); err != nil {
		return nil, err
	}

	return g.FindVolume(name)
}

// FindPool finds a named thin pool in this volume group.
func (g *VolumeGroup) FindPool(name string) (*ThinPool, error) {
	for _, pool := range g.ListPools() {
		if pool.Name() == name {
			return pool, nil
		}
	}
	return nil, ErrNotFound
}

// ListPools lists all thin pool volumes in this volume group.
func (g *VolumeGroup) ListPools() []*ThinPool {
	ret := []*ThinPool{}
	for _, lv := range g.lvs {
		if lv.isThinPool() {
			ret = append(ret, newThinPool(lv.name, g, lv))
		}
	}
	return ret
}

// CreatePool creates a pool for thin-provisioning volumes.
func (g *VolumeGroup) CreatePool(ctx context.Context, name string, size uint64) (*ThinPool, error) {
	if err := callLVM(ctx, "lvcreate", "-T", fmt.Sprintf("%v/%v", g.Name(), name),
		"--size", fmt.Sprintf("%vb", size)); err != nil {
		return nil, err
	}
	if err := g.Update(ctx); err != nil {
		return nil, err
	}
	return g.FindPool(name)
}

// ThinPool represents a lvm thin pool.
type ThinPool struct {
	vg    *VolumeGroup
	state lv
}

// ThinPoolUsage holds current usage of lvm thin pool
type ThinPoolUsage struct {
	DataPercent     float64
	MetadataPercent float64
	VirtualBytes    uint64
	SizeBytes       uint64
}

func fullName(name string, vg *VolumeGroup) string {
	return fmt.Sprintf("%v/%v", vg.Name(), name)
}

func newThinPool(name string, vg *VolumeGroup, lvm_lv lv) *ThinPool {
	return &ThinPool{
		vg,
		lvm_lv,
	}
}

// Name returns thin pool name.
func (t *ThinPool) Name() string {
	return t.state.name
}

// FullName returns a VG prefixed name.
func (t *ThinPool) FullName() string {
	return t.state.fullName
}

// VG returns a volume group in which the thin pool is.
func (t *ThinPool) VG() *VolumeGroup {
	return t.vg
}

// Size returns a size of the thin pool.
func (t *ThinPool) Size() uint64 {
	return t.state.size
}

// Resize the thin pool capacity.
func (t *ThinPool) Resize(ctx context.Context, newSize uint64) error {
	if t.state.size == newSize {
		return nil
	}
	if err := callLVM(ctx, "lvresize", "-f", "-L", fmt.Sprintf("%vb", newSize), t.state.fullName); err != nil {
		return err
	}
	return t.vg.Update(ctx)
}

// ListVolumes lists all volumes in this thin pool.
func (t *ThinPool) ListVolumes() []*LogicalVolume {
	ret := []*LogicalVolume{}
	for _, volume := range t.vg.ListVolumes() {
		if volume.pool != nil && *volume.pool == t.state.name {
			ret = append(ret, volume)
		}
	}
	return ret
}

// FindVolume finds a named logical volume in this thin pool
func (t *ThinPool) FindVolume(name string) (*LogicalVolume, error) {
	for _, volume := range t.vg.ListVolumes() {
		if volume.name == name && volume.pool != nil && *volume.pool == t.state.name {
			return volume, nil
		}
	}
	return nil, ErrNotFound
}

// CreateVolume creates a thin volume from this pool.
func (t *ThinPool) CreateVolume(ctx context.Context, name string, size uint64, tags []string, stripe uint, stripeSize string, lvcreateOptions []string) (*LogicalVolume, error) {

	lvcreateArgs := []string{"-T", t.FullName(), "-n", name, "-V", fmt.Sprintf("%vb", size), "-W", "y", "-y"}
	for _, tag := range tags {
		lvcreateArgs = append(lvcreateArgs, "--addtag")
		lvcreateArgs = append(lvcreateArgs, tag)
	}
	if stripe != 0 {
		lvcreateArgs = append(lvcreateArgs, "-i", fmt.Sprintf("%d", stripe))

		if stripeSize != "" {
			lvcreateArgs = append(lvcreateArgs, "-I", stripeSize)
		}
	}
	lvcreateArgs = append(lvcreateArgs, lvcreateOptions...)

	if err := callLVM(ctx, "lvcreate", lvcreateArgs...); err != nil {
		return nil, err
	}
	if err := t.vg.Update(ctx); err != nil {
		return nil, err
	}
	return t.vg.FindVolume(name)
}

// Free on a thinpool returns used data, metadata percentages,
// sum of virtualsizes of all thinlvs and size of thinpool
func (t *ThinPool) Free() (*ThinPoolUsage, error) {
	tpu := &ThinPoolUsage{}
	tpu.DataPercent = t.state.dataPercent
	tpu.MetadataPercent = t.state.metaDataPercent
	tpu.SizeBytes = t.state.size

	for _, l := range t.vg.lvs {
		if l.poolLV == t.state.name {
			tpu.VirtualBytes += l.size
		}
	}
	return tpu, nil
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

// ThinSnapshot takes a thin snapshot of a volume.
// The volume must be thinly-provisioned.
// snapshots can be created unconditionally.
func (l *LogicalVolume) ThinSnapshot(ctx context.Context, name string, tags []string) (*LogicalVolume, error) {
	if !l.IsThin() {
		return nil, fmt.Errorf("cannot take snapshot of non-thin volume: %s", l.fullname)
	}

	lvcreateArgs := []string{"-s", "-k", "n", "-n", name, l.fullname}

	for _, tag := range tags {
		lvcreateArgs = append(lvcreateArgs, "--addtag")
		lvcreateArgs = append(lvcreateArgs, tag)
	}
	if err := callLVM(ctx, "lvcreate", lvcreateArgs...); err != nil {
		return nil, err
	}
	if err := l.vg.Update(ctx); err != nil {
		return nil, err
	}

	return l.vg.FindVolume(name)
}

// Activate activates the logical volume for desired access.
func (l *LogicalVolume) Activate(ctx context.Context, access string) error {
	var lvchangeArgs []string
	switch access {
	case "ro":
		lvchangeArgs = []string{"-p", "r", l.path}
	case "rw":
		lvchangeArgs = []string{"-k", "n", "-a", "y", l.path}
	default:
		return fmt.Errorf("unknown access: %s for LogicalVolume %s", access, l.fullname)
	}
	err := callLVM(ctx, "lvchange", lvchangeArgs...)

	return err
}

// Resize this volume.
// newSize is a new size of this volume in bytes.
func (l *LogicalVolume) Resize(ctx context.Context, newSize uint64) error {
	if l.size > newSize {
		return fmt.Errorf("volume cannot be shrunk")
	}
	if l.size == newSize {
		return nil
	}
	if err := callLVM(ctx, "lvresize", "-L", fmt.Sprintf("%vb", newSize), l.fullname); err != nil {
		return err
	}
	if err := l.vg.Update(ctx); err != nil {
		return err
	}

	// now we need to update the size of this volume
	vol, err := l.vg.FindVolume(l.name)
	if err != nil {
		return err
	}
	l.size = vol.size

	return nil
}

// Remove this volume.
func (l *LogicalVolume) Remove(ctx context.Context) error {
	if err := callLVM(ctx, "lvremove", "-f", l.path); err != nil {
		return err
	}
	return l.vg.Update(ctx)
}

// Rename this volume.
// This method also updates properties such as Name() or Path().
func (l *LogicalVolume) Rename(ctx context.Context, name string) error {
	if err := callLVM(ctx, "lvrename", l.vg.Name(), l.name, name); err != nil {
		return err
	}
	l.fullname = fullName(name, l.vg)
	l.name = name
	l.path = path.Join(path.Dir(l.path), l.name)
	return nil
}
