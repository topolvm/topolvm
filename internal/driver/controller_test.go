package driver

import (
	"fmt"
	"testing"

	"github.com/topolvm/topolvm"
)

func Test_convertRequestCapacityBytes(t *testing.T) {
	_, err := convertRequestCapacityBytes(-1, 10)
	if err == nil {
		t.Error("should be error")
	}
	if err.Error() != "required capacity must not be negative" {
		t.Error("should report invalid required capacity")
	}

	_, err = convertRequestCapacityBytes(10, -1)
	if err == nil {
		t.Error("should be error")
	}
	if err.Error() != "capacity limit must not be negative" {
		t.Error("should report invalid capacity limit")
	}

	_, err = convertRequestCapacityBytes(20, 10)
	if err == nil {
		t.Error("should be error")
	}
	if err.Error() != "requested capacity exceeds limit capacity: request=20 limit=10" {
		t.Error("should report capacity limit exceeded")
	}

	v, err := convertRequestCapacityBytes(0, topolvm.MinimumSectorSize-1)
	if err == nil {
		t.Error("should be error")
	}
	if err.Error() != "requested capacity is 0, because it defaulted to the limit (4095) and was rounded down to the nearest sector size (4096). specify the limit to be at least 4096 bytes" {
		t.Errorf("should error if rounded-down limit is 0: %d", v)
	}

	v, err = convertRequestCapacityBytes(0, topolvm.MinimumSectorSize+1)
	if err != nil {
		t.Error("should not be error")
	}
	if v != topolvm.MinimumSectorSize {
		t.Errorf("should be nearest rounded down multiple of sector size if 0 is supplied and limit is larger than sector-size: %d", v)
	}

	v, err = convertRequestCapacityBytes(0, 2<<30)
	if err != nil {
		t.Error("should not be error")
	}
	if v != 1<<30 {
		t.Errorf("should be at least 1 Gi requested by default if 0 is supplied: %d", v)
	}

	v, err = convertRequestCapacityBytes(1, 0)
	if err != nil {
		t.Error("should not be error")
	}
	if v != topolvm.MinimumSectorSize {
		t.Errorf("should be resolve capacities < 1Gi without error if there is no limit: %d", v)
	}

	v, err = convertRequestCapacityBytes(1<<30, 1<<30)
	if err != nil {
		t.Error("should not be error")
	}
	if v != 1<<30 {
		t.Errorf("should be 1073741824 in byte precision: %d", v)
	}

	_, err = convertRequestCapacityBytes(1<<30+1, 1<<30+1)
	if err == nil {
		t.Error("should be error")
	}
	if err.Error() != "requested capacity rounded to nearest sector size (4096) exceeds limit capacity, either specify a lower request or a higher limit: request=1073745920 limit=1073741825" {
		t.Error("should report capacity limit exceeded after rounding")
	}

	v, err = convertRequestCapacityBytes(0, 0)
	if err != nil {
		t.Error("should not be error")
	}
	if v != 1<<30 {
		t.Errorf("should be 1073741825 in byte precision: %d", v)
	}

	v, err = convertRequestCapacityBytes(1, topolvm.MinimumSectorSize*2)
	if err != nil {
		t.Error("should not be error")
	}
	if v != topolvm.MinimumSectorSize {
		t.Errorf("should be %d in byte precision: %d", topolvm.MinimumSectorSize, v)
	}

}

func Test_roundUp(t *testing.T) {
	testCases := []struct {
		size     int64
		multiple int64
		expected int64
	}{
		{12, 4, 12},
		{11, 4, 12},
		{13, 4, 16},
		{0, 4, 0},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("nearest rounded up multiple of %d from %d should be %d", tc.multiple, tc.size, tc.expected)
		t.Run(name, func(t *testing.T) {
			rounded := roundUp(tc.size, tc.multiple)
			if rounded != tc.expected {
				t.Errorf("%s, but was %d", name, rounded)
			}
		})
	}
}
