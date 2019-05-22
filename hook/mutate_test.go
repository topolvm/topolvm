package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cybozu-go/topolvm"
	jsonpatch "github.com/evanphx/json-patch"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func prepare(h *hook, pvcs []*corev1.PersistentVolumeClaim, hasResources bool, t *testing.T) *admissionv1beta1.AdmissionReview {
	_, err := h.k8sClient.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	volumes := make([]corev1.Volume, len(pvcs))
	for i, pvc := range pvcs {
		_, err = h.k8sClient.CoreV1().PersistentVolumeClaims("default").Create(pvc)
		if err != nil {
			t.Fatal(err)
		}

		volumes[i] = corev1.Volume{
			Name: fmt.Sprintf("storage%d", i),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
				},
			},
		}
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
			Volumes: volumes,
		},
	}

	if hasResources {
		resourceList := make(corev1.ResourceList)
		resourceList[topolvm.CapacityResource] = *resource.NewQuantity(1024, resource.BinarySI)
		pod.Spec.Containers[0].Resources.Limits = resourceList
		pod.Spec.Containers[0].Resources.Requests = resourceList
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
		inputPvcs      []*corev1.PersistentVolumeClaim
		inputResources bool
		expect         []patchOperation
	}{
		{
			inputPvcs: []*corev1.PersistentVolumeClaim{
				{
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
			inputPvcs: []*corev1.PersistentVolumeClaim{
				{
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
			},
			expect: nil,
		},

		{
			inputPvcs: []*corev1.PersistentVolumeClaim{
				{
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

		{
			inputPvcs: []*corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pvc1",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: &scn,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceRequestsStorage: *resource.NewQuantity(3<<29, resource.BinarySI),
							},
						},
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimPending,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pvc2",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: &scn,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceRequestsStorage: *resource.NewQuantity(3<<29, resource.BinarySI),
							},
						},
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimPending,
					},
				},
			},
			expect: []patchOperation{
				{
					Op:   "add",
					Path: "/spec/containers/0/resources/requests",
					Value: map[string]string{
						topolvm.CapacityKey: fmt.Sprintf("%v", 4<<30),
					},
				},
				{
					Op:   "add",
					Path: "/spec/containers/0/resources/limits",
					Value: map[string]string{
						topolvm.CapacityKey: fmt.Sprintf("%v", 4<<30),
					},
				},
			},
		},

		{
			inputPvcs: []*corev1.PersistentVolumeClaim{
				{
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
			},
			inputResources: true,
			expect: []patchOperation{
				{
					Op:    "replace",
					Path:  "/spec/containers/0/resources/requests/topolvm.cybozu.com~1capacity",
					Value: fmt.Sprintf("%v", 1<<30),
				},
				{
					Op:    "replace",
					Path:  "/spec/containers/0/resources/limits/topolvm.cybozu.com~1capacity",
					Value: fmt.Sprintf("%v", 1<<30),
				},
			},
		},
	}

	hook := hook{
		testclient.NewSimpleClientset(),
	}

	for _, tt := range testCases {
		res, err := hook.mutatePod(prepare(&hook, tt.inputPvcs, tt.inputResources, t))
		if err != nil {
			t.Fatal(err)
		}

		if res.Patch != nil {
			_, err = jsonpatch.DecodePatch(res.Patch)
			if err != nil {
				t.Fatal(err)
			}
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
