package profiling

import (
	"net/http"
	"net/http/pprof"
)

// NewProfilingServer creates a new HTTP server for profiling.
func NewProfilingServer(bindAddress string) *http.Server {
	mux := http.NewServeMux()
	srv := http.Server{
		Addr:    bindAddress,
		Handler: mux,
	}

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	return &srv
}
