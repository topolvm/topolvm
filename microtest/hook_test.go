package microtest

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cybozu-go/topolvm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = BeforeSuite(func() {

})

var _ = Describe("Test topolvm-hook", func() {
	BeforeEach(func() {
		kubectl("delete", "namespace", "hook-test")
		stdout, stderr, err := kubectl("create", "namespace", "hook-test")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	})
	AfterEach(func() {
		kubectl("delete", "namespace", "hook-test")
	})

	It("should be deployed topolvm-hook pod", func() {
		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n=kube-system", "pods", "--selector=app.kubernetes.io/name=topolvm-hook", "-o=json")
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
		yml := `kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: local-pvc1
  namespace: hook-test
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: topolvm
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: local-pvc2
  namespace: hook-test
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: topolvm
---
apiVersion: v1
kind: Pod
metadata:
  name: testhttpd
  namespace: hook-test
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
		stdout, stderr, err := kubectlWithInput([]byte(yml), "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n=hook-test", "pods/testhttpd", "-o=json")
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
				return errors.New("not mutated")
			}
			if v.Value() != 2<<30 {
				return fmt.Errorf("wrong limit value: actual=%d, expect=%d", v.Value(), 1<<30)
			}

			v, ok = resources.Requests[topolvm.CapacityResource]
			if !ok {
				return errors.New("not mutated")
			}
			if v.Value() != 2<<30 {
				return fmt.Errorf("wrong request value: actual=%d, expect=%d", v.Value(), 1<<30)
			}

			return nil
		}).Should(Succeed())
	})
})
