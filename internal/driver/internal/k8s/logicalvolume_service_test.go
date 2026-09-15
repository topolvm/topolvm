package k8s

import (
	"context"
	"testing"
	"time"

	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// fakeAPI stands in for the API server. It reports the LogicalVolume as missing
// until it is created, then serves one scripted status per Get, repeating the
// last one once the script runs out.
type fakeAPI struct {
	client.Writer
	client.StatusClient

	statuses []topolvmv1.LogicalVolumeStatus

	created *topolvmv1.LogicalVolume
	gets    int
	deleted []string
}

func (f *fakeAPI) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	if f.created == nil {
		return apierrors.NewNotFound(schema.GroupResource{Resource: "logicalvolumes"}, key.Name)
	}

	i := min(f.gets, len(f.statuses)-1)
	f.gets++

	lv := obj.(*topolvmv1.LogicalVolume)
	lv.Name = key.Name
	lv.Status = *f.statuses[i].DeepCopy()
	return nil
}

func (f *fakeAPI) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	f.created = obj.(*topolvmv1.LogicalVolume)
	return nil
}

func (f *fakeAPI) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	f.deleted = append(f.deleted, obj.GetName())
	return nil
}

func newTestService(statuses ...topolvmv1.LogicalVolumeStatus) (*LogicalVolumeService, *fakeAPI) {
	api := &fakeAPI{statuses: statuses}
	return &LogicalVolumeService{writer: api, getter: api}, api
}

func TestCreateVolume(t *testing.T) {
	// With equal values the test could not tell the reported size from the
	// requested one.
	const (
		requestedBytes = 1<<30 + 1
		reportedBytes  = 1<<30 + 4<<20
	)

	provisioning := topolvmv1.LogicalVolumeStatus{
		VolumeID: "volume-id",
	}
	sized := topolvmv1.LogicalVolumeStatus{
		VolumeID:    "volume-id",
		CurrentSize: resource.NewQuantity(reportedBytes, resource.BinarySI),
	}
	// expandLV records its failures on a volume that already has a volumeID, so a
	// provisioned volume can carry a non-OK code while its size is still unknown.
	failedExpansion := topolvmv1.LogicalVolumeStatus{
		VolumeID: "volume-id",
		Code:     codes.Internal,
		Message:  "failed to resize",
	}

	t.Run("reports the size the node published, not the one it was asked for", func(t *testing.T) {
		s, api := newTestService(provisioning, sized)

		lv, err := s.CreateVolume(context.Background(), "node", "dc", "oc", "lv", "", requestedBytes)
		if err != nil {
			t.Fatalf("expected no error, but got %v", err)
		}
		if lv.Status.CurrentSize == nil {
			t.Fatal("expected status.currentSize to be reported, but it was nil")
		}
		if got := lv.Status.CurrentSize.Value(); got != reportedBytes {
			t.Errorf("expected the reported size %d, but got %d", int64(reportedBytes), got)
		}
		if api.gets < 2 {
			t.Errorf("expected the volume to be polled more than once, but it was read %d time(s)", api.gets)
		}
		if len(api.deleted) != 0 {
			t.Errorf("expected the LogicalVolume not to be deleted, but got %v", api.deleted)
		}
	})

	t.Run("keeps a provisioned volume while its size is missing instead of deleting it", func(t *testing.T) {
		s, api := newTestService(provisioning)

		// The wait runs until the context is canceled, so bound it here. The window
		// has to outlast the first backoff so that the volume is polled more than once.
		ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
		defer cancel()

		if _, err := s.CreateVolume(ctx, "node", "dc", "oc", "lv", "", requestedBytes); err == nil {
			t.Fatal("expected the wait to be cut short, but it returned successfully")
		}
		if len(api.deleted) != 0 {
			t.Errorf("expected the LogicalVolume not to be deleted, but got %v", api.deleted)
		}
		if api.gets < 2 {
			t.Errorf("expected the volume to be polled more than once, but it was read %d time(s)", api.gets)
		}
	})

	t.Run("reports the node's failure instead of waiting for a size it cannot produce", func(t *testing.T) {
		s, api := newTestService(failedExpansion)

		_, err := s.CreateVolume(context.Background(), "node", "dc", "oc", "lv", "", requestedBytes)
		if got := status.Code(err); got != codes.Internal {
			t.Fatalf("expected %s, but got %s (err: %v)", codes.Internal, got, err)
		}
		if msg := status.Convert(err).Message(); msg != "failed to resize" {
			t.Errorf("expected the node's message, but got %q", msg)
		}
		if len(api.deleted) != 0 {
			t.Errorf("expected the LogicalVolume not to be deleted, but got %v", api.deleted)
		}
	})
}
