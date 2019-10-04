package lvmetrics

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/well"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type logger struct{}

func (l logger) Println(v ...interface{}) {
	log.Error(fmt.Sprint(v...), nil)
}

type PromMetricsServer struct {
	port      string
	collector *Collector
}

// NewPromMetricsServer constructs metrics exporter of lvmetrics
func NewPromMetricsServer(port string, storage *atomic.Value) PromMetricsServer {
	collector := NewCollector(storage)
	return PromMetricsServer{port, collector}
}

func (p PromMetricsServer) Run(ctx context.Context) error {
	registry := prometheus.NewRegistry()
	err := registry.Register(p.collector)
	if err != nil {
		return err
	}

	handler := promhttp.HandlerFor(registry,
		promhttp.HandlerOpts{
			ErrorLog:      logger{},
			ErrorHandling: promhttp.ContinueOnError,
		})
	mux := http.NewServeMux()
	mux.Handle("/metrics", handler)
	serv := &well.HTTPServer{
		Server: &http.Server{
			Addr:    p.port,
			Handler: mux,
		},
	}
	return serv.ListenAndServe()
}
