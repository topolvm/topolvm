package scheduler

import "testing"

func TestCapacityToScore(t *testing.T) {
	testCases := []struct {
		input  uint64
		expect int
	}{
		{100 << 30, 2},
		{1, 0},
		{^uint64(0), 10},
	}

	for _, tt := range testCases {
		score := capacityToScore(tt.input)
		if score != tt.expect {
			t.Errorf("score incorrect: input=%d expect=%d actual=%d",
				tt.input,
				tt.expect,
				score,
			)
		}
	}
}
