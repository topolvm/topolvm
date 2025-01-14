package command

import (
	"errors"
	"fmt"
)

var (
	ErrPartialActivation                         = errors.New("found partial activation of physical volumes, one or more physical volumes are setup incorrectly")
	ErrUnknownVolumeHealth                       = errors.New("unknown volume health reported, verification on the host system is required")
	ErrWriteCacheError                           = errors.New("write cache error signifies that dm-writecache reports an error")
	ErrThinPoolFailed                            = errors.New("thin pool encounters serious failures and hence no further I/O is permitted at all")
	ErrThinPoolOutOfDataSpace                    = errors.New("thin pool is out of data space, no further data can be written to the thin pool without extension")
	ErrThinPoolMetadataReadOnly                  = errors.New("metadata read only signifies that thin pool encounters certain types of failures, but it's still possible to do data reads. However, no metadata changes are allowed")
	ErrThinVolumeFailed                          = errors.New("the underlying thin pool entered a failed state and no further I/O is permitted")
	ErrRAIDRefreshNeeded                         = errors.New("RAID volume requires a refresh, one or more Physical Volumes have suffered a write error. This could be due to temporary failure of the Physical Volume or an indication it is failing. The device should be refreshed or replaced")
	ErrRAIDMismatchesExist                       = errors.New("RAID volume has portions of the array that are not coherent. Inconsistencies are detected by initiating a check RAID logical volume. The scrubbing operations, \"check\" and \"repair\", can be performed on a RAID volume via the \"lvchange\" command")
	ErrRAIDReshaping                             = errors.New("RAID volume is currently reshaping. Reshaping signifies a RAID Logical Volume is either undergoing a stripe addition/removal, a stripe size or RAID algorithm change")
	ErrRAIDReshapeRemoved                        = errors.New("RAID volume signifies freed raid images after reshaping")
	ErrRAIDWriteMostly                           = errors.New("RAID volume is marked as write-mostly. this signifies the devices in a RAID 1 logical volume have been marked write-mostly. This means that reading from this device will be avoided, and other devices will be preferred for reading (unless no other devices are available). This minimizes the I/O to the specified device")
	ErrLogicalVolumeSuspended                    = errors.New("logical volume is in a suspended state, no I/O is permitted")
	ErrInvalidSnapshot                           = errors.New("logical volume is an invalid snapshot, no I/O is permitted")
	ErrSnapshotMergeFailed                       = errors.New("snapshot merge failed, no I/O is permitted")
	ErrMappedDevicePresentWithInactiveTables     = errors.New("mapped device present with inactive tables, no I/O is permitted")
	ErrMappedDevicePresentWithoutTables          = errors.New("mapped device present without tables, no I/O is permitted")
	ErrThinPoolCheckNeeded                       = errors.New("a thin pool check is needed")
	ErrUnknownVolumeState                        = errors.New("unknown volume state, verification on the host system is required")
	ErrHistoricalVolumeState                     = errors.New("historical volume state (volume no longer exists but is kept around in logs), verification on the host system is required")
	ErrLogicalVolumeUnderlyingDeviceStateUnknown = errors.New("logical volume underlying device state is unknown, verification on the host system is required")
)

type VolumeType rune

const (
	VolumeTypeCached                     VolumeType = 'C'
	VolumeTypeMirrored                   VolumeType = 'm'
	VolumeTypeMirroredNoInitialSync      VolumeType = 'M'
	VolumeTypeOrigin                     VolumeType = 'o'
	VolumeTypeOriginWithMergingSnapshot  VolumeType = 'O'
	VolumeTypeRAID                       VolumeType = 'r'
	VolumeTypeRAIDNoInitialSync          VolumeType = 'R'
	VolumeTypeSnapshot                   VolumeType = 's'
	VolumeTypeMergingSnapshot            VolumeType = 'S'
	VolumeTypePVMove                     VolumeType = 'p'
	VolumeTypeVirtual                    VolumeType = 'v'
	VolumeTypeMirrorOrRAIDImage          VolumeType = 'i'
	VolumeTypeMirrorOrRAIDImageOutOfSync VolumeType = 'I'
	VolumeTypeMirrorLogDevice            VolumeType = 'l'
	VolumeTypeUnderConversion            VolumeType = 'c'
	VolumeTypeThinVolume                 VolumeType = 'V'
	VolumeTypeThinPool                   VolumeType = 't'
	VolumeTypeThinPoolData               VolumeType = 'T'
	VolumeTypeThinPoolMetadata           VolumeType = 'e'
	VolumeTypeDefault                    VolumeType = '-'
)

type Permissions rune

const (
	PermissionsWriteable                             Permissions = 'w'
	PermissionsReadOnly                              Permissions = 'r'
	PermissionsReadOnlyActivationOfNonReadOnlyVolume Permissions = 'R'
	PermissionsNone                                  Permissions = '-'
)

type AllocationPolicy rune

const (
	AllocationPolicyAnywhere         AllocationPolicy = 'a'
	AllocationPolicyAnywhereLocked   AllocationPolicy = 'A'
	AllocationPolicyContiguous       AllocationPolicy = 'c'
	AllocationPolicyContiguousLocked AllocationPolicy = 'C'
	AllocationPolicyInherited        AllocationPolicy = 'i'
	AllocationPolicyInheritedLocked  AllocationPolicy = 'I'
	AllocationPolicyCling            AllocationPolicy = 'l'
	AllocationPolicyClingLocked      AllocationPolicy = 'L'
	AllocationPolicyNormal           AllocationPolicy = 'n'
	AllocationPolicyNormalLocked     AllocationPolicy = 'N'
	AllocationPolicyNone             AllocationPolicy = '-'
)

type Minor rune

const (
	MinorTrue  Minor = 'm'
	MinorFalse Minor = '-'
)

type State rune

const (
	StateActive                                State = 'a'
	StateSuspended                             State = 's'
	StateInvalidSnapshot                       State = 'I'
	StateSuspendedSnapshot                     State = 'S'
	StateSnapshotMergeFailed                   State = 'm'
	StateSuspendedSnapshotMergeFailed          State = 'M'
	StateMappedDevicePresentWithoutTables      State = 'd'
	StateMappedDevicePresentWithInactiveTables State = 'i'
	StateNone                                  State = '-'
	StateHistorical                            State = 'h'
	StateThinPoolCheckNeeded                   State = 'c'
	StateSuspendedThinPoolCheckNeeded          State = 'C'
	StateUnknown                               State = 'X'
)

type Open rune

const (
	OpenTrue    Open = 'o'
	OpenFalse   Open = '-'
	OpenUnknown Open = 'X'
)

type OpenTarget rune

const (
	OpenTargetCache    OpenTarget = 'C'
	OpenTargetMirror   OpenTarget = 'm'
	OpenTargetRaid     OpenTarget = 'r'
	OpenTargetSnapshot OpenTarget = 's'
	OpenTargetThin     OpenTarget = 't'
	OpenTargetUnknown  OpenTarget = 'u'
	OpenTargetVirtual  OpenTarget = 'v'
	OpenTargetNone     OpenTarget = '-'
)

type Zero rune

const (
	ZeroTrue  Zero = 'z'
	ZeroFalse Zero = '-'
)

type VolumeHealth rune

const (
	VolumeHealthPartialActivation        VolumeHealth = 'p'
	VolumeHealthUnknown                  VolumeHealth = 'X'
	VolumeHealthOK                       VolumeHealth = '-'
	VolumeHealthRAIDRefreshNeeded        VolumeHealth = 'r'
	VolumeHealthRAIDMismatchesExist      VolumeHealth = 'm'
	VolumeHealthRAIDWriteMostly          VolumeHealth = 'w'
	VolumeHealthRAIDReshaping            VolumeHealth = 's'
	VolumeHealthRAIDReshapeRemoved       VolumeHealth = 'R'
	VolumeHealthThinFailed               VolumeHealth = 'F'
	VolumeHealthThinPoolOutOfDataSpace   VolumeHealth = 'D'
	VolumeHealthThinPoolMetadataReadOnly VolumeHealth = 'M'
	VolumeHealthWriteCacheError          VolumeHealth = 'E'
)

type SkipActivation rune

const (
	SkipActivationTrue  SkipActivation = 'k'
	SkipActivationFalse SkipActivation = '-'
)

// LVAttr has mapped lv_attr information, see https://linux.die.net/man/8/lvs
// It is a complete parsing of the entire attribute byte flags that is attached to each LV.
// This is useful when attaching logic to the state of an LV as the state of an LV can be determined
// from the Attributes, e.g. for determining whether an LV is considered a Thin-Pool or not.
type LVAttr struct {
	VolumeType
	Permissions
	AllocationPolicy
	Minor
	State
	Open
	OpenTarget
	Zero
	VolumeHealth
	SkipActivation
}

const lvAttrLength = 10

func ParsedLVAttr(raw string) (*LVAttr, error) {
	if len(raw) != lvAttrLength {
		return nil, fmt.Errorf("%s is an invalid length lv_attr, expected %v, but got %v",
			raw, lvAttrLength, len(raw))
	}
	return &LVAttr{
		VolumeType(raw[0]),
		Permissions(raw[1]),
		AllocationPolicy(raw[2]),
		Minor(raw[3]),
		State(raw[4]),
		Open(raw[5]),
		OpenTarget(raw[6]),
		Zero(raw[7]),
		VolumeHealth(raw[8]),
		SkipActivation(raw[9]),
	}, nil
}

func (l *LVAttr) String() string {
	return fmt.Sprintf(
		"%c%c%c%c%c%c%c%c%c%c",
		l.VolumeType,
		l.Permissions,
		l.AllocationPolicy,
		l.Minor,
		l.State,
		l.Open,
		l.OpenTarget,
		l.Zero,
		l.VolumeHealth,
		l.SkipActivation,
	)
}

// VerifyHealth checks the health of the logical volume based on the attributes, mainly
// bit 9 (volume health indicator) based on bit 1 (volume type indicator)
// All failed known states are reported with an error message.
func (l *LVAttr) VerifyHealth() error {
	if l.VolumeHealth == VolumeHealthPartialActivation {
		return ErrPartialActivation
	}
	if l.VolumeHealth == VolumeHealthUnknown {
		return ErrUnknownVolumeHealth
	}
	if l.VolumeHealth == VolumeHealthWriteCacheError {
		return ErrWriteCacheError
	}

	if l.VolumeType == VolumeTypeThinPool {
		switch l.VolumeHealth {
		case VolumeHealthThinFailed:
			return ErrThinPoolFailed
		case VolumeHealthThinPoolOutOfDataSpace:
			return ErrThinPoolOutOfDataSpace
		case VolumeHealthThinPoolMetadataReadOnly:
			return ErrThinPoolMetadataReadOnly
		}
	}

	if l.VolumeType == VolumeTypeThinVolume {
		switch l.VolumeHealth {
		case VolumeHealthThinFailed:
			return ErrThinVolumeFailed
		}
	}

	if l.VolumeType == VolumeTypeRAID || l.VolumeType == VolumeTypeRAIDNoInitialSync {
		switch l.VolumeHealth {
		case VolumeHealthRAIDRefreshNeeded:
			return ErrRAIDRefreshNeeded
		case VolumeHealthRAIDMismatchesExist:
			return ErrRAIDMismatchesExist
		case VolumeHealthRAIDReshaping:
			return ErrRAIDReshaping
		case VolumeHealthRAIDReshapeRemoved:
			return ErrRAIDReshapeRemoved
		case VolumeHealthRAIDWriteMostly:
			return ErrRAIDWriteMostly
		}
	}

	switch l.State {
	case StateSuspended, StateSuspendedSnapshot:
		return ErrLogicalVolumeSuspended
	case StateInvalidSnapshot:
		return ErrInvalidSnapshot
	case StateSnapshotMergeFailed, StateSuspendedSnapshotMergeFailed:
		return ErrSnapshotMergeFailed
	case StateMappedDevicePresentWithInactiveTables:
		return ErrMappedDevicePresentWithInactiveTables
	case StateMappedDevicePresentWithoutTables:
		return ErrMappedDevicePresentWithoutTables
	case StateThinPoolCheckNeeded, StateSuspendedThinPoolCheckNeeded:
		return ErrThinPoolCheckNeeded
	case StateUnknown:
		return ErrUnknownVolumeState
	case StateHistorical:
		return ErrHistoricalVolumeState
	}

	switch l.Open {
	case OpenUnknown:
		return ErrLogicalVolumeUnderlyingDeviceStateUnknown
	}

	return nil
}
