package e2e

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const nsHookTest = "hook-test"

//go:embed testdata/hook/pod-with-pvc.yaml
var podWithPVCYAML []byte

//go:embed testdata/hook/generic-ephemeral-volume.yaml
var hookGenericEphemeralVolumeYAML []byte

func hasTopoLVMFinalizer(pvc *corev1.PersistentVolumeClaim) bool {
	return controllerutil.ContainsFinalizer(pvc, topolvm.PVCFinalizer)
}

func testHook() {
	var cc CleanupContext
	BeforeEach(func() {
		createNamespace(nsHookTest)
		cc = commonBeforeEach()
	})
	AfterEach(func() {
		kubectl("delete", "namespaces/"+nsHookTest)
		commonAfterEach(cc)
	})

	It("should test hooks", func() {
		By("creating pod with TopoLVM PVC")
		Eventually(func() error {
			_, _, err := kubectlWithInput(podWithPVCYAML, "-n", nsHookTest, "apply", "-f", "-", "--dry-run=server")
			if err != nil {
				return err
			}
			return nil
		}).Should(Succeed())

		_, _, err := kubectlWithInput(podWithPVCYAML, "-n", nsHookTest, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("checking pod is properly annotated")
		Eventually(func() error {
			if isStorageCapacity() {
				return nil
			}

			result, _, err := kubectl("get", "-n", nsHookTest, "pods/pause", "-o=json")
			if err != nil {
				return err
			}

			var pod corev1.Pod
			err = json.Unmarshal(result, &pod)
			if err != nil {
				return err
			}

			resources := pod.Spec.Containers[0].Resources
			_, ok := resources.Limits[topolvm.GetCapacityResource()]
			if !ok {
				return errors.New("resources.Limits is not mutated")
			}
			_, ok = resources.Requests[topolvm.GetCapacityResource()]
			if !ok {
				return errors.New("resources.Requests is not mutated")
			}

			capacity, ok := pod.Annotations[topolvm.GetCapacityKeyPrefix()+"ssd"]
			if !ok {
				return errors.New("not annotated")
			}
			if capacity != strconv.Itoa(3<<30) {
				return fmt.Errorf("wrong capacity: actual=%s, expect=%d", capacity, 3<<30)
			}

			return nil
		}).Should(Succeed())

		By("checking pvc has TopoLVM finalizer")
		result, _, err := kubectl("get", "-n", nsHookTest, "pvc", "-o=json")
		Expect(err).ShouldNot(HaveOccurred())

		var pvcList corev1.PersistentVolumeClaimList
		err = json.Unmarshal(result, &pvcList)
		Expect(err).ShouldNot(HaveOccurred())

		for _, pvc := range pvcList.Items {
			hasFinalizer := hasTopoLVMFinalizer(&pvc)
			Expect(hasFinalizer).Should(Equal(true), "finalizer is not set: pvc=%s finalizers=%v", pvc.Name, pvc.ObjectMeta.Finalizers)
		}
	})

	It("should test hooks for generic ephemeral volumes", func() {
		if isStorageCapacity() {
			Skip(skipMessageForStorageCapacity)
			return
		}

		By("creating pod with TopoLVM generic ephemeral volumes")
		_, _, err := kubectlWithInput(hookGenericEphemeralVolumeYAML, "-n", nsHookTest, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("checking pod is annotated with" + topolvm.GetCapacityResource().String())
		Eventually(func() error {
			result, _, err := kubectl("get", "-n", nsHookTest, "pods/pause", "-o=json")
			if err != nil {
				return err
			}

			var pod corev1.Pod
			err = json.Unmarshal(result, &pod)
			if err != nil {
				return err
			}

			resources := pod.Spec.Containers[0].Resources
			_, ok := resources.Limits[topolvm.GetCapacityResource()]
			if !ok {
				return errors.New("resources.Limits is not mutated")
			}
			_, ok = resources.Requests[topolvm.GetCapacityResource()]
			if !ok {
				return errors.New("resources.Requests is not mutated")
			}

			capacity, ok := pod.Annotations[topolvm.GetCapacityKeyPrefix()+"ssd"]
			if !ok {
				return errors.New("not annotated")
			}
			if capacity != strconv.Itoa(1<<30) {
				return fmt.Errorf("wrong capacity: actual=%s, expect=%d", capacity, 1<<30)
			}

			return nil
		}).Should(Succeed())
	})
}
