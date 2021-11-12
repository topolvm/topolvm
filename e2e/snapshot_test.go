package e2e

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	snapapi "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"strings"
)

//go:embed testdata/snapshot/volume_snapshotclass.yaml
var volumeSnapshotclassYAML string

//go:embed testdata/snapshot/origin-pvc.yaml
var originPVCYAML string

//go:embed testdata/snapshot/volume_snapshot.yaml
var volumeSnapshotYAML string

//go:embed testdata/snapshot/restore-pvc.yaml
var restorePVCYAML string

//go:embed testdata/snapshot/restore-pod.yaml
var restorePODYAML string

const (
	namespace = "snapshot-test"
)

func testSnapshot() {

	It("should be deployed snapshot controller", func() {
		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n=kube-system", "pods", "--selector=app=snapshot-controller", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var podlist corev1.PodList
			err = json.Unmarshal(result, &podlist)
			if err != nil {
				return err
			}

			if len(podlist.Items) == 0 {
				return errors.New("pod is not found")
			}

			for _, pod := range podlist.Items {
				err = checkPodReady(&pod)
				if err != nil {
					return err
				}
			}

			return nil
		}).Should(Succeed())
	})

	It("should be deployed snapshot class", func() {

		_, _, err := kubectlWithInput([]byte(volumeSnapshotclassYAML), "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("checking snapshot is ok ", func() {

		_, _, err := kubectl("create", "ns", namespace)
		Expect(err).ShouldNot(HaveOccurred())

		By("starting create pod with pvc")
		_, _, err = kubectlWithInput([]byte(originPVCYAML), "-n", namespace, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			result, stderr, err := kubectl("-n", namespace, "get", "pod", "snapshot-test", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			By("checking pod is ready")
			var snapshotPod corev1.Pod
			err = json.Unmarshal(result, &snapshotPod)
			if err != nil {
				return err
			}
			err = checkPodReady(&snapshotPod)
			if err != nil {
				return err
			}
			return nil
		}).Should(Succeed())

		By("writing some data to pvc before snapshot")
		_, _, err = kubectl("-n", namespace, "exec", "snapshot-test", "--", "bash", "-c", "echo hello >> /test1/test.txt")
		Expect(err).ShouldNot(HaveOccurred())

		By("create snapshot")
		_, _, err = kubectlWithInput([]byte(volumeSnapshotYAML), "-n", namespace, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			By("checking snapshot is ready")
			result, stderr, err := kubectl("-n", namespace, "get", "volumesnapshot", "new-snapshot-test", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var s snapapi.VolumeSnapshot
			err = json.Unmarshal(result, &s)
			if err != nil {
				return err
			}

			if s.Status != nil {
				if s.Status.ReadyToUse == nil {
					return errors.New("volume snapshot is not yet ready")
				} else {
					if !(*s.Status.ReadyToUse) {
						return errors.New("volume snapshot is not yet ready")
					}
				}
			} else {
				return errors.New("volume snapshot is not yet ready")
			}
			return nil
		}).Should(Succeed())

		By("writing some data to pvc after snapshot")
		_, _, err = kubectl("-n", namespace, "exec", "snapshot-test", "--", "bash", "-c", "echo world >> /test1/test.txt")
		Expect(err).ShouldNot(HaveOccurred())

		By("restore snapshot")
		_, _, err = kubectlWithInput([]byte(restorePVCYAML), "-n", namespace, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {

			By("checking restore pvc bound")

			result, stderr, err := kubectl("-n", namespace, "get", "pvc", "myclaim-restore", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}
			var pvc corev1.PersistentVolumeClaim
			err = json.Unmarshal(result, &pvc)
			if err != nil {
				return err
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				eventMessages := ""
				result, stderr, err = kubectl("-n", namespace, "get", "events", "--field-selector=involvedObject.name=myclaim-restore", "-o=jsonpath='{.items[*].message}'")
				if err != nil {
					eventMessages = fmt.Sprintf("failed to get pvc events. stdout: %s, stderr: %s, err: %v", result, stderr, err)
				} else {
					eventMessages = strings.TrimSpace(string(result))
				}
				fmt.Printf("pvc %s is not bind. Event messages: %s", "myclaim-restore", eventMessages)
				return fmt.Errorf("pvc %s is not bind. Event messages: %s", "myclaim-restore", eventMessages)
			}
			return nil
		}).Should(Succeed())

		By("checking restore pod")
		_, _, err = kubectlWithInput([]byte(restorePODYAML), "-n", namespace, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			result, stderr, err := kubectl("-n", namespace, "get", "pod", "restore-test", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}

			var restorePod corev1.Pod
			err = json.Unmarshal(result, &restorePod)
			if err != nil {
				return err
			}
			err = checkPodReady(&restorePod)
			if err != nil {
				return err
			}

			By("checking restore data is correct")
			result, stderr, err = kubectl("-n", namespace, "exec", "restore-test", "--", "cat", "/test1/test.txt")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}
			Expect(string(result)).Should(Equal("hello\n"))
			return nil

		}).Should(Succeed())

	})

	It("clean namespace", func() {

		_, _, err := kubectlWithInput([]byte(restorePODYAML), "-n", namespace, "delete", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		_, _, err = kubectlWithInput([]byte(restorePVCYAML), "-n", namespace, "delete", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		_, _, err = kubectlWithInput([]byte(volumeSnapshotYAML), "-n", namespace, "delete", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		_, _, err = kubectlWithInput([]byte(originPVCYAML), "-n", namespace, "delete", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("delete ns " + namespace)
		_, _, err = kubectl("delete", "ns", namespace)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			_, _, err := kubectl("get", "ns", namespace)
			if err != nil {
				return nil
			} else {
				return fmt.Errorf("ns %s not deleted", namespace)
			}
		}).Should(Succeed())

	})
}

func checkPodReady(pod *corev1.Pod) error {
	podReady := false
	for _, cond := range pod.Status.Conditions {
		fmt.Fprintln(GinkgoWriter, cond)
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			podReady = true
			break
		}
	}
	if !podReady {
		return errors.New("pod is not yet ready")
	}

	return checkPodRunning(pod)
}

func checkPodRunning(pod *corev1.Pod) error {

	for _, item := range pod.Status.ContainerStatuses {
		if !item.Ready {
			return errors.New("pod container is not ready")
		}
	}
	return nil
}
