package lvmd

import (
	"strconv"
	"testing"
)

func TestValidateDeviceClasses(t *testing.T) {
	cases := []struct {
		deviceClasses []*DeviceClass
		valid         bool
	}{
		{
			deviceClasses: []*DeviceClass{
				{
					Name:        "hdd",
					VolumeGroup: "node1-myvg1",
				},
				{
					Name:        "ssd",
					VolumeGroup: "node1-myvg2",
					Default:     true,
				},
			},
			valid: true,
		},
		{
			deviceClasses: []*DeviceClass{
				{
					Name:        "__invalid-device-class-name__",
					VolumeGroup: "node1-myvg1",
					Default:     true,
				},
			},
			valid: false,
		},
		{
			deviceClasses: []*DeviceClass{
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
			deviceClasses: []*DeviceClass{
				{
					Name:        "hdd",
					VolumeGroup: "node1-hdd",
				},
				{
					Name:        "ssd",
					VolumeGroup: "node1-ssd",
				},
			},
			valid: false,
		},
		{
			deviceClasses: []*DeviceClass{
				{
					Name:        "hdd",
					VolumeGroup: "node1-hdd",
					Default:     true,
				},
				{
					Name:        "ssd",
					VolumeGroup: "node1-ssd",
					Default:     true,
				},
			},
			valid: false,
		},
		{
			deviceClasses: []*DeviceClass{},
			valid:         false,
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
	deviceClasses := []*DeviceClass{
		{
			Name:        "hdd1",
			VolumeGroup: "hdd1-vg",
			SpareGB:     &spare50gb,
		},
		{
			Name:        "hdd2",
			VolumeGroup: "hdd2-vg",
			SpareGB:     &spare100gb,
		},
		{
			Name:        "ssd",
			VolumeGroup: "ssd-vg",
			Default:     true,
		},
	}
	manager := NewDeviceClassManager(deviceClasses)

	dc, err := manager.DeviceClass("hdd1")
	if err != nil {
		t.Fatal(err)
	}
	if dc.GetSpare() != spare50gb<<30 {
		t.Error("hdd1's spare should be 50GB")
	}

	_, err = manager.DeviceClass("unknown")
	if err != ErrNotFound {
		t.Error("'unknown' should not be found")
	}

	dc, err = manager.FindDeviceClassByVGName("hdd2-vg")
	if err != nil {
		t.Fatal(err)
	}
	if dc.GetSpare() != spare100gb<<30 {
		t.Error("hdd2's spare should be 100GB")
	}

	_, err = manager.FindDeviceClassByVGName("unknown")
	if err != ErrNotFound {
		t.Error("'unknown' should not be found")
	}

	dc = manager.defaultDeviceClass
	if dc == nil {
		t.Fatal("default not found")
	}
	if dc.Name != "ssd" {
		t.Fatal("wrong device-class found")
	}
	if dc.GetSpare() != defaultSpareGB<<30 {
		t.Error("ssd's spare should be default")
	}
}
