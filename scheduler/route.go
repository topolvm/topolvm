package scheduler

import (
	"fmt"
	"net/http"
)

type scheduler struct {
	divisor float64
	spareGB uint64
}

func (s scheduler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/predicate":
		s.predicate(w, r)
	case "/prioritize":
		s.prioritize(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// NewHandler return new http.Handler of the scheduler extender
func NewHandler(divisor float64, spareGB uint64) (http.Handler, error) {
	if divisor <= 0 {
		return nil, fmt.Errorf("invalid divisor: %f", divisor)
	}
	return scheduler{divisor, spareGB}, nil
}
