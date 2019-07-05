package util

import "testing"

func TestIsMountpoint(t *testing.T) {
	cases := []struct {
		target      string
		expectError bool
		expectTrue  bool
	}{
		{"/", false, true},
		{"/proc", false, true},
		{"a8383", true, false},
		{"..", false, false},
	}

	for _, c := range cases {
		c := c
		t.Run(c.target, func(t *testing.T) {
			t.Parallel()
			ok, err := IsMountpoint(c.target)
			if err != nil {
				if !c.expectError {
					t.Error(err)
				}
				return
			}
			if ok != c.expectTrue {
				t.Error("unexpected result for", c.target)
			}
		})
	}
}
