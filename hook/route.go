package hook

import (
	"net/http"
)

type hook struct{}

func (h hook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/mutate":
		h.mutate(w, r)
	case "/status":
		status(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// NewHandler return new http.Handler of the scheduler extender
func NewHandler() http.Handler {
	return hook{}
}

func status(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
