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

var _ = Describe("Test topolvm-hook", func() {
	testNamespacePrefix := "hook-test"

	It("should be deployed topolvm-hook pod", func() {
		ns := testNamespacePrefix + randomString(10)
		createNamespace(ns)

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
		ns := testNamespacePrefix + randomString(10)
		createNamespace(ns)

		yml := fmt.Sprintf(`kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: local-pvc1
  namespace: %s 
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
  namespace: %s
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
  namespace: %s
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
`, ns, ns, ns)
		stdout, stderr, err := kubectlWithInput([]byte(yml), "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n", ns, "pods/testhttpd", "-o=json")
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

	It("should replace pod annotation of topolvm.cybozu.com/capacity", func() {
		ns := testNamespacePrefix + randomString(10)
		createNamespace(ns)

		yml := fmt.Sprintf(`kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: local-pvc1
  namespace: %s
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
  namespace: %s
  labels:
    app.kubernetes.io/name: testhttpd
spec:
  containers:
    - name: testhttpd
      image: quay.io/cybozu/testhttpd:0
      volumeMounts:
        - mountPath: /test1
          name: my-volume1
      resources:
        limits:
          topolvm.cybozu.com/capacity: "1024"
        requests:
          topolvm.cybozu.com/capacity: "1024"
  volumes:
    - name: my-volume1
      persistentVolumeClaim:
        claimName: local-pvc1
`, ns, ns)
		stdout, stderr, err := kubectlWithInput([]byte(yml), "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n", ns, "pods/testhttpd", "-o=json")
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
				return errors.New("resources.Limits is deleted")
			}
			if v.Value() != 1<<30 {
				return fmt.Errorf("wrong limit value: actual=%d, expect=%d", v.Value(), 1<<30)
			}

			v, ok = resources.Requests[topolvm.CapacityResource]
			if !ok {
				return errors.New("resources.Requests is deleted")
			}
			if v.Value() != 1<<30 {
				return fmt.Errorf("wrong request value: actual=%d, expect=%d", v.Value(), 1<<30)
			}

			return nil
		}).Should(Succeed())
	})

	It("should annotate pod with topolvm.cybozu.com/capacity", func() {
		ns := testNamespacePrefix + randomString(10)
		createNamespace(ns)

		yml := fmt.Sprintf(`kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: local-pvc1
  namespace: %s
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
  name: testhttpd
  namespace: %s
  labels:
    app.kubernetes.io/name: testhttpd
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
        claimName: local-pvc1`, ns, ns)
		stdout, stderr, err := kubectlWithInput([]byte(yml), "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n", ns, "pods/testhttpd", "-o=json")
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

	It("should replace pod annotation of topolvm.cybozu.com/capacity", func() {
		ns := testNamespacePrefix + randomString(10)
		createNamespace(ns)

		yml := fmt.Sprintf(`kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: local-pvc1
  namespace: %s
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
  namespace: %s
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: topolvm-no-provisioner
---
apiVersion: v1
kind: Pod
metadata:
  name: testhttpd
  namespace: %s
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
      resources:
        limits:
          topolvm.cybozu.com/capacity: "2024"
        requests:
          topolvm.cybozu.com/capacity: "2024"
  volumes:
    - name: my-volume1
      persistentVolumeClaim:
        claimName: local-pvc1
    - name: my-volume2
      persistentVolumeClaim:
        claimName: local-pvc2
`, ns, ns, ns)
		stdout, stderr, err := kubectlWithInput([]byte(yml), "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n", ns, "pods/testhttpd", "-o=json")
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
				return errors.New("resources.Limits is deleted")
			}
			if v.Value() != 1<<30 {
				return fmt.Errorf("wrong limit value: actual=%d, expect=%d", v.Value(), 1<<30)
			}

			v, ok = resources.Requests[topolvm.CapacityResource]
			if !ok {
				return errors.New("resources.Requests is deleted")
			}
			if v.Value() != 1<<30 {
				return fmt.Errorf("wrong request value: actual=%d, expect=%d", v.Value(), 1<<30)
			}

			return nil
		}).Should(Succeed())
	})
})
