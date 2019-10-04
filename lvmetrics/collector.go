package lvmetrics

import (
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "lvmetrics"

// Collector implements prometheus.Collector interface.
type Collector struct {
	storage        atomic.Value
	availableBytes prometheus.Gauge
	lastUpdateDesc *prometheus.Desc
}

// NewCollector returns a new instance of Collector.
func NewCollector(storage atomic.Value) *Collector {
	desc := prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "last_update"), "", nil, nil)
	bytes := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "vg",
		Name:      "available_bytes",
		Help:      "LVM VG available bytes under lvmd management",
	})
	return &Collector{storage, bytes, desc}
}

// Describe sends descriptions of metrics.
func (c Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.lastUpdateDesc
	ch <- c.availableBytes.Desc()
}

// Collect sends metrics Collected from BMC via Redfish.
func (c Collector) Collect(ch chan<- prometheus.Metric) {
	v := c.storage.Load()
	if v == nil {
		return
	}
	m := v.(Metrics)
	c.availableBytes.Set(float64(m.AvailableBytes))
	ch <- c.availableBytes
}
