package hook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/topolvm"
	"k8s.io/api/admission/v1beta1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

const defaultSize = 1 << 30

func (h hook) hasTopolvmPVC(pod *corev1.Pod) (bool, error) {
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim == nil {
			continue
		}
		claimName := vol.PersistentVolumeClaim.ClaimName

		pvc, err := h.k8sClient.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(claimName, metav1.GetOptions{})
		if err != nil {
			log.Error("failed to get pvc", map[string]interface{}{
				"pod":       pod.Name,
				"namespace": pod.Namespace,
				"pvc":       claimName,
			})
			return false, err
		}

		if pvc.Status.Phase != corev1.ClaimPending {
			continue
		}

		sc, err := h.k8sClient.StorageV1().StorageClasses().Get(*pvc.Spec.StorageClassName, metav1.GetOptions{})
		if err != nil {
			log.Error("failed to get sc", map[string]interface{}{
				"pod":       pod.Name,
				"namespace": pod.Namespace,
				"sc":        *pvc.Spec.StorageClassName,
			})
			return false, err
		}
		if sc.Provisioner == topolvm.PluginName {
			return true, nil
		}
	}
	return false, nil
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
			log.Error("failed to get pvc", map[string]interface{}{
				"pod":       pod.Name,
				"namespace": pod.Namespace,
				"pvc":       claimName,
			})
			continue
		}

		if pvc.Status.Phase != corev1.ClaimPending {
			continue
		}

		sc, err := h.k8sClient.StorageV1().StorageClasses().Get(*pvc.Spec.StorageClassName, metav1.GetOptions{})
		if err != nil {
			log.Error("failed to get sc", map[string]interface{}{
				"pod":       pod.Name,
				"namespace": pod.Namespace,
				"sc":        *pvc.Spec.StorageClassName,
			})
			continue
		}
		if sc.Provisioner == topolvm.PluginName {
			req, ok := pvc.Spec.Resources.Requests[corev1.ResourceRequestsStorage]
			if ok && req.Value() != 0 {
				requested += ((req.Value()-1)>>30 + 1) << 30
			} else {
				requested += defaultSize
			}
		}
	}

	return requested
}

func createPatch(request int64, pod *corev1.Pod) []patchOperation {
	var patch []patchOperation

	requestedStr := fmt.Sprintf("%v", request)

	reqPath := "/spec/containers/0/resources/requests"
	limitPath := "/spec/containers/0/resources/limits"

	escapedCapacityKey := strings.ReplaceAll(topolvm.CapacityKey, "/", "~1")

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
			Path:  reqPath + "/" + escapedCapacityKey,
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
			Path:  limitPath + "/" + escapedCapacityKey,
			Value: requestedStr,
		})
	}

	return patch
}

func (h hook) mutatePod(ar *admissionv1beta1.AdmissionReview) (*admissionv1beta1.AdmissionResponse, error) {
	req := ar.Request
	if req == nil {
		return nil, nil
	}

	pod := new(corev1.Pod)
	err := json.Unmarshal(req.Object.Raw, pod)
	if err != nil {
		return nil, err
	}
	hasPVC, err := h.hasTopolvmPVC(pod)
	if err != nil {
		return nil, err
	}
	if !hasPVC {
		return &admissionv1beta1.AdmissionResponse{
			Allowed: true,
		}, nil
	}

	requested := h.calcRequested(pod)
	patch, err := json.Marshal(createPatch(requested, pod))
	if err != nil {
		return nil, err
	}

	log.Info("mutate pod", map[string]interface{}{
		"pod":       req.Name,
		"namespace": req.Namespace,
		"requested": requested,
	})

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
		log.Error("bad request", map[string]interface{}{})
		http.Error(w, "bad request", http.StatusBadRequest)
	}

	result, err := h.mutatePod(&input)
	if err != nil {
		log.Error("failed to mutate", map[string]interface{}{
			"name": input.Request.Name,
		})
		result = &admissionv1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	output := v1beta1.AdmissionReview{}
	if result != nil {
		output.Response = result
		if input.Request != nil {
			output.Response.UID = input.Request.UID
		}
	}

	w.Header().Set("content-type", "application/json")
	json.NewEncoder(w).Encode(output)
}
