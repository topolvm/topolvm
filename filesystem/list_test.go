package filesystem

import "testing"

func TestList(t *testing.T) {
	fsTypes := List()
	if len(fsTypes) == 0 {
		t.Error("empty list")
	}
	t.Log(fsTypes)
}
