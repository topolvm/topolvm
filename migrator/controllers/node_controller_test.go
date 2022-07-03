package controllers

import (
	"fmt"
	"sort"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/topolvm/topolvm"
)

func testNode() *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Finalizers: []string{
				topolvm.LegacyNodeFinalizer,
				"aaa/bbb",
			},
			Annotations: map[string]string{
				topolvm.LegacyCapacityKeyPrefix + topolvm.DefaultDeviceClassAnnotationName: fmt.Sprintf("%d", 1<<30),
				"aaa": "bbb",
			},
		},
	}
}

var _ = Describe("test node controller", func() {
	It("should migrate pvc finalizer and capacity annotation", func() {
		By("create a node")
		node := testNode()
		err := k8sClient.Create(testCtx, node)
		Expect(err).ShouldNot(HaveOccurred())
		defer func() {
			err = k8sClient.Delete(testCtx, node)
			Expect(err).ShouldNot(HaveOccurred())
		}()

		By("check a node data")
		expetedFinalizers := []string{
			// Unrelated finalizers does not remove
			"aaa/bbb",
			// Replaces the legacy TopoLVM finalizer
			topolvm.NodeFinalizer,
		}
		sort.Strings(expetedFinalizers)
		expectedAnnotations := map[string]string{
			// Unrelated annotations does not remove
			"aaa": "bbb",
			// Replaces the legacy TopoLVM annotations
			topolvm.CapacityKeyPrefix + topolvm.DefaultDeviceClassAnnotationName: fmt.Sprintf("%d", 1<<30),
		}
		name := types.NamespacedName{
			Name: node.Name,
		}
		Eventually(func() error {
			n := &corev1.Node{}
			err := k8sClient.Get(testCtx, name, n)
			if err != nil {
				return fmt.Errorf("can not get target node: err=%v", err)
			}
			sort.Strings(n.Finalizers)
			if diff := cmp.Diff(expetedFinalizers, n.Finalizers); diff != "" {
				return fmt.Errorf("node finalizers does not match: (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(expectedAnnotations, n.Annotations); diff != "" {
				return fmt.Errorf("node annotations does not match: (-want,+got):\n%s", diff)
			}
			return nil
		}).Should(Succeed())
	})
})
