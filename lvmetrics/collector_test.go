package lvmetrics

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestCollector(t *testing.T) {
	var storage atomic.Value
	storage.Store(Metrics{AvailableBytes: 100})
	collector := NewCollector(&storage)
	const metadata = `
	# HELP topolvm_volumegroup_available_bytes LVM VG available bytes under lvmd management
	# TYPE topolvm_volumegroup_available_bytes gauge
	`
	expected := `

	topolvm_volumegroup_available_bytes %d
	`
	err := testutil.CollectAndCompare(collector, strings.NewReader(metadata+fmt.Sprintf(expected, 100)), "topolvm_volumegroup_available_bytes")
	if err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}

	storage.Store(Metrics{AvailableBytes: 200})

	err = testutil.CollectAndCompare(collector, strings.NewReader(metadata+fmt.Sprintf(expected, 200)), "topolvm_volumegroup_available_bytes")
	if err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}
