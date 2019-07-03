package kindtest

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cybozu-go/topolvm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

const nsHookTest = "hook-test"

var _ = FContext("in hook-test namespace", func() {
	BeforeEach(func() {
		createNamespace(nsHookTest)
	})
	AfterEach(func() {
		kubectl("delete", "namespaces/"+nsHookTest)
	})

	Describe("Test topolvm-hook", testTopoLVMHook)
})

func testTopoLVMHook() {
	It("should have deployed topolvm-hook pod", func() {
		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n=topolvm-system", "pods", "--selector=app.kubernetes.io/name=topolvm-hook", "-o=json")
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

			return errors.New("topolvm-hook is not yet ready")
		}).Should(Succeed())
	})

	It("should annotate pod with topolvm.cybozu.com/capacity", func() {
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
      storage: 1Gi
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
    - name: testhttpd
      image: quay.io/cybozu/testhttpd:0
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
			v, ok := resources.Limits[topolvm.CapacityResource]
			if !ok {
				return errors.New("resources.Limits is not mutated")
			}
			if v.Value() != 2<<30 {
				return fmt.Errorf("wrong limit value: actual=%d, expect=%d", v.Value(), 2<<30)
			}

			v, ok = resources.Requests[topolvm.CapacityResource]
			if !ok {
				return errors.New("resources.Requests is not mutated")
			}
			if v.Value() != 2<<30 {
				return fmt.Errorf("wrong request value: actual=%d, expect=%d", v.Value(), 2<<30)
			}

			return nil
		}).Should(Succeed())
	})

	It("should not annotate pod with topolvm.cybozu.com/capacity", func() {
		yml := `
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: local-pvc3
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: host-local
---
apiVersion: v1
kind: Pod
metadata:
  name: testhttpd2
  labels:
    app.kubernetes.io/name: testhttpd2
spec:
  containers:
    - name: testhttpd
      image: quay.io/cybozu/testhttpd:0
      volumeMounts:
        - mountPath: /test1
          name: my-volume1
  volumes:
    - name: my-volume1
      persistentVolumeClaim:
        claimName: local-pvc3
`
		stdout, stderr, err := kubectlWithInput([]byte(yml), "-n", nsHookTest, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n", nsHookTest, "pods/testhttpd2", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var pod corev1.Pod
			err = json.Unmarshal(result, &pod)
			if err != nil {
				return err
			}

			resources := pod.Spec.Containers[0].Resources
			v, ok := resources.Limits[topolvm.CapacityResource]
			if ok {
				return fmt.Errorf("resources.Limits is mutated: value=%d", v.Value())
			}

			v, ok = resources.Requests[topolvm.CapacityResource]
			if ok {
				return fmt.Errorf("resources.Requests is mutated: value=%d", v.Value())
			}

			return nil
		}).Should(Succeed())
	})

	It("should not add resource for bound PVC", func() {
		yml := `
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: bound-pvc
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: topolvm-provisioner-immediate
`
		_, stderr, err := kubectlWithInput([]byte(yml), "-n", nsHookTest, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", string(stderr))

		Eventually(func() error {
			stdout, _, err := kubectl("-n", nsHookTest, "get", "pvc/bound-pvc", "-o", "json")
			if err != nil {
				return err
			}

			var pvc corev1.PersistentVolumeClaim
			err = json.Unmarshal(stdout, &pvc)
			if err != nil {
				return err
			}

			if len(pvc.Spec.VolumeName) == 0 {
				return errors.New("pvc is not bound")
			}
			fmt.Println("pvc bound with pv " + pvc.Spec.VolumeName)
			return nil
		}).Should(Succeed())

		yml = `
apiVersion: v1
kind: Pod
metadata:
  name: testhttpd3
  labels:
    app.kubernetes.io/name: testhttpd3
spec:
  containers:
    - name: testhttpd
      image: quay.io/cybozu/testhttpd:0
      volumeMounts:
        - mountPath: /test1
          name: my-volume1
  volumes:
    - name: my-volume1
      persistentVolumeClaim:
        claimName: bound-pvc
`
		stdout, stderr, err := kubectlWithInput([]byte(yml), "-n", nsHookTest, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		var pod *corev1.Pod
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "-n", nsHookTest, "pods/testhttpd3", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stderr=%s", err, stderr)
			}

			pod = new(corev1.Pod)
			err = json.Unmarshal(stdout, pod)
			if err != nil {
				return err
			}

			if len(pod.Spec.NodeName) == 0 {
				return errors.New("pod is not scheduled")
			}
			return nil
		}).Should(Succeed())

		resources := pod.Spec.Containers[0].Resources
		Expect(resources.Limits).ShouldNot(HaveKey(topolvm.CapacityResource))
		Expect(resources.Requests).ShouldNot(HaveKey(topolvm.CapacityResource))
	})
}
