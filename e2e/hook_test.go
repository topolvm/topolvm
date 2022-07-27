package e2e

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
)

const nsHookTest = "hook-test"

//go:embed testdata/hook/pod-with-pvc.yaml
var podWithPVCYAML []byte

//go:embed testdata/hook/generic-ephemeral-volume.yaml
var hookGenericEphemeralVolumeYAML []byte

func hasTopoLVMFinalizer(pvc *corev1.PersistentVolumeClaim) bool {
	for _, fin := range pvc.Finalizers {
		if fin == topolvm.PVCFinalizer {
			return true
		}
	}
	return false
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
			stdout, stderr, err := kubectlWithInput(podWithPVCYAML, "-n", nsHookTest, "apply", "-f", "-", "--dry-run=server")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, stdout, stderr)
			}
			return nil
		}).Should(Succeed())

		stdout, stderr, err := kubectlWithInput(podWithPVCYAML, "-n", nsHookTest, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("checking pod is properly annotated")
		Eventually(func() error {
			if isStorageCapacity() {
				return nil
			}

			result, stderr, err := kubectl("get", "-n", nsHookTest, "pods/testhttpd", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var pod corev1.Pod
			err = json.Unmarshal(result, &pod)
			if err != nil {
				return err
			}

			resources := pod.Spec.Containers[0].Resources
			_, ok := resources.Limits[topolvm.CapacityResource]
			if !ok {
				return errors.New("resources.Limits is not mutated")
			}
			_, ok = resources.Requests[topolvm.CapacityResource]
			if !ok {
				return errors.New("resources.Requests is not mutated")
			}

			capacity, ok := pod.Annotations[topolvm.CapacityKeyPrefix+"ssd"]
			if !ok {
				return errors.New("not annotated")
			}
			if capacity != strconv.Itoa(3<<30) {
				return fmt.Errorf("wrong capacity: actual=%s, expect=%d", capacity, 3<<30)
			}

			return nil
		}).Should(Succeed())

		By("checking pvc has TopoLVM finalizer")
		result, stderr, err := kubectl("get", "-n", nsHookTest, "pvc", "-o=json")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", result, stderr)

		var pvcList corev1.PersistentVolumeClaimList
		err = json.Unmarshal(result, &pvcList)
		Expect(err).ShouldNot(HaveOccurred())

		for _, pvc := range pvcList.Items {
			hasFinalizer := hasTopoLVMFinalizer(&pvc)
			Expect(hasFinalizer).Should(Equal(true), "finalizer is not set: pvc=%s", pvc.Name)
		}
	})

	It("should test hooks for generic ephemeral volumes", func() {
		if isStorageCapacity() {
			Skip(skipMessageForStorageCapacity)
			return
		}

		By("creating pod with TopoLVM generic ephemeral volumes")
		stdout, stderr, err := kubectlWithInput(hookGenericEphemeralVolumeYAML, "-n", nsHookTest, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		By("checking pod is annotated with topolvm.cybozu.com/capacity")
		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n", nsHookTest, "pods/testhttpd", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var pod corev1.Pod
			err = json.Unmarshal(result, &pod)
			if err != nil {
				return err
			}

			resources := pod.Spec.Containers[0].Resources
			_, ok := resources.Limits[topolvm.CapacityResource]
			if !ok {
				return errors.New("resources.Limits is not mutated")
			}
			_, ok = resources.Requests[topolvm.CapacityResource]
			if !ok {
				return errors.New("resources.Requests is not mutated")
			}

			capacity, ok := pod.Annotations[topolvm.CapacityKeyPrefix+"ssd"]
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
