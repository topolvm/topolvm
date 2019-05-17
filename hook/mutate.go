package hook

import (
	"encoding/json"
	"net/http"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
)

func (h hook) mutate(w http.ResponseWriter, r *http.Request) {
	var input admissionv1beta1.AdmissionReview

	reader := http.MaxBytesReader(w, r.Body, 10<<20)
	err := json.NewDecoder(reader).Decode(&input)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
	}

	result := admissionv1beta1.AdmissionResponse{}
	w.Header().Set("content-type", "application/json")
	json.NewEncoder(w).Encode(result)
}
