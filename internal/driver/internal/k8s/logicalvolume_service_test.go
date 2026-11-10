package k8s

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// With equal values the test could not tell the reported size from the
// requested one.
const (
	requestedBytes = 1<<30 + 1
	reportedBytes  = 1<<30 + 4<<20
	sourceBytes    = 1 << 29
)

// newTestService returns a service whose LogicalVolume gets the given status as
// soon as CreateVolume creates it, since no node runs here to publish one.
func newTestService(t *testing.T, st topolvmv1.LogicalVolumeStatus) (*LogicalVolumeService, client.WithWatch) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := topolvmv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to build the scheme: %v", err)
	}

	c := interceptor.NewClient(
		fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&topolvmv1.LogicalVolume{}).Build(),
		interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				lv := obj.DeepCopyObject().(*topolvmv1.LogicalVolume)
				// Without a finalizer a deletion would remove the object and leave nothing to inspect.
				controllerutil.AddFinalizer(lv, topolvm.GetLogicalVolumeFinalizer())
				if err := c.Create(ctx, lv, opts...); err != nil {
					return err
				}
				lv.Status = *st.DeepCopy()
				return c.Status().Update(ctx, lv)
			},
		},
	)
	return &LogicalVolumeService{writer: c, getter: newRetryMissingGetter(c, c)}, c
}

func deletionTimestamp(t *testing.T, c client.Client) *metav1.Time {
	t.Helper()
	var lv topolvmv1.LogicalVolume
	if err := c.Get(context.Background(), client.ObjectKey{Name: "lv"}, &lv); err != nil {
		t.Fatalf("failed to get the LogicalVolume: %v", err)
	}
	return lv.DeletionTimestamp
}

func TestCreateVolume(t *testing.T) {
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
	// A thin snapshot starts at the size of its source, so a clone found again
	// before it was resized reports less than it asked for.
	undersized := topolvmv1.LogicalVolumeStatus{
		VolumeID:    "volume-id",
		CurrentSize: resource.NewQuantity(sourceBytes, resource.BinarySI),
	}
	undersizedAndFailed := *undersized.DeepCopy()
	undersizedAndFailed.Code = codes.Internal
	undersizedAndFailed.Message = "failed to resize"

	t.Run("reports the size the node published, not the one it was asked for", func(t *testing.T) {
		s, c := newTestService(t, sized)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		lv, err := s.CreateVolume(ctx, "node", "dc", "oc", "lv", "", requestedBytes)
		if err != nil {
			t.Fatalf("expected no error, but got %v", err)
		}
		if lv.Status.CurrentSize == nil {
			t.Fatal("expected status.currentSize to be reported, but it was nil")
		}
		if got := lv.Status.CurrentSize.Value(); got != reportedBytes {
			t.Errorf("expected the reported size %d, but got %d", int64(reportedBytes), got)
		}
		if got := deletionTimestamp(t, c); got != nil {
			t.Errorf("expected the LogicalVolume not to be deleted, but got %v", got)
		}
	})

	t.Run("keeps a provisioned volume while its size is missing instead of deleting it", func(t *testing.T) {
		s, c := newTestService(t, provisioning)

		// The wait runs until the context is canceled, so bound it here. The window
		// has to outlast the first backoff so that the volume is polled more than once.
		ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
		defer cancel()

		if _, err := s.CreateVolume(ctx, "node", "dc", "oc", "lv", "", requestedBytes); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected the wait to run until the deadline, but got %v", err)
		}
		if got := deletionTimestamp(t, c); got != nil {
			t.Errorf("expected the LogicalVolume not to be deleted, but got %v", got)
		}
	})

	t.Run("deletes the LogicalVolume when reporting a failed creation", func(t *testing.T) {
		s, c := newTestService(t, topolvmv1.LogicalVolumeStatus{Code: codes.Internal, Message: "failed to create"})

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := s.CreateVolume(ctx, "node", "dc", "oc", "lv", "", requestedBytes)
		if got := status.Code(err); got != codes.Internal {
			t.Fatalf("expected %s, but got %s (err: %v)", codes.Internal, got, err)
		}
		if got := deletionTimestamp(t, c); got == nil {
			t.Errorf("expected the LogicalVolume to be deleted, but got %v", got)
		}
	})

	t.Run("reports the node's failure instead of waiting for a size it cannot produce", func(t *testing.T) {
		s, c := newTestService(t, failedExpansion)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := s.CreateVolume(ctx, "node", "dc", "oc", "lv", "", requestedBytes)
		if got := status.Code(err); got != codes.Internal {
			t.Fatalf("expected %s, but got %s (err: %v)", codes.Internal, got, err)
		}
		if msg := status.Convert(err).Message(); msg != "failed to resize" {
			t.Errorf("expected the node's message, but got %q", msg)
		}
		if got := deletionTimestamp(t, c); got == nil {
			t.Errorf("expected the LogicalVolume to be deleted, but got %v", got)
		}
	})

	t.Run("keeps waiting while the reported size is below spec.size", func(t *testing.T) {
		s, c := newTestService(t, undersized)

		ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
		defer cancel()

		if _, err := s.CreateVolume(ctx, "node", "dc", "oc", "lv", "source", requestedBytes); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected the wait to run until the deadline, but got %v", err)
		}
		if got := deletionTimestamp(t, c); got != nil {
			t.Errorf("expected the LogicalVolume not to be deleted, but got %v", got)
		}
	})

	t.Run("keeps a LogicalVolume the node fixed after its failure was read", func(t *testing.T) {
		s, c := newTestService(t, undersizedAndFailed)
		s.writer = interceptor.NewClient(c, interceptor.Funcs{
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				var fixed topolvmv1.LogicalVolume
				if err := c.Get(ctx, client.ObjectKeyFromObject(obj), &fixed); err != nil {
					return err
				}
				fixed.Status = *sized.DeepCopy()
				if err := c.Status().Update(ctx, &fixed); err != nil {
					return err
				}
				return c.Delete(ctx, obj, opts...)
			},
		})

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		lv, err := s.CreateVolume(ctx, "node", "dc", "oc", "lv", "source", requestedBytes)
		if err != nil {
			t.Fatalf("expected no error, but got %v", err)
		}
		if got := lv.Status.CurrentSize.Value(); got != reportedBytes {
			t.Errorf("expected the reported size %d, but got %d", int64(reportedBytes), got)
		}
		if got := deletionTimestamp(t, c); got != nil {
			t.Errorf("expected the LogicalVolume not to be deleted, but got %v", got)
		}
	})

	t.Run("reports the node's failure when the size stays below spec.size", func(t *testing.T) {
		s, c := newTestService(t, undersizedAndFailed)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := s.CreateVolume(ctx, "node", "dc", "oc", "lv", "source", requestedBytes)
		if got := status.Code(err); got != codes.Internal {
			t.Fatalf("expected %s, but got %s (err: %v)", codes.Internal, got, err)
		}
		if got := deletionTimestamp(t, c); got == nil {
			t.Errorf("expected the LogicalVolume to be deleted, but got %v", got)
		}
	})
}
