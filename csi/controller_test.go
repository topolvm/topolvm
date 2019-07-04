package csi

import (
	"testing"
)

func TestController(t *testing.T) {
	_, err := convertRequestCapacity(-1, 10)
	if err == nil {
		t.Error("should be error")
	}

	_, err = convertRequestCapacity(10, -1)
	if err == nil {
		t.Error("should be error")
	}

	_, err = convertRequestCapacity(20, 10)
	if err == nil {
		t.Error("should be error")
	}

	v, err := convertRequestCapacity(0, 10)
	if err != nil {
		t.Error("should not be error")
	}
	if v != 1 {
		t.Errorf("should be 1: %d", v)
	}

	v, err = convertRequestCapacity(1, 0)
	if err != nil {
		t.Error("should not be error")
	}
	if v != 1 {
		t.Errorf("should be 1: %d", v)
	}

	v, err = convertRequestCapacity(1<<30, 1<<30)
	if err != nil {
		t.Error("should not be error")
	}
	if v != 1 {
		t.Errorf("should be 1: %d", v)
	}

	v, err = convertRequestCapacity(1<<30+1, 1<<30+1)
	if err != nil {
		t.Error("should not be error")
	}
	if v != 2 {
		t.Errorf("should be 2: %d", v)
	}
}
