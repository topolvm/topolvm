package capacitytemplate

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation"
)

func TestCapacityName(t *testing.T) {
	got := CapacityName("datadog", "remote-workers", "ephemeral-remote-data", "remote-ssd")
	if got != CapacityName("datadog", "remote-workers", "ephemeral-remote-data", "remote-ssd") {
		t.Fatal("name is not stable")
	}
	if !strings.HasPrefix(got, "topolvm-template-datadog-remote-workers-") {
		t.Fatalf("name %q lacks readable prefix", got)
	}
	if errs := validation.IsDNS1123Subdomain(got); len(errs) != 0 {
		t.Fatalf("name %q is invalid: %v", got, errs)
	}

	long := CapacityName(strings.Repeat("a", 63), strings.Repeat("b", 253), "ephemeral-remote-data", "remote-ssd")
	if len(long) > 253 {
		t.Fatalf("long name has length %d", len(long))
	}
	if errs := validation.IsDNS1123Subdomain(long); len(errs) != 0 {
		t.Fatalf("long name %q is invalid: %v", long, errs)
	}
}

func TestCapacityNameIdentityCollisionResistance(t *testing.T) {
	identities := [][4]string{
		{"a", "b-c", "sc", "dc"},
		{"a-b", "c", "sc", "dc"},
		{"a", "b", "c-sc", "dc"},
		{"a", "b", "sc", "c-dc"},
		{"other", "b", "sc", "dc"},
	}
	seen := make(map[string]struct{})
	for _, identity := range identities {
		name := CapacityName(identity[0], identity[1], identity[2], identity[3])
		if _, ok := seen[name]; ok {
			t.Fatalf("identity %q collided at %q", identity, name)
		}
		seen[name] = struct{}{}
	}
}
