package driver

import (
	"testing"
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

	v, err := convertRequestCapacityBytes(0, 10)
	if err != nil {
		t.Error("should not be error")
	}
	if v != 10 {
		t.Errorf("should be the limit capacity by default if 0 is supplied and limit is smaller than 1Gi: %d", v)
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
	if v != 1 {
		t.Errorf("should be resolve capacities < 1Gi without error if there is no limit: %d", v)
	}

	v, err = convertRequestCapacityBytes(1<<30, 1<<30)
	if err != nil {
		t.Error("should not be error")
	}
	if v != 1<<30 {
		t.Errorf("should be 1073741824 in byte precision: %d", v)
	}

	v, err = convertRequestCapacityBytes(1<<30+1, 1<<30+1)
	if err != nil {
		t.Error("should not be error")
	}
	if v != (1<<30)+1 {
		t.Errorf("should be 1073741825 in byte precision: %d", v)
	}

	v, err = convertRequestCapacityBytes(0, 0)
	if err != nil {
		t.Error("should not be error")
	}
	if v != 1<<30 {
		t.Errorf("should be 1073741825 in byte precision: %d", v)
	}
}
