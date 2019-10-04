package lvmetrics

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/well"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type logger struct{}

func (l logger) Println(v ...interface{}) {
	log.Error(fmt.Sprint(v...), nil)
}

func metricsHandler(ctx context.Context) error {
	return nil
}

type PromMetricsServer struct {
	addr      string
	collector Collector
}

func NewPromMetricsServer(addr string, collector Collector) PromMetricsServer {
	return PromMetricsServer{addr, collector}
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
			Addr:    p.addr,
			Handler: mux,
		},
	}
	return serv.ListenAndServe()
}
