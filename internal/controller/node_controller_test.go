package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	corev1 "k8s.io/api/core/v1"
	storegev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("NodeController controller", func() {
	ctx := context.Background()
	var stopFunc func()
	errCh := make(chan error)

	startReconciler := func(skipNodeFinalize bool) {
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: scheme,
		})
		Expect(err).ToNot(HaveOccurred())

		reconciler := NewNodeReconciler(mgr.GetClient(), skipNodeFinalize)
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

	setupResources := func(ctx context.Context, suffix string) (
		corev1.Node, corev1.PersistentVolumeClaim, topolvmv1.LogicalVolume) {
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

		return node, pvc, lv
	}

	It("should delete PVC and LogicalVolume when the node is deleted if the finalizer is not skipped", func() {
		startReconciler(false)

		ctx := context.Background()

		// Setup
		node, pvc, lv := setupResources(ctx, "-do-finalizer")

		// Exercise
		err := k8sClient.Delete(ctx, &node)
		Expect(err).NotTo(HaveOccurred())

		// Verify
		Eventually(func(g Gomega) error {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&node), &node)
			if !apierrors.IsNotFound(err) {
				return errors.New("Node is not deleted")
			}

			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &pvc)
			if err == nil {
				// k8s may add the pvc-protection finalizer, so check DeletionTimestamp.
				if pvc.DeletionTimestamp == nil {
					return fmt.Errorf("delete API is not called")
				}
			} else if !apierrors.IsNotFound(err) {
				return fmt.Errorf("PVC is not deleted: %w", err)
			}

			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&lv), &lv)
			if !apierrors.IsNotFound(err) {
				return errors.New("LV is not deleted")
			}

			return nil
		}).Should(Succeed())
	})

	It("should not touch PVC and LogicalVolume when the node is deleted if the finalizer is skipped", func() {
		startReconciler(true)

		ctx := context.Background()

		// Setup
		node, pvc, lv := setupResources(ctx, "-skip-finalizer")

		// Exercise
		err := k8sClient.Delete(ctx, &node)
		Expect(err).NotTo(HaveOccurred())

		// Verify
		// wait for node delete
		Eventually(func(g Gomega) error {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&node), &node)
			if !apierrors.IsNotFound(err) {
				return errors.New("Node is not deleted")
			}
			return nil
		}).Should(Succeed())

		err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &pvc)
		Expect(err).NotTo(HaveOccurred())
		Expect(pvc.DeletionTimestamp).To(BeNil())

		err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&lv), &lv)
		Expect(err).NotTo(HaveOccurred())
		Expect(lv.DeletionTimestamp).To(BeNil())
	})
})
