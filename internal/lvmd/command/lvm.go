package command

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path"

	"github.com/topolvm/topolvm"
)

// ErrNotFound is returned when a VG or LV is not found.
var ErrNotFound = errors.New("not found")

// ErrNoMultipleOfSectorSize is returned when a volume is requested that is smaller than the minimum sector size.
var ErrNoMultipleOfSectorSize = fmt.Errorf("cannot create volume as given size "+
	"is not a multiple of %d and could get rejected", topolvm.MinimumSectorSize)

// VolumeGroup represents a volume group of linux lvm.
// The state should be considered immutable and will not automatically update.
// The Update method should be called to refresh the state in case it is known that the state may have changed.
type VolumeGroup struct {
	state vg
	// reportLvs is used with getLVMState, which populates vg and lv at the same time.
	// should not be used otherwise as fields are fetched dynamically.
	reportLvs map[string]lv
}

// getLVs returns the current state of lvm lvs for the given volume group.
// If lvname is empty, all lvs are returned. Otherwise, only the lv with the given name is returned or an error if not found.
func getLVs(ctx context.Context, vg *VolumeGroup, lvname string) (map[string]lv, error) {
	// use fast path if we have the lvs already through the report
	if len(vg.reportLvs) > 0 {
		if lvname != "" {
			if lvFromMap, ok := vg.reportLvs[lvname]; ok {
				return map[string]lv{lvname: lvFromMap}, nil
			}
			return nil, ErrNotFound
		}
		return vg.reportLvs, nil
	}

	// by default, fetch all lvs for the vg
	name := vg.state.name
	// if lvname is set, only fetch that lv in the vg
	if lvname != "" {
		name += "/" + lvname
	}

	return getLVReport(ctx, name)
}

func (vg *VolumeGroup) Update(ctx context.Context) error {
	newVG, err := FindVolumeGroup(ctx, vg.Name())
	if err != nil {
		return err
	}
	vg.reportLvs = nil
	vg.state = newVG.state
	return nil
}

// Name returns the volume group name.
func (vg *VolumeGroup) Name() string {
	return vg.state.name
}

// Size returns the capacity of the volume group in bytes.
func (vg *VolumeGroup) Size() (uint64, error) {
	return vg.state.size, nil
}

// Free returns the free space of the volume group in bytes.
func (vg *VolumeGroup) Free() (uint64, error) {
	return vg.state.free, nil
}

// FindVolumeGroup finds a named volume group.
// name is volume group name to look up.
func FindVolumeGroup(ctx context.Context, name string) (*VolumeGroup, error) {
	vg, err := getVGReport(ctx, name)
	if err != nil {
		return nil, err
	}
	return &VolumeGroup{state: vg}, nil
}

func SearchVolumeGroupList(vgs []*VolumeGroup, name string) (*VolumeGroup, error) {
	for _, vg := range vgs {
		if vg.state.name == name {
			return vg, nil
		}
	}
	return nil, ErrNotFound
}

func filterLV(vgName string, lvs []lv) map[string]lv {
	filtered := map[string]lv{}
	for _, l := range lvs {
		if l.vgName == vgName {
			filtered[l.name] = l
		}
	}
	return filtered
}

// ListVolumeGroups lists all volume groups and logical volumes through the lvm state, which
// is more efficient than calling vgs / lvs for every command.
// Any VolumeGroup returned will already have the reportLvs populated.
func ListVolumeGroups(ctx context.Context) ([]*VolumeGroup, error) {
	vgs, lvs, err := getLVMState(ctx)
	if err != nil {
		return nil, err
	}

	groups := make([]*VolumeGroup, 0, len(vgs))
	for _, vg := range vgs {
		groups = append(groups, &VolumeGroup{state: vg, reportLvs: filterLV(vg.name, lvs)})
	}
	return groups, nil
}

// FindVolume finds a named logical volume in this volume group.
func (vg *VolumeGroup) FindVolume(ctx context.Context, name string) (*LogicalVolume, error) {
	volumes, err := vg.listVolumes(ctx, name)
	if err != nil {
		return nil, err
	}
	vol, ok := volumes[name]
	if !ok {
		return nil, ErrNotFound
	}

	return vol, nil
}

// ListVolumes lists all logical volumes in this volume group.
func (vg *VolumeGroup) ListVolumes(ctx context.Context) (map[string]*LogicalVolume, error) {
	return vg.listVolumes(ctx, "")
}

// listVolumes is the internal implementation for retrieving logical volumes and converting them to LogicalVolume instances.
// It is the backing implementation for both ListVolumes and FindVolume, since the lvs command can be used for both.
func (vg *VolumeGroup) listVolumes(ctx context.Context, name string) (map[string]*LogicalVolume, error) {
	ret := map[string]*LogicalVolume{}

	var lvs map[string]lv
	// use fast path if we have the lvs already through the report
	if vg.reportLvs != nil {
		lvs = vg.reportLvs
	} else {
		var err error
		// skip ErrNotFound because an empty list is a valid response
		if lvs, err = getLVs(ctx, vg, name); err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}

	for _, lv := range lvs {
		if !lv.isThinPool() {
			ret[lv.name] = vg.convertLV(lv)
		}
	}
	return ret, nil
}

func (vg *VolumeGroup) convertLV(lv lv) *LogicalVolume {
	size := lv.size

	var origin *string
	if len(lv.origin) > 0 {
		origin = &lv.origin
	}

	var pool *string
	if len(lv.poolLV) > 0 {
		pool = &lv.poolLV
	}

	if origin != nil && pool == nil {
		// this volume is a snapshot, but not a thin volume.
		size = lv.originSize
	}

	return &LogicalVolume{
		fullName(lv.name, vg),
		lv.name,
		lv.path,
		vg,
		size,
		origin,
		pool,
		uint32(lv.major),
		uint32(lv.minor),
		lv.tags,
		lv.attr,
	}
}

// CreateVolume creates logical volume in this volume group.
// name is a name of creating volume. size is volume size in bytes. volTags is a
// list of tags to add to the volume.
// lvcreateOptions are additional arguments to pass to lvcreate.
func (vg *VolumeGroup) CreateVolume(ctx context.Context, name string, size uint64, tags []string, stripe uint, stripeSize string,
	lvcreateOptions []string) error {

	if size%uint64(topolvm.MinimumSectorSize) != 0 {
		return ErrNoMultipleOfSectorSize
	}

	lvcreateArgs := []string{"lvcreate", "-n", name, "-L", fmt.Sprintf("%vb", size), "-W", "y", "-y"}
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
	lvcreateArgs = append(lvcreateArgs, vg.Name())

	return callLVM(ctx, lvcreateArgs...)
}

// FindPool finds a named thin pool in this volume group.
func (vg *VolumeGroup) FindPool(ctx context.Context, name string) (*ThinPool, error) {
	pools, err := vg.ListPools(ctx, name)
	if err != nil {
		return nil, err
	}
	pool, ok := pools[name]
	if !ok {
		return nil, ErrNotFound
	}

	return pool, nil
}

// ListPools lists all thin pool volumes in this volume group.
func (vg *VolumeGroup) ListPools(ctx context.Context, poolname string) (map[string]*ThinPool, error) {
	ret := map[string]*ThinPool{}

	var lvs map[string]lv
	// use fast path if we have the lvs already through the report
	if vg.reportLvs != nil {
		lvs = vg.reportLvs
	} else {
		var err error
		// skip ErrNotFound because an empty list is a valid response
		if lvs, err = getLVs(ctx, vg, poolname); err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}

	for _, lv := range lvs {
		if lv.isThinPool() {
			ret[lv.name] = newThinPool(vg, lv)
		}
	}
	return ret, nil
}

// CreatePool creates a pool for thin-provisioning volumes.
func (vg *VolumeGroup) CreatePool(ctx context.Context, name string, size uint64) (*ThinPool, error) {
	if err := callLVM(ctx, "lvcreate", "-T", fmt.Sprintf("%v/%v", vg.Name(), name),
		"--size", fmt.Sprintf("%vb", size)); err != nil {
		return nil, err
	}
	return vg.FindPool(ctx, name)
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

func (t ThinPoolUsage) FreeOverprovisionedBytes(overprovisionRatio float64) (uint64, error) {
	if overprovisionRatio < 1.0 {
		return 0, errors.New("overprovision ratio must be >= 1.0")
	}
	virtualPoolSize := uint64(math.Floor(overprovisionRatio * float64(t.SizeBytes)))
	if virtualPoolSize <= t.VirtualBytes {
		return 0, nil
	}
	return virtualPoolSize - t.VirtualBytes, nil
}

// FreePoolBytes pool's current data% usage
func (t ThinPoolUsage) FreePoolBytes() uint64 {
	return uint64(((100.0 - t.DataPercent) / 100.0) * float64(t.SizeBytes))
}

func fullName(name string, vg *VolumeGroup) string {
	return fmt.Sprintf("%v/%v", vg.Name(), name)
}

func newThinPool(vg *VolumeGroup, lvmLv lv) *ThinPool {
	return &ThinPool{
		vg,
		lvmLv,
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

	if newSize%uint64(topolvm.MinimumSectorSize) != 0 {
		return ErrNoMultipleOfSectorSize
	}

	if err := callLVM(ctx, "lvresize", "-f", "-L", fmt.Sprintf("%vb", newSize), t.state.fullName); err != nil {
		return err
	}

	// now we need to update the size of this volume, as it might slightly differ from the creation argument due to rounding
	vol, err := t.vg.FindVolume(ctx, t.Name())
	if err != nil {
		return err
	}
	t.state.size = vol.size

	return nil
}

// ListVolumes lists all volumes in this thin pool.
func (t *ThinPool) ListVolumes(ctx context.Context) (map[string]*LogicalVolume, error) {
	volumes, err := t.vg.ListVolumes(ctx)
	filteredVolumes := make(map[string]*LogicalVolume, len(volumes))
	for _, volume := range volumes {
		if volume.pool != nil && *volume.pool == t.Name() {
			filteredVolumes[volume.Name()] = volume
		}
	}
	if err != nil {
		return nil, err
	}
	return filteredVolumes, nil
}

// FindVolume finds a named logical volume in this thin pool
func (t *ThinPool) FindVolume(ctx context.Context, name string) (*LogicalVolume, error) {
	volumeCandidate, err := t.vg.FindVolume(ctx, name)
	if err != nil {
		return nil, err
	}
	// volume exists in vg but is in no pool or different pool
	if volumeCandidate.pool == nil || *volumeCandidate.pool != t.Name() {
		return nil, ErrNotFound
	}
	return volumeCandidate, nil
}

// CreateVolume creates a thin volume from this pool.
func (t *ThinPool) CreateVolume(ctx context.Context, name string, size uint64, tags []string, stripe uint, stripeSize string, lvcreateOptions []string) error {
	lvcreateArgs := []string{
		"lvcreate",
		"-T",
		t.FullName(),
		"-n",
		name,
		"-V",
		fmt.Sprintf("%vb", size),
		"-W",
		"y",
		"-y",
	}
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

	return callLVM(ctx, lvcreateArgs...)
}

// Usage on a thinpool returns used data, metadata percentages,
// sum of virtualsizes of all thinlvs and size of thinpool
func (t *ThinPool) Usage(ctx context.Context) (*ThinPoolUsage, error) {
	lvs, err := t.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}
	tpu := &ThinPoolUsage{}
	tpu.DataPercent = t.state.dataPercent
	tpu.MetadataPercent = t.state.metaDataPercent
	tpu.SizeBytes = t.state.size
	for _, l := range lvs {
		tpu.VirtualBytes += l.size
	}
	return tpu, nil
}

func CheckCapacity(requestedBytes uint64, freeBytes uint64, skipOverprovisioning bool) bool {
	if skipOverprovisioning {
		return freeBytes > 0
	} else {
		return requestedBytes <= freeBytes
	}
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
	attr     string
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
func (l *LogicalVolume) Origin(ctx context.Context) (*LogicalVolume, error) {
	if l.origin == nil {
		return nil, nil
	}
	return l.vg.FindVolume(ctx, *l.origin)
}

// IsThin checks if the volume is thin volume or not.
func (l *LogicalVolume) IsThin() bool {
	return l.attr[0] == byte(VolumeTypeThinVolume)
}

// Pool returns thin pool if this is a thin pool, or nil if not.
func (l *LogicalVolume) Pool(ctx context.Context) (*ThinPool, error) {
	if l.pool == nil {
		return nil, nil
	}
	return l.vg.FindPool(ctx, *l.pool)
}

// MajorNumber returns the device major number.
func (l *LogicalVolume) MajorNumber() uint32 {
	return l.devMajor
}

// MinorNumber returns the device minor number.
func (l *LogicalVolume) MinorNumber() uint32 {
	return l.devMinor
}

// Tags returns the tags of the logical volume.
func (l *LogicalVolume) Tags() []string {
	return l.tags
}

// Attr returns the attr flag field of the logical volume.
func (l *LogicalVolume) Attr() string {
	return l.attr
}

// ThinSnapshot takes a thin snapshot of a volume.
// The volume must be thinly-provisioned.
// snapshots can be created unconditionally.
func (l *LogicalVolume) ThinSnapshot(ctx context.Context, name string, tags []string) error {
	if !l.IsThin() {
		return fmt.Errorf("cannot take snapshot of non-thin volume: %s", l.fullname)
	}

	lvcreateArgs := []string{"lvcreate", "-s", "-k", "n", "-n", name, l.fullname}

	for _, tag := range tags {
		lvcreateArgs = append(lvcreateArgs, "--addtag")
		lvcreateArgs = append(lvcreateArgs, tag)
	}

	return callLVM(ctx, lvcreateArgs...)
}

// Activate activates the logical volume for desired access.
func (l *LogicalVolume) Activate(ctx context.Context, access string) error {
	var lvchangeArgs []string
	switch access {
	case "ro":
		lvchangeArgs = []string{"lvchange", "-p", "r", l.path}
	case "rw":
		lvchangeArgs = []string{"lvchange", "-k", "n", "-a", "y", l.path}
	default:
		return fmt.Errorf("unknown access: %s for LogicalVolume %s", access, l.fullname)
	}

	return callLVM(ctx, lvchangeArgs...)
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

	// now we need to update the size of this volume, as it might slightly differ from the creation argument due to rounding
	vol, err := l.vg.FindVolume(ctx, l.name)
	if err != nil {
		return err
	}
	l.size = vol.size

	return nil
}

// RemoveVolume removes the given volume from the volume group.
func (vg *VolumeGroup) RemoveVolume(ctx context.Context, name string) error {
	err := callLVM(ctx, "lvremove", "-f", fullName(name, vg))

	if IsLVMNotFound(err) {
		return errors.Join(ErrNotFound, err)
	}

	return err
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
