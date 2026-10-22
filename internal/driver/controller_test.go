package driver

import (
	"errors"
	"fmt"
	"testing"

	"github.com/topolvm/topolvm"
	v1 "github.com/topolvm/topolvm/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Test_convertRequestCapacityBytes(t *testing.T) {
	testCases := []struct {
		requestBytes int64
		limitBytes   int64
		expected     int64
		err          error
	}{
		{
			requestBytes: -1,
			limitBytes:   10,
			err:          ErrNoNegativeRequestBytes,
		},
		{
			requestBytes: 10,
			limitBytes:   -1,
			err:          ErrNoNegativeLimitBytes,
		},
		{
			requestBytes: 20,
			limitBytes:   10,
			err:          ErrRequestedExceedsLimit,
		},
		{
			requestBytes: 1<<30 + 1,
			limitBytes:   1<<30 + 1,
			err:          ErrRequestedExceedsLimit,
		},
		{
			requestBytes: 0,
			limitBytes:   topolvm.MinimumSectorSize - 1,
			err:          ErrResultingRequestIsZero,
		},
		{
			requestBytes: 0,
			limitBytes:   topolvm.MinimumSectorSize + 1,
			expected:     topolvm.MinimumSectorSize,
		},
		{
			requestBytes: 1,
			limitBytes:   topolvm.MinimumSectorSize * 2,
			expected:     topolvm.MinimumSectorSize,
		},
		{
			requestBytes: 0,
			limitBytes:   2 << 30,
			expected:     1 << 30,
		},
		{
			requestBytes: 1,
			limitBytes:   0,
			expected:     topolvm.MinimumSectorSize,
		},
		{
			requestBytes: 1 << 30,
			limitBytes:   1 << 30,
			expected:     1 << 30,
		},
		{
			requestBytes: 0,
			limitBytes:   0,
			expected:     1 << 30,
		},
	}

	for _, tc := range testCases {
		tcName := fmt.Sprintf("request:%d limit:%d", tc.requestBytes, tc.limitBytes)
		if tc.err != nil {
			tcName += fmt.Sprintf(" = %s", tc.err)
		} else {
			tcName += fmt.Sprintf(" = %v", tc.expected)
		}

		t.Run(tcName, func(t *testing.T) {
			v, err := convertRequestCapacityBytes(tc.requestBytes, tc.limitBytes)
			if !errors.Is(err, tc.err) {
				t.Errorf("expected error %v, but got %v", tc.err, err)
			}
			if v != tc.expected {
				t.Errorf("expected %d, but got %d", tc.expected, v)
			}
		})
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

func Test_notEnoughSpaceError(t *testing.T) {
	testCases := []struct {
		statusCode   codes.Code
		currentSize  *resource.Quantity
		currentBytes int64
		expected     codes.Code
	}{
		{codes.OK, nil, 2 << 30, codes.ResourceExhausted},
		{codes.OK, resource.NewQuantity(1<<30, resource.BinarySI), 1 << 30, codes.ResourceExhausted},
		{codes.OK, resource.NewQuantity(2<<30, resource.BinarySI), 2 << 30, codes.OutOfRange},
		{codes.ResourceExhausted, nil, 2 << 30, codes.OutOfRange},
		{codes.ResourceExhausted, resource.NewQuantity(1<<30, resource.BinarySI), 1 << 30, codes.OutOfRange},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("status.code:%s status.currentSize:%v = %s", tc.statusCode, tc.currentSize, tc.expected)
		t.Run(name, func(t *testing.T) {
			lv := &v1.LogicalVolume{
				Spec:   v1.LogicalVolumeSpec{Size: *resource.NewQuantity(2<<30, resource.BinarySI)},
				Status: v1.LogicalVolumeStatus{Code: tc.statusCode, CurrentSize: tc.currentSize},
			}
			err := notEnoughSpaceError(lv, 3<<30+512, tc.currentBytes, 1<<29+1)
			if code := status.Code(err); code != tc.expected {
				t.Errorf("expected %s, but got %s", tc.expected, code)
			}
			expectedMessage := fmt.Sprintf("not enough space: requested=3221225984 bytes, current=%d bytes, available=536870913 bytes", tc.currentBytes)
			if tc.expected == codes.ResourceExhausted {
				expectedMessage += "; volume expansion is pending"
			}
			if message := status.Convert(err).Message(); message != expectedMessage {
				t.Errorf("expected message %q, but got %q", expectedMessage, message)
			}
		})
	}
}
