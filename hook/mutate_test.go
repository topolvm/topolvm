package hook

import (
	"encoding/json"
	"testing"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestMutate(t *testing.T) {
	hook := hook{
		testclient.NewSimpleClientset(),
	}

	_, err := hook.k8sClient.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	scn := "topolvm"
	_, err = hook.k8sClient.CoreV1().PersistentVolumeClaims("default").Create(&corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pvc",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scn,
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimPending,
		},
	})
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
	_, err = hook.k8sClient.CoreV1().Pods("default").Create(pod)
	if err != nil {
		t.Fatal(err)
	}

	podObj, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}
	res, err := hook.mutatePod(&admissionv1beta1.AdmissionReview{
		Request: &admissionv1beta1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: podObj,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var patch []patchOperation
	err = json.Unmarshal(res.Patch, &patch)
	if err != nil {
		t.Fatal(err)
	}
	if len(patch) != 2 {
		t.Fatalf("unexpected patch: %s", patch)
	}
}
