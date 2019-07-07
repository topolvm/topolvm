package filesystem

import "sort"

// List returns a list of supported filesystem types.
func List() []string {
	t := make([]string, 0, len(fsTypeMap))
	for k := range fsTypeMap {
		t = append(t, k)
	}
	sort.Strings(t)
	return t
}
