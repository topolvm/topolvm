package e2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cybozu-go/topolvm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

const nsHookTest = "hook-test"

func hasTopoLVMFinalizer(pvc *corev1.PersistentVolumeClaim) bool {
	for _, fin := range pvc.Finalizers {
		if fin == topolvm.PVCFinalizer {
			return true
		}
	}
	return false
}

func testHook() {
	BeforeEach(func() {
		createNamespace(nsHookTest)
	})
	AfterEach(func() {
		kubectl("delete", "namespaces/"+nsHookTest)
	})

	It("should test hooks", func() {
		By("waiting controller pod become ready")
		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n=topolvm-system", "pods", "--selector=app.kubernetes.io/name=controller", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var podlist corev1.PodList
			err = json.Unmarshal(result, &podlist)
			if err != nil {
				return err
			}

			if len(podlist.Items) != 1 {
				return errors.New("pod is not found")
			}

			pod := podlist.Items[0]
			for _, cond := range pod.Status.Conditions {
				fmt.Println(cond)
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return nil
				}
			}

			return errors.New("controller is not yet ready")
		}).Should(Succeed())

		By("creating pod with TopoLVM PVC")
		yml := `
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: local-pvc1
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 2Gi
  storageClassName: topolvm-provisioner
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: local-pvc2
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: topolvm-provisioner
---
apiVersion: v1
kind: Pod
metadata:
  name: testhttpd
  labels:
    app.kubernetes.io/name: testhttpd
spec:
  containers:
    - name: ubuntu
      image: quay.io/cybozu/ubuntu:18.04
      command: ["/usr/local/bin/pause"]
      volumeMounts:
        - mountPath: /test1
          name: my-volume1
        - mountPath: /test2
          name: my-volume2
  volumes:
    - name: my-volume1
      persistentVolumeClaim:
        claimName: local-pvc1
    - name: my-volume2
      persistentVolumeClaim:
        claimName: local-pvc2
`
		stdout, stderr, err := kubectlWithInput([]byte(yml), "-n", nsHookTest, "apply", "-f", "-")
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

			capacity, ok := pod.Annotations[topolvm.CapacityKey+"-myvg1"]
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
	It("should test hooks for inline ephemeral volumes", func() {
		const minInlineEphemeralVer int64 = 16
		kubernetesVersionStr := os.Getenv("TEST_KUBERNETES_VERSION")
		kubernetesVersion := strings.Split(kubernetesVersionStr, ".")
		Expect(len(kubernetesVersion)).To(Equal(2))
		kubernetesMinorVersion, err := strconv.ParseInt(kubernetesVersion[1], 10, 64)
		Expect(err).ShouldNot(HaveOccurred())

		if kubernetesMinorVersion < minInlineEphemeralVer {
			Skip(fmt.Sprintf(
				"inline ephemeral volumes not supported on Kubernetes version: %s. Min supported version is 1.%d",
				kubernetesVersionStr,
				minInlineEphemeralVer,
			))
		}
		By("waiting controller pod become ready")
		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n=topolvm-system", "pods", "--selector=app.kubernetes.io/name=controller", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var podlist corev1.PodList
			err = json.Unmarshal(result, &podlist)
			if err != nil {
				return err
			}

			if len(podlist.Items) != 1 {
				return errors.New("pod is not found")
			}

			pod := podlist.Items[0]
			for _, cond := range pod.Status.Conditions {
				fmt.Println(cond)
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return nil
				}
			}

			return errors.New("controller is not yet ready")
		}).Should(Succeed())

		By("creating pod with TopoLVM inline ephemeral volumes and a TopoLVM PVC")
		yml := `
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: local-pvc1
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 2Gi
  storageClassName: topolvm-provisioner
---
apiVersion: v1
kind: Pod
metadata:
  name: testhttpd
  labels:
    app.kubernetes.io/name: testhttpd
spec:
  containers:
    - name: ubuntu
      image: quay.io/cybozu/ubuntu:18.04
      command: ["/usr/local/bin/pause"]
      volumeMounts:
        - mountPath: /test1
          name: my-ephemeral-volume1
  volumes:
      - name: my-ephemeral-volume1
        csi:
          driver: topolvm.cybozu.com
          fsType: xfs
          volumeAttributes:
            topolvm.cybozu.com/size: "2"
      - name: my-ephemeral-volume2
        csi:
          driver: topolvm.cybozu.com
          fsType: xfs
          volumeAttributes:
            topolvm.cybozu.com/size: "1"
      - name: my-pvc-volume1
        persistentVolumeClaim:
          claimName: local-pvc1
`
		stdout, stderr, err := kubectlWithInput([]byte(yml), "-n", nsHookTest, "apply", "-f", "-")
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

			capacity, ok := pod.Annotations[topolvm.CapacityKey+"-myvg1"]
			if !ok {
				return errors.New("not annotated")
			}
			if capacity != strconv.Itoa(5<<30) {
				return fmt.Errorf("wrong capacity: actual=%s, expect=%d", capacity, 5<<30)
			}

			return nil
		}).Should(Succeed())

	})
}
