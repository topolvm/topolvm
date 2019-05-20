package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cybozu-go/topolvm"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func prepare(h *hook, pvc *corev1.PersistentVolumeClaim, t *testing.T) *admissionv1beta1.AdmissionReview {
	_, err := h.k8sClient.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = h.k8sClient.CoreV1().PersistentVolumeClaims("default").Create(pvc)
	if err != nil {
		t.Fatal(err)
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "test",
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "storage1",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "test-pvc",
						},
					},
				},
			},
		},
	}
	_, err = h.k8sClient.CoreV1().Pods("default").Create(pod)
	if err != nil {
		t.Fatal(err)
	}

	podObj, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}

	return &admissionv1beta1.AdmissionReview{
		Request: &admissionv1beta1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: podObj,
			},
		},
	}
}

func clear(h *hook, t *testing.T) {
	h.k8sClient.CoreV1().Namespaces().Delete("default", &metav1.DeleteOptions{})
	h.k8sClient.CoreV1().PersistentVolumeClaims("default").Delete("test-pvc", &metav1.DeleteOptions{})
	h.k8sClient.CoreV1().Pods("default").Delete("test-pod", &metav1.DeleteOptions{})
}

func TestMutate(t *testing.T) {
	scn := "topolvm"
	testCases := []struct {
		inputPvc *corev1.PersistentVolumeClaim
		expect   []patchOperation
	}{
		{
			inputPvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pvc",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: &scn,
				},
				Status: corev1.PersistentVolumeClaimStatus{
					Phase: corev1.ClaimPending,
				},
			},
			expect: []patchOperation{
				{
					Op:   "add",
					Path: "/spec/containers/0/resources/requests",
					Value: map[string]string{
						topolvm.CapacityKey: fmt.Sprintf("%v", 1<<30),
					},
				},
				{
					Op:   "add",
					Path: "/spec/containers/0/resources/limits",
					Value: map[string]string{
						topolvm.CapacityKey: fmt.Sprintf("%v", 1<<30),
					},
				},
			},
		},

		{
			inputPvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pvc",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: &scn,
				},
				Status: corev1.PersistentVolumeClaimStatus{
					Phase: corev1.ClaimBound,
				},
			},
			expect: nil,
		},

		{
			inputPvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pvc",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: &scn,
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceRequestsStorage: *resource.NewQuantity(5<<30, resource.BinarySI),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceRequestsStorage: *resource.NewQuantity(3<<30, resource.BinarySI),
						},
					},
				},
				Status: corev1.PersistentVolumeClaimStatus{
					Phase: corev1.ClaimPending,
				},
			},
			expect: []patchOperation{
				{
					Op:   "add",
					Path: "/spec/containers/0/resources/requests",
					Value: map[string]string{
						topolvm.CapacityKey: fmt.Sprintf("%v", 3<<30),
					},
				},
				{
					Op:   "add",
					Path: "/spec/containers/0/resources/limits",
					Value: map[string]string{
						topolvm.CapacityKey: fmt.Sprintf("%v", 3<<30),
					},
				},
			},
		},
	}

	hook := hook{
		testclient.NewSimpleClientset(),
	}

	for _, tt := range testCases {
		res, err := hook.mutatePod(prepare(&hook, tt.inputPvc, t))
		if err != nil {
			t.Fatal(err)
		}

		expectJ, err := json.Marshal(tt.expect)
		if err != nil {
			t.Fatal(err)
		}
		if tt.expect == nil {
			expectJ = nil
		}

		if !bytes.Equal(res.Patch, expectJ) {
			t.Errorf("incorrect patch: actual=%s, expect=%s", string(res.Patch), string(expectJ))
		}

		clear(&hook, t)
	}
}
