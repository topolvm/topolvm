package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/internal/lvmd/proto"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	storegev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
		Name: in.Name,
		//lint:ignore SA1019 gRPC API has two fields for Gb and Bytes, both are valid
		SizeGb:    in.SizeGb,
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
func (MockLVServiceClient) ResizeLV(ctx context.Context, in *proto.ResizeLVRequest, opts ...grpc.CallOption) (*proto.Empty, error) {
	panic("unimplemented")
}

var _ = Describe("LogicalVolume controller", func() {
	ctx := context.Background()
	var stopFunc func()
	errCh := make(chan error)
	var vgService MockVGServiceClient
	var lvService MockLVServiceClient

	startReconciler := func(suffix string) {
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: scheme,
		})
		Expect(err).ToNot(HaveOccurred())

		vgService = MockVGServiceClient{}
		lvService = MockLVServiceClient{}

		reconciler := NewLogicalVolumeReconcilerWithServices(mgr.GetClient(), "node"+suffix, vgService, lvService)
		err = reconciler.SetupWithManager(mgr)
		Expect(err).NotTo(HaveOccurred())

		ctx, cancel := context.WithCancel(ctx)
		stopFunc = cancel
		go func() {
			errCh <- mgr.Start(ctx)
		}()
		time.Sleep(100 * time.Millisecond)
	}

	AfterEach(func() {
		stopFunc()
		Expect(<-errCh).NotTo(HaveOccurred())
	})

	setupResources := func(ctx context.Context, suffix string) topolvmv1.LogicalVolume {
		node := corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node" + suffix,
				Finalizers: []string{
					topolvm.GetNodeFinalizer(),
				},
			},
		}
		err := k8sClient.Create(ctx, &node)
		Expect(err).NotTo(HaveOccurred())

		sc := storegev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "sc" + suffix,
			},
			Provisioner: topolvm.GetPluginName(),
		}
		err = k8sClient.Create(ctx, &sc)
		Expect(err).NotTo(HaveOccurred())

		ns := createNamespace()
		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pvc" + suffix,
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
				Resources: corev1.ResourceRequirements{
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
				Name: "lv" + suffix,
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
})
