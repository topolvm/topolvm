package scheduler

import "testing"

func TestCapacityToScore(t *testing.T) {
	testCases := []struct {
		input   uint64
		divisor float64
		expect  int
	}{
		{0, 1, 0},
		{1, 1, 0},
		{128 << 30, 1, 7},
		{128 << 30, 2, 6},
		{128 << 30, 0.5, 8},
		{^uint64(0), 1, 10},
	}

	for _, tt := range testCases {
		score := capacityToScore(tt.input, tt.divisor)
		if score != tt.expect {
			t.Errorf("score incorrect: input=%d expect=%d actual=%d",
				tt.input,
				tt.expect,
				score,
			)
		}
	}
}
