package command

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestParsedLVAttr(t *testing.T) {
	noError := func(t *testing.T, err error, args ...string) bool {
		if err != nil {
			t.Helper()
			out := fmt.Sprintf("received unexpected error: %v", err)
			if len(args) > 0 {
				out = fmt.Sprintf("%s, %v", out, strings.Join(args, ","))
			}
			t.Error(out)
		}

		return true
	}

	type args struct {
		raw string
	}
	tests := []struct {
		name    string
		args    args
		want    LVAttr
		wantErr func(*testing.T, error, ...string) bool
	}{
		{
			"Basic LV",
			args{raw: "-wi-a-----"},
			LVAttr{
				VolumeType:       VolumeTypeDefault,
				Permissions:      PermissionsWriteable,
				AllocationPolicy: AllocationPolicyInherited,
				Minor:            MinorFalse,
				State:            StateActive,
				Open:             OpenFalse,
				OpenTarget:       OpenTargetNone,
				Zero:             ZeroFalse,
				VolumeHealth:     VolumeHealthOK,
				SkipActivation:   SkipActivationFalse,
			},
			noError,
		},
		{
			"RAID Config without Initial Sync",
			args{raw: "Rwi-a-r---"},
			LVAttr{
				VolumeType:       VolumeTypeRAIDNoInitialSync,
				Permissions:      PermissionsWriteable,
				AllocationPolicy: AllocationPolicyInherited,
				Minor:            MinorFalse,
				State:            StateActive,
				Open:             OpenFalse,
				OpenTarget:       OpenTargetRaid,
				Zero:             ZeroFalse,
				VolumeHealth:     VolumeHealthOK,
				SkipActivation:   SkipActivationFalse,
			},
			noError,
		},
		{
			"ThinPool with Zeroing",
			args{raw: "twi-a-tz--"},
			LVAttr{
				VolumeType:       VolumeTypeThinPool,
				Permissions:      PermissionsWriteable,
				AllocationPolicy: AllocationPolicyInherited,
				Minor:            MinorFalse,
				State:            StateActive,
				Open:             OpenFalse,
				OpenTarget:       OpenTargetThin,
				Zero:             ZeroTrue,
				VolumeHealth:     VolumeHealthOK,
				SkipActivation:   SkipActivationFalse,
			},
			noError,
		},
		{
			"Cache",
			args{raw: "Cwi-aoC---"},
			LVAttr{
				VolumeType:       VolumeTypeCached,
				Permissions:      PermissionsWriteable,
				AllocationPolicy: AllocationPolicyInherited,
				Minor:            MinorFalse,
				State:            StateActive,
				Open:             OpenTrue,
				OpenTarget:       OpenTargetCache,
				Zero:             ZeroFalse,
				VolumeHealth:     VolumeHealthOK,
				SkipActivation:   SkipActivationFalse,
			},
			noError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsedLVAttr(tt.args.raw)
			if !tt.wantErr(t, err, fmt.Sprintf("ParsedLVAttr(%v)", tt.args.raw)) {
				return
			}
			if (&tt.want).String() != got.String() {
				t.Errorf("ParsedLVAttr() = %v, want %v, raw %v", got, tt.want, tt.args.raw)
			}
		})
	}
}

func TestVerifyHealth(t *testing.T) {
	tests := []struct {
		name    string
		rawAttr string
		wantErr error
	}{
		{
			name:    "Partial Activation",
			rawAttr: "--------p-",
			wantErr: ErrPartialActivation,
		},
		{
			name:    "Unknown Volume Health",
			rawAttr: "--------X-",
			wantErr: ErrUnknownVolumeHealth,
		},
		{
			name:    "Write Cache Error",
			rawAttr: "--------E-",
			wantErr: ErrWriteCacheError,
		},
		{
			name:    "Thin Pool Failed",
			rawAttr: "t-------F-",
			wantErr: ErrThinPoolFailed,
		},
		{
			name:    "Thin Pool Out of Data Space",
			rawAttr: "t-------D-",
			wantErr: ErrThinPoolOutOfDataSpace,
		},
		{
			name:    "Thin Volume Failed",
			rawAttr: "V-------F-",
			wantErr: ErrThinVolumeFailed,
		},
		{
			name:    "RAID Refresh Needed",
			rawAttr: "r-------r-",
			wantErr: ErrRAIDRefreshNeeded,
		},
		{
			name:    "RAID Mismatches Exist",
			rawAttr: "r-------m-",
			wantErr: ErrRAIDMismatchesExist,
		},
		{
			name:    "RAID Reshaping",
			rawAttr: "r-------s-",
			wantErr: ErrRAIDReshaping,
		},
		{
			name:    "RAID Reshape Removed",
			rawAttr: "r-------R-",
			wantErr: ErrRAIDReshapeRemoved,
		},
		{
			name:    "RAID Write Mostly",
			rawAttr: "r-------w-",
			wantErr: ErrRAIDWriteMostly,
		},
		{
			name:    "Logical Volume Suspended",
			rawAttr: "----s-----",
			wantErr: ErrLogicalVolumeSuspended,
		},
		{
			name:    "Invalid Snapshot",
			rawAttr: "----I-----",
			wantErr: ErrInvalidSnapshot,
		},
		{
			name:    "Snapshot Merge Failed",
			rawAttr: "----m-----",
			wantErr: ErrSnapshotMergeFailed,
		},
		{
			name:    "Mapped Device Present With Inactive Tables",
			rawAttr: "----i-----",
			wantErr: ErrMappedDevicePresentWithInactiveTables,
		},
		{
			name:    "Mapped Device Present Without Tables",
			rawAttr: "----d-----",
			wantErr: ErrMappedDevicePresentWithoutTables,
		},
		{
			name:    "Thin Pool Check Needed",
			rawAttr: "----c-----",
			wantErr: ErrThinPoolCheckNeeded,
		},
		{
			name:    "Unknown Volume State",
			rawAttr: "----X-----",
			wantErr: ErrUnknownVolumeState,
		},
		{
			name:    "Historical Volume State",
			rawAttr: "----h-----",
			wantErr: ErrHistoricalVolumeState,
		},
		{
			name:    "Logical Volume Underlying Device State Unknown",
			rawAttr: "-----X----",
			wantErr: ErrLogicalVolumeUnderlyingDeviceStateUnknown,
		},
		{
			name:    "Healthy Volume",
			rawAttr: "-wi-a-----",
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lvAttr, err := ParsedLVAttr(tt.rawAttr)
			if err != nil {
				t.Fatalf("ParsedLVAttr() error = %v", err)
			}
			if err := lvAttr.VerifyHealth(); !errors.Is(err, tt.wantErr) {
				t.Errorf("VerifyHealth() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLVAttrString(t *testing.T) {
	for _, tt := range []struct {
		name    string
		rawAttr string
		want    string
	}{
		{
			name:    "Basic LV",
			rawAttr: "-wi-a-----",
			want:    "-wi-a-----",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			attr, err := ParsedLVAttr(tt.rawAttr)
			if err != nil {
				t.Fatalf("ParsedLvAttr() error = %v", err)
			}
			if got := attr.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}
