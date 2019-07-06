package k8s

import (
	"testing"

	"github.com/cybozu-go/topolvm/csi"
)

func TestIsValidVolumeCapabilities(t *testing.T) {
	validAccessMode := &csi.VolumeCapability_AccessMode{
		Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER}

	type args struct {
		requestCapabilities []*csi.VolumeCapability
		volumeMode          string
		fsType              string
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "valid capabilities for creating block device",
			args: args{
				requestCapabilities: []*csi.VolumeCapability{{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: validAccessMode,
				}},
				volumeMode: volumeModeBlock,
			},
			want: true,
		},
		{
			name: "valid capabilities for creating file system",
			args: args{
				requestCapabilities: []*csi.VolumeCapability{{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{FsType: "btrfs"},
					},
					AccessMode: validAccessMode,
				}},
				volumeMode: volumeModeMount,
				fsType:     "btrfs",
			},
			want: true,
		},
		{
			name: "valid capabilities for creating file system with blank fsType",
			args: args{
				requestCapabilities: []*csi.VolumeCapability{{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: validAccessMode,
				}},
				volumeMode: volumeModeMount,
				fsType:     defaultFsType,
			},
			want: true,
		},
		{
			name: "invalid case: topolvm only support single node writer",
			args: args{
				requestCapabilities: []*csi.VolumeCapability{{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER},
				}},
				volumeMode: volumeModeMount,
				fsType:     defaultFsType,
			},
			want: false,
		},
		{
			name: "invalid case: request volume mode and existing one are different",
			args: args{
				requestCapabilities: []*csi.VolumeCapability{{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: validAccessMode,
				}},
				volumeMode: volumeModeMount,
				fsType:     defaultFsType,
			},
			want: false,
		},
		{
			name: "invalid case: request fs type and existing one are different",
			args: args{
				requestCapabilities: []*csi.VolumeCapability{{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{FsType: defaultFsType},
					},
					AccessMode: validAccessMode,
				}},
				volumeMode: volumeModeMount,
				fsType:     "btrfs",
			},
			want: false,
		},
		{
			name: "invalid case: volume mode is not specified",
			args: args{
				requestCapabilities: []*csi.VolumeCapability{{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{FsType: defaultFsType},
					},
					AccessMode: validAccessMode,
				}},
				fsType: "btrfs",
			},
			want: false,
		},
		{
			name: "invalid case: contradictory argument, both 'mount' and 'block' volume mode are specified",
			args: args{
				requestCapabilities: []*csi.VolumeCapability{{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{FsType: defaultFsType},
					},
					AccessMode: validAccessMode,
				}, {
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: validAccessMode,
				}},
				volumeMode: volumeModeMount,
				fsType:     defaultFsType,
			},
			want: false,
		},
		{
			name: "invalid case: contradictory argument, multiple fs types are specified",
			args: args{
				requestCapabilities: []*csi.VolumeCapability{{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{FsType: defaultFsType},
					},
					AccessMode: validAccessMode,
				}, {
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{FsType: "btrfs"},
					},
					AccessMode: validAccessMode,
				}},
				volumeMode: volumeModeMount,
				fsType:     defaultFsType,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidVolumeCapabilities(tt.args.requestCapabilities, tt.args.volumeMode, tt.args.fsType); got != tt.want {
				t.Errorf("isValidVolumeCapabilities() = %v, want %v", got, tt.want)
			}
		})
	}
}
