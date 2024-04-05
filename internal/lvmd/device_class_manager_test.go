package lvmd

import (
	"strconv"
	"testing"

	lvmdTypes "github.com/topolvm/topolvm/pkg/lvmd/types"
)

func TestValidateDeviceClasses(t *testing.T) {
	stripe := uint(2)
	opRatio := float64(10.0)
	wrongOpRatio := float64(0.5)

	cases := []struct {
		deviceClasses []*lvmdTypes.DeviceClass
		valid         bool
	}{
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				{
					Name:        "dc1",
					VolumeGroup: "node1-myvg1",
					Default:     true,
				},
				{
					Name:        "dc2",
					VolumeGroup: "node1-myvg2",
				},
			},
			valid: true,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				{
					Name:        "stripe-size",
					VolumeGroup: "node1-myvg1",
					Stripe:      &stripe,
					StripeSize:  "4",
					Default:     true,
				},
			},
			valid: true,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				{
					Name:        "stripe-size-with-unit1",
					VolumeGroup: "node1-myvg1",
					Stripe:      &stripe,
					StripeSize:  "4m",
					Default:     true,
				},
				{
					Name:        "stripe-size-with-unit2",
					VolumeGroup: "node1-myvg2",
					Stripe:      &stripe,
					StripeSize:  "4G",
				},
			},
			valid: true,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				{
					Name:            "extra-options",
					VolumeGroup:     "node1-myvg1",
					Default:         true,
					LVCreateOptions: []string{"--mirrors=1"},
				},
				{
					Name:            "stripes-and-options",
					VolumeGroup:     "node1-myvg2",
					Stripe:          &stripe,
					StripeSize:      "4G",
					LVCreateOptions: []string{"--mirrors=1"},
				},
			},
			valid: true,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				{
					// dev0 -> vg0/pool0
					Name:        "dev0",
					VolumeGroup: "vg0",
					Default:     true,
					Type:        lvmdTypes.TypeThin,
					ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
						Name:               "pool0",
						OverprovisionRatio: opRatio,
					},
				},
				{
					// dev1 -> vg0/pool1
					// same volume group as in dev0
					Name:        "dev1",
					VolumeGroup: "vg0",
					Type:        lvmdTypes.TypeThin,
					ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
						Name:               "pool1",
						OverprovisionRatio: opRatio,
					},
				},
				{
					// dev3 -> vg1/pool0
					// different device-class and volumegroup but same thinpool
					// name as in dev0
					Name:        "dev3",
					VolumeGroup: "vg1",
					Type:        lvmdTypes.TypeThin,
					ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
						Name:               "pool0",
						OverprovisionRatio: opRatio,
					},
				},
			},
			valid: true,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				{
					// dev0 -> vg0/pool0
					Name:        "dev0",
					VolumeGroup: "vg0",
					Default:     true,
					Type:        lvmdTypes.TypeThin,
					ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
						Name:               "pool0",
						OverprovisionRatio: opRatio,
					},
				},
				{
					// dev1 -> vg0
					// same volume group as in dev0 with Type specified
					Name:        "dev1",
					VolumeGroup: "vg0",
					Type:        lvmdTypes.TypeThick,
				},
				{
					// dev2 -> vg1/pool0
					// different vg and different pool
					Name:        "dev2",
					VolumeGroup: "vg1",
					Type:        lvmdTypes.TypeThin,
					ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
						Name:               "pool1",
						OverprovisionRatio: opRatio,
					},
				},
				{
					// dev3 -> vg1
					// same volume group as in dev2 with Type not specified
					Name:        "dev3",
					VolumeGroup: "vg1",
				},
			},
			valid: true,
		},
		{
			// ThinPoolConfig should be ignored if Type is TypeThick
			deviceClasses: []*lvmdTypes.DeviceClass{
				{
					// dev0 -> vg0 since Type is TypeThick
					Name:        "dev0",
					VolumeGroup: "vg0",
					Default:     true,
					Type:        lvmdTypes.TypeThick,
					ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
						Name: "pool0",
					},
				},
			},
			valid: true,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				{
					Name:        "__invalid-device-class-name__",
					VolumeGroup: "node1-myvg1",
					Default:     true,
				},
			},
			valid: false,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				{
					Name:        "duplicate-name",
					VolumeGroup: "node1-myvg1",
					Default:     true,
				},
				{
					Name:        "duplicate-name",
					VolumeGroup: "node1-myvg2",
				},
			},
			valid: false,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				{
					Name:        "dc1",
					VolumeGroup: "node1-myvg1",
				},
				{
					Name:        "dc2",
					VolumeGroup: "node1-myvg2",
				},
			},
			valid: true,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				{
					Name:        "dc1",
					VolumeGroup: "node1-myvg1",
					Default:     true,
				},
				{
					Name:        "dc2",
					VolumeGroup: "node1-myvg2",
					Default:     true,
				},
			},
			valid: false,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{},
			valid:         false,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				{
					Name:        "invalid-stripe-size",
					VolumeGroup: "node1-myvg1",
					Stripe:      &stripe,
					StripeSize:  "4gib",
					Default:     true,
				},
			},
			valid: false,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				// effectively pointing to same volume group, since empty Type or
				// TypeThick considers thick volume creation on volume group
				{
					// dev0 -> vg0
					Name:        "dev0",
					VolumeGroup: "vg0",
					Default:     true,
					Type:        lvmdTypes.TypeThick,
					ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
						Name: "pool0",
					},
				},
				{
					// dev1 -> vg0
					Name:        "dev1",
					VolumeGroup: "vg0",
					ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
						OverprovisionRatio: opRatio,
					},
				},
			},
			valid: false,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				// incomplete thinpool info, no OverprovisionRatio
				{
					Name:        "dev0",
					VolumeGroup: "vg0",
					Default:     true,
					Type:        lvmdTypes.TypeThin,
					ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
						Name: "pool0",
					},
				},
			},
			valid: false,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				// incomplete thinpool info, no thinpool Name
				{
					Name:        "dev0",
					VolumeGroup: "vg0",
					Default:     true,
					Type:        lvmdTypes.TypeThin,
					ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
						OverprovisionRatio: opRatio,
					},
				},
			},
			valid: false,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				// no thinpool info
				{
					Name:        "dev0",
					VolumeGroup: "vg0",
					Default:     true,
					Type:        lvmdTypes.TypeThin,
				},
			},
			valid: false,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				// overprovision ratio should be > 1.0
				{
					Name:        "dev0",
					VolumeGroup: "vg0",
					Default:     true,
					Type:        lvmdTypes.TypeThin,
					ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
						Name:               "pool0",
						OverprovisionRatio: wrongOpRatio,
					},
				},
			},
			valid: false,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				// incorrect device-class target type
				{
					Name:        "dev0",
					VolumeGroup: "vg0",
					Default:     true,
					Type:        lvmdTypes.DeviceType("dummy"),
					ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
						Name:               "pool0",
						OverprovisionRatio: wrongOpRatio,
					},
				},
			},
			valid: false,
		},
		{
			deviceClasses: []*lvmdTypes.DeviceClass{
				// duplicate thin pools
				{
					// dev0 -> vg0
					Name:        "dev0",
					VolumeGroup: "vg0",
					Default:     true,
				},
				{
					// dev1 -> vg0/pool0
					Name:        "dev1",
					VolumeGroup: "vg0",
					Type:        lvmdTypes.TypeThin,
					ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
						Name:               "pool0",
						OverprovisionRatio: opRatio,
					},
				},
				{
					// dev2 -> vg0/pool0
					Name:        "dev2",
					VolumeGroup: "vg0",
					Type:        lvmdTypes.TypeThin,
					ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
						Name:               "pool0",
						OverprovisionRatio: opRatio,
					},
				},
			},
			valid: false,
		},
	}

	for i, c := range cases {
		err := ValidateDeviceClasses(c.deviceClasses)
		if c.valid && err != nil {
			t.Fatal(strconv.Itoa(i)+": should be valid: ", err)
		} else if !c.valid && err == nil {
			t.Fatal(strconv.Itoa(i) + ": should be invalid")
		}
	}
}

func TestDeviceClassManager(t *testing.T) {
	spare50gb := uint64(50)
	spare100gb := uint64(100)
	opRatio := float64(10.0)
	lvcreateOptions := []string{"--mirrors=1"}
	deviceClasses := []*lvmdTypes.DeviceClass{
		{
			Name:        "dc1",
			VolumeGroup: "vg1",
			Default:     true,
		},
		{
			Name:        "dc2",
			VolumeGroup: "vg2",
			SpareGB:     &spare50gb,
		},
		{
			Name:        "dc3",
			VolumeGroup: "vg3",
			SpareGB:     &spare100gb,
		},
		{
			Name:            "mirrors",
			VolumeGroup:     "vg2",
			LVCreateOptions: lvcreateOptions,
		},
		{
			// dev0 -> vg0/pool0
			Name:        "dev0",
			VolumeGroup: "vg0",
			SpareGB:     &spare100gb,
			Type:        lvmdTypes.TypeThin,
			ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
				Name:               "pool0",
				OverprovisionRatio: opRatio,
			},
		},
	}
	manager := NewDeviceClassManager(deviceClasses)

	dc, err := manager.DeviceClass("dc2")
	if err != nil {
		t.Fatal(err)
	}
	if GetSpare(dc) != spare50gb<<30 {
		t.Error("dc2's spare should be 50GB")
	}

	_, err = manager.DeviceClass("unknown")
	if err != ErrDeviceClassNotFound {
		t.Error("'unknown' should not be found")
	}

	dc, err = manager.FindDeviceClassByVGName("vg3")
	if err != nil {
		t.Fatal(err)
	}
	if GetSpare(dc) != spare100gb<<30 {
		t.Error("dc3's spare should be 100GB")
	}

	_, err = manager.FindDeviceClassByVGName("unknown")
	if err != ErrDeviceClassNotFound {
		t.Error("'unknown' should not be found")
	}

	dc, err = manager.DeviceClass("mirrors")
	if err != nil {
		t.Fatal(err)
	}
	for i := range dc.LVCreateOptions {
		if dc.LVCreateOptions[i] != lvcreateOptions[i] {
			t.Fatal("Wrong LVCreateOptions")
		}
	}

	dc = manager.defaultDeviceClass
	if dc == nil {
		t.Fatal("default not found")
	}
	if dc.Name != "dc1" {
		t.Fatal("wrong device-class found")
	}
	if GetSpare(dc) != defaultSpareGB<<30 {
		t.Error("dc1's spare should be default")
	}
	if dc.Type != lvmdTypes.TypeThick {
		t.Error("Default type should be TypeThick")
	}

	_, err = manager.DeviceClass("dev0")
	if err != nil {
		t.Fatal(err)
	}
}
