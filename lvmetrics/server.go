package lvmetrics

import (
	"context"
	"net/http"
	"sync/atomic"

	"github.com/cybozu-go/well"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PromMetricsServer is the Prometheus exporter for lvmetrics
type PromMetricsServer struct {
	addr      string
	collector *Collector
}

// NewPromMetricsServer constructs metrics exporter of lvmetrics
func NewPromMetricsServer(addr string, storage *atomic.Value) PromMetricsServer {
	collector := NewCollector(storage)
	return PromMetricsServer{addr, collector}
}

// Run starts the server
func (p PromMetricsServer) Run(ctx context.Context) error {
	registry := prometheus.NewRegistry()
	err := registry.Register(p.collector)
	if err != nil {
		return err
	}

	handler := promhttp.HandlerFor(registry,
		promhttp.HandlerOpts{
			ErrorHandling: promhttp.ContinueOnError,
		})
	mux := http.NewServeMux()
	mux.Handle("/metrics", handler)
	serv := &well.HTTPServer{
		Server: &http.Server{
			Addr:    p.addr,
			Handler: mux,
		},
	}
	return serv.ListenAndServe()
}
