package scheduler

import (
	"fmt"
	"net/http"
)

const defaultDivisor = 1

type scheduler struct {
	defaultDivisor float64
	divisors       map[string]float64
}

func (s scheduler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/predicate":
		s.predicate(w, r)
	case "/prioritize":
		s.prioritize(w, r)
	case "/status":
		status(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// NewHandler return new http.Handler of the scheduler extender
func NewHandler(defaultDiv float64, divisors map[string]float64) (http.Handler, error) {
	if defaultDiv <= 0 {
		defaultDiv = defaultDivisor
	}
	for _, divisor := range divisors {
		if divisor <= 0 {
			return nil, fmt.Errorf("invalid divisor: %f", divisor)
		}
	}
	return scheduler{defaultDiv, divisors}, nil
}

func status(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
