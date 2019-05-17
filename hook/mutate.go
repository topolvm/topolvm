package hook

import (
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
)

func (h hook) mutate(w http.ResponseWriter, r *http.Request) {
}