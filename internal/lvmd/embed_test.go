package lvmd

import (
	"context"
	"testing"

	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	lvmdTypes "github.com/topolvm/topolvm/pkg/lvmd/types"
)

func TestNewEmbeddedServiceClients(t *testing.T) {
	overprovisionRatio := float64(2.0)
	tests := []struct {
		name          string
		deviceClasses []*lvmdTypes.DeviceClass
	}{

		{"volumegroup", []*lvmdTypes.DeviceClass{
			{
				Name:        "dc",
				VolumeGroup: "test_vgservice",
			}},
		},
		{"thinpool", []*lvmdTypes.DeviceClass{
			{
				Name:        "dc",
				VolumeGroup: "test_vgservice",
				Type:        lvmdTypes.TypeThin,
				ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
					Name:               "test_pool",
					OverprovisionRatio: overprovisionRatio,
				},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, vgclient := NewEmbeddedServiceClients(ctx, NewDeviceClassManager(tt.deviceClasses), NewLvcreateOptionClassManager(nil))

			watchClient, err := vgclient.Watch(ctx, &proto.Empty{}, nil)
			if err != nil {
				t.Fatal(err)
			}

			go func() {
				if err := watchClient.SendMsg(&proto.WatchResponse{
					FreeBytes: 1,
				}); err != nil {
					t.Error(err)
					return
				}
			}()

			res, err := watchClient.Recv()
			if err != nil {
				t.Fatal(err)
			}
			if res.FreeBytes != 1 {
				t.Fatal("no free byte set")
			}

		})
	}
}
