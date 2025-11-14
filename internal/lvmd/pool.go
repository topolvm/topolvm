package lvmd

import (
	"context"
	"fmt"

	"github.com/topolvm/topolvm/internal/lvmd/command"
	lvmdTypes "github.com/topolvm/topolvm/pkg/lvmd/types"
)

type storagePool interface {
	Free(ctx context.Context) (uint64, error)
	ListVolumes(ctx context.Context) (map[string]*command.LogicalVolume, error)
	FindVolume(ctx context.Context, name string) (*command.LogicalVolume, error)
	CreateVolume(ctx context.Context, name string, size uint64, tags []string, stripe uint, stripeSize string, lvcreateOptions []string) error
}

func storagePoolForDeviceClass(ctx context.Context, dc *lvmdTypes.DeviceClass) (storagePool, error) {
	vg, err := command.FindVolumeGroup(ctx, dc.VolumeGroup)
	if err != nil {
		return nil, err
	}
	switch dc.Type {
	case lvmdTypes.TypeThick:
		return &volumeGroupAdapter{vg}, nil
	case lvmdTypes.TypeThin:
		pool, err := vg.FindPool(ctx, dc.ThinPoolConfig.Name)
		if err != nil {
			return nil, err
		}
		return &thinPoolAdapter{pool, dc.ThinPoolConfig.OverprovisionRatio}, nil
	}

	return nil, fmt.Errorf("unsupported device class target: %s", dc.Type)
}

type thinPoolAdapter struct {
	*command.ThinPool
	overprovisionRatio *float64
}

func (p *thinPoolAdapter) Free(ctx context.Context) (uint64, error) {
	usage, err := p.Usage(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get free space: %w", err)
	}

	return usage.FreeBytes(p.overprovisionRatio)
}

func CheckCapacity(pool storagePool, requestedBytes uint64, freeBytes uint64) bool {
	switch p := pool.(type) {
	case *thinPoolAdapter:
		return command.CheckCapacity(requestedBytes, freeBytes, p.overprovisionRatio != nil)
	default:
		return requestedBytes <= freeBytes
	}
}

type volumeGroupAdapter struct {
	*command.VolumeGroup
}

func (vg *volumeGroupAdapter) Free(_ context.Context) (uint64, error) {
	return vg.VolumeGroup.Free()
}
