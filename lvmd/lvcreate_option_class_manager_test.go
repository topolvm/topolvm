package lvmd

import (
	"strconv"
	"testing"
)

func TestLvcreateOptionClassManager(t *testing.T) {
	cases := []struct {
		found                 bool
		name                  string
		lvcreateOptionClasses []*LvcreateOptionClass
	}{
		{
			found: true,
			name:  "ssd",
			lvcreateOptionClasses: []*LvcreateOptionClass{
				{
					Name:    "ssd",
					Options: []string{"--type=raid1"},
				},
			},
		},
		{
			found: false,
			name:  "not-found",
			lvcreateOptionClasses: []*LvcreateOptionClass{
				{
					Name:    "ssd",
					Options: []string{"--type=raid1"},
				},
			},
		},
	}

	for i, c := range cases {
		ocm := NewLvcreateOptionClassManager(c.lvcreateOptionClasses)
		oc := ocm.LvcreateOptionClass(c.name)
		if c.found && oc == nil {
			t.Fatal(strconv.Itoa(i) + ": should be invalid")
		}
	}
}
