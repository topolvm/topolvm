package controllers

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("PersistentVolumeClaimController controller", func() {
	ctx := context.Background()
	var stopFunc func()
	errCh := make(chan error)

	BeforeEach(func() {
		err := k8sClient.DeleteAllOf(ctx, &corev1.PersistentVolumeClaim{}, client.InNamespace("test"))
		Expect(err).NotTo(HaveOccurred())

		time.Sleep(100 * time.Millisecond)

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: scheme,
		})
		Expect(err).ToNot(HaveOccurred())

		reconciler := PersistentVolumeClaimReconciler{
			Client: k8sClient,
		}
		err = reconciler.SetupWithManager(mgr)
		Expect(err).NotTo(HaveOccurred())

		ctx, cancel := context.WithCancel(ctx)
		stopFunc = cancel
		go func() {
			errCh <- mgr.Start(ctx)
		}()
		time.Sleep(100 * time.Millisecond)
	})

	AfterEach(func() {
		stopFunc()
		Expect(<-errCh).NotTo(HaveOccurred())
	})

	It("should delete PVC finalizer", func() {
		pvc := newPVCWithFinalizer()
		err := k8sClient.Create(ctx, pvc)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() error {
			gotPVC := corev1.PersistentVolumeClaim{}
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "sample"}, &gotPVC)
			if err != nil {
				return err
			}
			// "kubernetes.io/pvc-protection" is automatically added.
			expectedFinalizers := []string{"topolvm.io/finalizer1", "topolvm.io/finalizer2", "kubernetes.io/pvc-protection"}
			for i, f := range gotPVC.Finalizers {
				if f != expectedFinalizers[i] {
					return fmt.Errorf("unexpected finalizer. expected = %s, actual = %s", expectedFinalizers[i], f)
				}
			}
			return nil
		}).Should(Succeed())
	})
})

func newPVCWithFinalizer() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "sample",
			Namespace:  "test",
			Finalizers: []string{topolvm.GetPVCFinalizer(), "topolvm.io/finalizer1", "topolvm.io/finalizer2"},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("3Mi"),
				},
			},
		},
	}
}
