package controller

import (
	"testing"
	"time"
)

func TestValidateLVMOperationTimeout(t *testing.T) {
	for _, tc := range []struct {
		timeout time.Duration
		valid   bool
	}{
		{0, true},
		{MinLVMOperationTimeout, true},
		{10 * time.Minute, true},
		{MinLVMOperationTimeout - time.Second, false},
		{time.Minute, false},
		{-time.Second, false},
	} {
		err := ValidateLVMOperationTimeout(tc.timeout)
		if valid := err == nil; valid != tc.valid {
			t.Errorf("ValidateLVMOperationTimeout(%s): got error %v, want valid=%v", tc.timeout, err, tc.valid)
		}
	}
}
