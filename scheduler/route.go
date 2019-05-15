package scheduler

import "net/http"

// NewHandler return new http.Handler of the scheduler extender
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/predicate", predicate)
	mux.HandleFunc("/prioritize", nil)

	return mux
}
