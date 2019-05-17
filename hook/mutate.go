package hook

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cybozu-go/topolvm"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func (h hook) hasTopolvmPVC(pod *corev1.Pod) bool {
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim == nil {
			continue
		}
		claimName := vol.PersistentVolumeClaim.ClaimName

		pvc, err := h.k8sClient.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(claimName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		if pvc.Status.Phase != corev1.ClaimPending {
			continue
		}

		if *pvc.Spec.StorageClassName == topolvm.StorageClassName {
			return true
		}
	}
	return false
}

func (h hook) calcRequested(pod *corev1.Pod) int64 {
	var requested int64

	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim == nil {
			continue
		}
		claimName := vol.PersistentVolumeClaim.ClaimName

		pvc, err := h.k8sClient.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(claimName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		if pvc.Status.Phase != corev1.ClaimPending {
			continue
		}

		if *pvc.Spec.StorageClassName == topolvm.StorageClassName {
			req := pvc.Spec.Resources.Requests[corev1.ResourceRequestsStorage]
			requested += req.Value()
		}
	}
	return requested
}

func createPatch(request int64, pod *corev1.Pod) []patchOperation {
	var patch []patchOperation

	requestedStr := fmt.Sprintf("%v", request)

	reqPath := "/spec/containers/0/resources/requests"
	limitPath := "/spec/containers/0/resources/requests"

	container := pod.Spec.Containers[0]
	_, ok := container.Resources.Requests[topolvm.CapacityResource]
	if !ok {
		patch = append(patch, patchOperation{
			Op:   "add",
			Path: reqPath,
			Value: map[string]string{
				topolvm.CapacityKey: requestedStr,
			},
		})
	} else {
		patch = append(patch, patchOperation{
			Op:    "replace",
			Path:  reqPath + "/" + topolvm.CapacityKey,
			Value: requestedStr,
		})
	}

	_, ok = container.Resources.Limits[topolvm.CapacityResource]
	if !ok {
		patch = append(patch, patchOperation{
			Op:   "add",
			Path: limitPath,
			Value: map[string]string{
				topolvm.CapacityKey: requestedStr,
			},
		})
	} else {
		patch = append(patch, patchOperation{
			Op:    "replace",
			Path:  limitPath + "/" + topolvm.CapacityKey,
			Value: requestedStr,
		})
	}

	return patch
}

func (h hook) mutatePod(ar *admissionv1beta1.AdmissionReview) (*admissionv1beta1.AdmissionResponse, error) {
	req := ar.Request
	pod := new(corev1.Pod)

	err := json.Unmarshal(req.Object.Raw, pod)
	if err != nil {
		return nil, err
	}
	if !h.hasTopolvmPVC(pod) {
		return &admissionv1beta1.AdmissionResponse{
			Allowed: true,
		}, nil
	}

	requested := h.calcRequested(pod)
	patch, err := json.Marshal(createPatch(requested, pod))
	if err != nil {
		return nil, err
	}

	return &admissionv1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   patch,
		PatchType: func() *admissionv1beta1.PatchType {
			pt := admissionv1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}, nil
}

func (h hook) mutate(w http.ResponseWriter, r *http.Request) {
	var input admissionv1beta1.AdmissionReview

	reader := http.MaxBytesReader(w, r.Body, 10<<20)
	err := json.NewDecoder(reader).Decode(&input)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
	}

	result, err := h.mutatePod(&input)
	if err != nil {
		result = &admissionv1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	w.Header().Set("content-type", "application/json")
	json.NewEncoder(w).Encode(result)
}
