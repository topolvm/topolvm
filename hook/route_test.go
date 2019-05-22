package hook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/api/admission/v1beta1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func testMutate(t *testing.T) {
	scn := "topolvm"
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pvc",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scn,
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimPending,
		},
	}

	pod := corev1.Pod{
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
							ClaimName: pvc.Name,
						},
					},
				},
			},
		},
	}
	podJSON, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}

	uid := types.UID("40ef3e89-a7c6-44a1-86d6-f6ece9f860ed")
	ar := admissionv1beta1.AdmissionReview{
		Request: &admissionv1beta1.AdmissionRequest{
			UID: uid,
			Object: runtime.RawExtension{
				Raw: podJSON,
			},
		},
	}
	arJSON, err := json.Marshal(ar)
	if err != nil {
		t.Fatal(err)
	}

	handler := hook{
		testclient.NewSimpleClientset(),
	}

	_, err = handler.k8sClient.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = handler.k8sClient.CoreV1().PersistentVolumeClaims("default").Create(&pvc)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/mutate", bytes.NewReader(arJSON))
	handler.ServeHTTP(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Error("resp.StatusCode != http.StatusOK:", resp.StatusCode)
	}

	result := new(v1beta1.AdmissionReview)
	err = json.NewDecoder(resp.Body).Decode(result)
	if err != nil {
		t.Fatal(err)
	}

	if result.Response == nil {
		t.Fatal("result.Response == nil")
	}
	if result.Response.UID != uid {
		t.Error("result.Response.UID != uid:", result.Response.UID)
	}
	if !result.Response.Allowed {
		t.Error("result.Response.Allowed == false")
	}
	if result.Response.PatchType == nil {
		t.Fatal("result.Response.PatchType == nil")
	}
	if *result.Response.PatchType != admissionv1beta1.PatchTypeJSONPatch {
		t.Error("*result.Response.PatchType == PatchTypeJSONPatch:", *result.Response.PatchType)
	}

	expected, err := handler.mutatePod(&ar)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result.Response.Patch, expected.Patch) {
		t.Errorf("wrong result.Response.Patch; expected: %s, result: %s", string(expected.Patch), string(result.Response.Patch))
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/mutate", nil)
	handler.ServeHTTP(w, r)

	resp = w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Error("resp.StatusCode != http.StatusBadRequest:", resp.StatusCode)
	}
}

func TestRoute(t *testing.T) {
	t.Run("mutate", testMutate)
}
