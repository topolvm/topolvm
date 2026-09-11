package v1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestAddToScheme(t *testing.T) {
	s := runtime.NewScheme()
	if err := AddToScheme(s); err != nil {
		t.Fatalf("failed to add types to scheme: %v", err)
	}

	for _, obj := range []runtime.Object{&LogicalVolume{}, &LogicalVolumeList{}} {
		gvks, _, err := s.ObjectKinds(obj)
		if err != nil {
			t.Fatalf("failed to get object kinds of %T: %v", obj, err)
		}
		var found bool
		for _, gvk := range gvks {
			if gvk.GroupVersion() == GroupVersion {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%T is not registered with %s, got %v", obj, GroupVersion, gvks)
		}
	}
}
