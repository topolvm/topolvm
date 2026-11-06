package k8s

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"

	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// fakeLVReader serves LogicalVolumes from memory. It filters by the volume ID
// field selector like the cache index does, and honors Limit and Continue
// like the API server does.
type fakeLVReader struct {
	items     []topolvmv1.LogicalVolume
	listErr   error
	listCalls []client.ListOptions
}

func (r *fakeLVReader) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	return errors.New("not implemented")
}

func (r *fakeLVReader) List(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
	o := client.ListOptions{}
	o.ApplyOptions(opts)
	r.listCalls = append(r.listCalls, o)
	if r.listErr != nil {
		return r.listErr
	}

	items := r.items
	if o.FieldSelector != nil {
		volumeID, _ := o.FieldSelector.RequiresExactMatch(indexFieldVolumeID)
		items = nil
		for _, lv := range r.items {
			if lv.Status.VolumeID == volumeID {
				items = append(items, lv)
			}
		}
	}

	start := 0
	if o.Continue != "" {
		var err error
		start, err = strconv.Atoi(o.Continue)
		if err != nil {
			return err
		}
	}
	end := len(items)
	if o.Limit > 0 && start+int(o.Limit) < end {
		end = start + int(o.Limit)
	}

	lvList := list.(*topolvmv1.LogicalVolumeList)
	lvList.Items = append([]topolvmv1.LogicalVolume(nil), items[start:end]...)
	lvList.Continue = ""
	if end < len(items) {
		lvList.Continue = strconv.Itoa(end)
	}
	return nil
}

func makeLVs(n int) []topolvmv1.LogicalVolume {
	lvs := make([]topolvmv1.LogicalVolume, n)
	for i := range lvs {
		lvs[i] = topolvmv1.LogicalVolume{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("lv-%d", i)},
			Status:     topolvmv1.LogicalVolumeStatus{VolumeID: fmt.Sprintf("vol-%d", i)},
		}
	}
	return lvs
}

func TestVolumeGetterCacheHit(t *testing.T) {
	cache := &fakeLVReader{items: makeLVs(3)}
	api := &fakeLVReader{items: makeLVs(3)}
	getter := &volumeGetter{cacheReader: cache, apiReader: api}

	lv, err := getter.Get(context.Background(), "vol-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lv.Name != "lv-1" {
		t.Errorf("got %q, want lv-1", lv.Name)
	}
	if len(api.listCalls) != 0 {
		t.Errorf("direct reader called %d times on a cache hit", len(api.listCalls))
	}
}

func TestVolumeGetterPagesDirectList(t *testing.T) {
	total := 2*directListPageSize + 10
	wantPages := 3

	cases := []struct {
		name     string
		items    []topolvmv1.LogicalVolume
		volumeID string
		wantName string
		wantErr  error
	}{
		{
			name:     "found on the last page",
			items:    makeLVs(total),
			volumeID: fmt.Sprintf("vol-%d", total-1),
			wantName: fmt.Sprintf("lv-%d", total-1),
		},
		{
			name:     "not found",
			items:    makeLVs(total),
			volumeID: "vol-missing",
			wantErr:  ErrVolumeNotFound,
		},
		{
			name: "duplicates on different pages",
			items: func() []topolvmv1.LogicalVolume {
				lvs := makeLVs(total)
				lvs[total-1].Status.VolumeID = "vol-0"
				return lvs
			}(),
			volumeID: "vol-0",
			wantErr:  errors.New("multiple LogicalVolume is found for VolumeID vol-0"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := &fakeLVReader{items: tc.items}
			getter := &volumeGetter{cacheReader: &fakeLVReader{}, apiReader: api}

			lv, err := getter.Get(context.Background(), tc.volumeID)
			switch {
			case tc.wantErr != nil:
				if err == nil || err.Error() != tc.wantErr.Error() {
					t.Fatalf("got error %v, want %v", err, tc.wantErr)
				}
			case err != nil:
				t.Fatalf("unexpected error: %v", err)
			case lv.Name != tc.wantName:
				t.Errorf("got %q, want %q", lv.Name, tc.wantName)
			}

			if len(api.listCalls) != wantPages {
				t.Fatalf("direct reader called %d times, want %d", len(api.listCalls), wantPages)
			}
			for i, call := range api.listCalls {
				if call.Limit != directListPageSize {
					t.Errorf("call %d: limit %d, want %d", i, call.Limit, directListPageSize)
				}
			}
			if api.listCalls[0].Continue != "" {
				t.Errorf("first call sent continue token %q", api.listCalls[0].Continue)
			}
		})
	}
}

func TestVolumeGetterDirectListError(t *testing.T) {
	listErr := errors.New("boom")
	getter := &volumeGetter{
		cacheReader: &fakeLVReader{},
		apiReader:   &fakeLVReader{items: makeLVs(1), listErr: listErr},
	}

	if _, err := getter.Get(context.Background(), "vol-0"); !errors.Is(err, listErr) {
		t.Fatalf("got error %v, want %v", err, listErr)
	}
}
