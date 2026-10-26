package controller

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	corev1 "k8s.io/api/core/v1"
	storegev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var volumes = &[]*proto.LogicalVolume{}

type MockVGServiceClient struct {
}

// GetFreeBytes implements proto.VGServiceClient.
func (MockVGServiceClient) GetFreeBytes(ctx context.Context, in *proto.GetFreeBytesRequest, opts ...grpc.CallOption) (*proto.GetFreeBytesResponse, error) {
	panic("unimplemented")
}

// GetLVList implements proto.VGServiceClient.
func (c MockVGServiceClient) GetLVList(ctx context.Context, in *proto.GetLVListRequest, opts ...grpc.CallOption) (*proto.GetLVListResponse, error) {
	return &proto.GetLVListResponse{
		Volumes: *volumes,
	}, nil
}

// Watch implements proto.VGServiceClient.
func (MockVGServiceClient) Watch(ctx context.Context, in *proto.Empty, opts ...grpc.CallOption) (proto.VGService_WatchClient, error) {
	panic("unimplemented")
}

type MockLVServiceClient struct {
}

// CreateLV implements proto.LVServiceClient.
func (c MockLVServiceClient) CreateLV(ctx context.Context, in *proto.CreateLVRequest, opts ...grpc.CallOption) (*proto.CreateLVResponse, error) {
	lv := proto.LogicalVolume{
		Name:      in.Name,
		SizeBytes: in.SizeBytes,
	}
	*volumes = append(*volumes, &lv)
	createResponse := proto.CreateLVResponse{
		Volume: &lv,
	}
	return &createResponse, nil
}

// CreateLVSnapshot implements proto.LVServiceClient.
func (MockLVServiceClient) CreateLVSnapshot(ctx context.Context, in *proto.CreateLVSnapshotRequest, opts ...grpc.CallOption) (*proto.CreateLVSnapshotResponse, error) {
	panic("unimplemented")
}

// RemoveLV implements proto.LVServiceClient.
func (MockLVServiceClient) RemoveLV(ctx context.Context, in *proto.RemoveLVRequest, opts ...grpc.CallOption) (*proto.Empty, error) {
	panic("unimplemented")
}

// ResizeLV implements proto.LVServiceClient.
func (MockLVServiceClient) ResizeLV(ctx context.Context, in *proto.ResizeLVRequest, opts ...grpc.CallOption) (*proto.ResizeLVResponse, error) {
	panic("unimplemented")
}

// blockingLVServiceClient never answers CreateLV until the context ends,
// like an lvcreate stuck waiting for a udev cookie. It returns a plain error,
// as the embedded lvmd does, rather than a gRPC status.
type blockingLVServiceClient struct {
	MockLVServiceClient
}

// CreateLV implements proto.LVServiceClient.
func (blockingLVServiceClient) CreateLV(ctx context.Context, in *proto.CreateLVRequest, opts ...grpc.CallOption) (*proto.CreateLVResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// blockingVGServiceClient never answers GetLVList until the context ends,
// like an lvs waiting for a VG lock that a stuck LVM command holds.
type blockingVGServiceClient struct {
	MockVGServiceClient
}

// GetLVList implements proto.VGServiceClient.
func (blockingVGServiceClient) GetLVList(ctx context.Context, in *proto.GetLVListRequest, opts ...grpc.CallOption) (*proto.GetLVListResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

var _ = Describe("LogicalVolume controller", func() {
	ctx := context.Background()
	var stopFunc func()
	errCh := make(chan error)
	startReconcilerWith := func(suffix string, vgService proto.VGServiceClient, lvService proto.LVServiceClient,
		opts ...LogicalVolumeReconcilerOption) {
		skipNameValidation := true
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: scheme,
			Controller: config.Controller{
				SkipNameValidation: &skipNameValidation,
			},
			Metrics: server.Options{
				BindAddress: "0", // disable metrics
			},
		})
		Expect(err).ToNot(HaveOccurred())

		reconciler := NewLogicalVolumeReconcilerWithServices(mgr.GetClient(), nodeNameBase+suffix, vgService, lvService, opts...)
		err = reconciler.SetupWithManager(mgr)
		Expect(err).NotTo(HaveOccurred())

		ctx, cancel := context.WithCancel(ctx)
		stopFunc = cancel
		go func() {
			errCh <- mgr.Start(ctx)
		}()
		time.Sleep(100 * time.Millisecond)
	}

	startReconciler := func(suffix string) {
		startReconcilerWith(suffix, MockVGServiceClient{}, MockLVServiceClient{})
	}

	AfterEach(func() {
		stopFunc()
		Expect(<-errCh).NotTo(HaveOccurred())
	})

	setupResources := func(ctx context.Context, suffix string) topolvmv1.LogicalVolume {
		node := corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeNameBase + suffix,
				Finalizers: []string{
					topolvm.GetNodeFinalizer(),
				},
			},
		}
		err := k8sClient.Create(ctx, &node)
		Expect(err).NotTo(HaveOccurred())

		sc := storegev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: storageClassNameBase + suffix,
			},
			Provisioner: topolvm.GetPluginName(),
		}
		err = k8sClient.Create(ctx, &sc)
		Expect(err).NotTo(HaveOccurred())

		ns := createNamespace()
		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcNameBase + suffix,
				Namespace: ns,
				Annotations: map[string]string{
					AnnSelectedNode: node.Name,
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: &sc.Name,
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: *resource.NewQuantity(1, resource.BinarySI),
					},
				},
			},
		}
		err = k8sClient.Create(ctx, &pvc)
		Expect(err).NotTo(HaveOccurred())

		lv := topolvmv1.LogicalVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: logicalVolumeNameBase + suffix,
			},
			Spec: topolvmv1.LogicalVolumeSpec{
				NodeName: node.Name,
			},
		}
		err = k8sClient.Create(ctx, &lv)
		Expect(err).NotTo(HaveOccurred())

		return lv
	}

	It("should add finalizer to LogicalVolume", func() {
		startReconciler("-add-finalizer")

		ctx := context.Background()

		// Setup
		lv := setupResources(ctx, "-add-finalizer")

		// Verify
		// ensure LV has finalizer
		Eventually(func(g Gomega) bool {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&lv), &lv)
			if err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(&lv, topolvm.GetLogicalVolumeFinalizer())
		}).Should(BeTrue())
	})

	It("should not add finalizer to LogicalVolume when volume has pendingdeletion annotation", func() {
		startReconciler("-pendingdeletion")

		ctx := context.Background()

		// Setup
		lv := setupResources(ctx, "-pendingdeletion")

		// ensure LV gets finalizer
		Eventually(func(g Gomega) bool {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&lv), &lv)
			if err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(&lv, topolvm.GetLogicalVolumeFinalizer())
		}).Should(BeTrue())

		// Add pending deletion key & remove finalizer
		lv2 := lv.DeepCopy()
		lv2.Annotations = map[string]string{
			topolvm.GetLVPendingDeletionKey(): "true",
		}
		controllerutil.RemoveFinalizer(lv2, topolvm.GetLogicalVolumeFinalizer())

		patch := client.MergeFrom(&lv)
		err := k8sClient.Patch(ctx, lv2, patch)
		Expect(err).NotTo(HaveOccurred())

		// ensure LV finalizer is removed
		Consistently(func(g Gomega) bool {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&lv), &lv)
			if err != nil {
				return false
			}
			return !controllerutil.ContainsFinalizer(&lv, topolvm.GetLogicalVolumeFinalizer())
		}, "2s").Should(BeTrue())
	})

	It("should fail the LogicalVolume with DeadlineExceeded when CreateLV does not finish in time", func() {
		startReconcilerWith("-lvm-timeout", MockVGServiceClient{}, blockingLVServiceClient{},
			WithLVMOperationTimeout(time.Second))

		ctx := context.Background()

		// Setup
		lv := setupResources(ctx, "-lvm-timeout")

		// Verify
		// the stuck CreateLV is given up on, and the failure is recorded on the LogicalVolume
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&lv), &lv)).To(Succeed())
			g.Expect(lv.Status.Code).To(Equal(codes.DeadlineExceeded))
			g.Expect(lv.Status.Message).To(ContainSubstring("did not finish within 1s"))
			g.Expect(lv.Status.VolumeID).To(BeEmpty())
		}).Should(Succeed())
	})

	It("should fail the LogicalVolume with DeadlineExceeded when listing LVs does not finish in time", func() {
		startReconcilerWith("-lvs-timeout", blockingVGServiceClient{}, MockLVServiceClient{},
			WithLVMOperationTimeout(time.Second))

		ctx := context.Background()

		// Setup
		lv := setupResources(ctx, "-lvs-timeout")

		// Verify
		// the stuck existence check is given up on, and the timeout is named on the LogicalVolume
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&lv), &lv)).To(Succeed())
			g.Expect(lv.Status.Code).To(Equal(codes.DeadlineExceeded))
			g.Expect(lv.Status.Message).To(ContainSubstring("did not finish within 1s"))
			g.Expect(lv.Status.VolumeID).To(BeEmpty())
		}).Should(Succeed())
	})
})

func TestLVMContext(t *testing.T) {
	t.Run("the default timeout sets a deadline", func(t *testing.T) {
		r := NewLogicalVolumeReconcilerWithServices(nil, "node", nil, nil)
		ctx, cancel := r.lvmContext(context.Background())
		defer cancel()
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("expected a deadline")
		}
		if remaining := time.Until(deadline); remaining <= 0 || remaining > DefaultLVMOperationTimeout {
			t.Fatalf("deadline is %v away, expected at most %v", remaining, DefaultLVMOperationTimeout)
		}
	})

	t.Run("a zero timeout disables the deadline", func(t *testing.T) {
		r := NewLogicalVolumeReconcilerWithServices(nil, "node", nil, nil, WithLVMOperationTimeout(0))
		ctx, cancel := r.lvmContext(context.Background())
		defer cancel()
		if _, ok := ctx.Deadline(); ok {
			t.Fatal("expected no deadline when the timeout is disabled")
		}
	})
}
