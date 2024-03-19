package runners

import (
	"context"
	"io"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/topolvm/topolvm"
	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	metricsNamespace = "topolvm"

	TypeThick = "thick"
	TypeThin  = "thin"
)

var meLogger = ctrl.Log.WithName("runners").WithName("metrics_exporter")

// NodeMetrics is a set of metrics of a TopoLVM Node.
type NodeMetrics struct {
	DataPercent        float64
	MetadataPercent    float64
	FreeBytes          uint64
	SizeBytes          uint64
	ThinPoolSizeBytes  uint64
	OverProvisionBytes uint64
	DeviceClass        string
	DeviceClassType    string
}

// thinPoolMetricsExporter is the subset of metricsExporter corresponding to the deviceclass target
type thinPoolMetricsExporter struct {
	tpSizeBytes      *prometheus.GaugeVec
	dataPercent      *prometheus.GaugeVec
	metadataPercent  *prometheus.GaugeVec
	opAvailableBytes *prometheus.GaugeVec
}

type metricsExporter struct {
	client         client.Client
	nodeName       string
	vgService      proto.VGServiceClient
	availableBytes *prometheus.GaugeVec
	sizeBytes      *prometheus.GaugeVec
	thinPool       *thinPoolMetricsExporter
}

var _ manager.LeaderElectionRunnable = &metricsExporter{}

// NewMetricsExporter creates controller-runtime's manager.Runnable to run
// a metrics exporter for a node.
func NewMetricsExporter(vgServiceClient proto.VGServiceClient, client client.Client, nodeName string) manager.Runnable {
	// metrics available under volumegroup subsystem
	availableBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "volumegroup",
		Name:        "available_bytes",
		Help:        "LVM VG available bytes under lvmd management",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"device_class"})

	sizeBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "volumegroup",
		Name:        "size_bytes",
		Help:        "LVM VG size bytes under lvmd management",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"device_class"})

	// metrics available under thinpool subsystem
	tpSizeBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "thinpool",
		Name:        "size_bytes",
		Help:        "LVM VG Thin Pool raw size bytes",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"device_class"})

	dataPercent := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "thinpool",
		Name:        "data_percent",
		Help:        "LVM VG Thin Pool data usage percent",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"device_class"})

	metadataPercent := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "thinpool",
		Name:        "metadata_percent",
		Help:        "LVM VG Thin Pool metadata usage percent",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"device_class"})

	opAvailableBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "thinpool",
		Name:        "overprovisioned_available",
		Help:        "LVM VG Thin Pool bytes available with overprovisioning",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"device_class"})

	return &metricsExporter{
		client:         client,
		nodeName:       nodeName,
		vgService:      vgServiceClient,
		availableBytes: availableBytes,
		sizeBytes:      sizeBytes,
		thinPool: &thinPoolMetricsExporter{
			tpSizeBytes:      tpSizeBytes,
			dataPercent:      dataPercent,
			metadataPercent:  metadataPercent,
			opAvailableBytes: opAvailableBytes,
		},
	}
}

func (m *metricsExporter) getCollectors() []prometheus.Collector {
	return []prometheus.Collector{
		m.availableBytes,
		m.sizeBytes,
		m.thinPool.tpSizeBytes,
		m.thinPool.dataPercent,
		m.thinPool.metadataPercent,
		m.thinPool.opAvailableBytes,
	}
}

func (m *metricsExporter) registerAll() error {
	for _, c := range m.getCollectors() {
		if err := metrics.Registry.Register(c); err != nil {
			return err
		}
	}
	return nil
}

func (m *metricsExporter) unregisterAll() {
	for _, c := range m.getCollectors() {
		metrics.Registry.Unregister(c)
	}
}

// Start implements controller-runtime's manager.Runnable.
func (m *metricsExporter) Start(ctx context.Context) error {
	if err := m.registerAll(); err != nil {
		return err
	}

	metricsCh := make(chan NodeMetrics)
	go func() {
		for {
			select {
			case <-ctx.Done():
				m.unregisterAll()
				return
			case met := <-metricsCh:

				// metrics for volumegroup subsystem, these are exported from thinpool type as well
				m.availableBytes.WithLabelValues(met.DeviceClass).Set(float64(met.FreeBytes))
				m.sizeBytes.WithLabelValues(met.DeviceClass).Set(float64(met.SizeBytes))

				if met.DeviceClassType == TypeThin {
					// metrics for thinpool subsystem exclusively
					m.thinPool.tpSizeBytes.WithLabelValues(met.DeviceClass).Set(float64(met.ThinPoolSizeBytes))
					m.thinPool.dataPercent.WithLabelValues(met.DeviceClass).Set(float64(met.DataPercent))
					m.thinPool.metadataPercent.WithLabelValues(met.DeviceClass).Set(float64(met.MetadataPercent))
					m.thinPool.opAvailableBytes.WithLabelValues(met.DeviceClass).Set(float64(met.OverProvisionBytes))
				}
			}
		}
	}()

	wc, err := m.vgService.Watch(ctx, &proto.Empty{})
	if err != nil {
		return err
	}
	return m.updateNode(ctx, wc, metricsCh)
}

// NeedLeaderElection implements controller-runtime's manager.LeaderElectionRunnable.
func (m *metricsExporter) NeedLeaderElection() bool {
	return false
}

func (m *metricsExporter) updateNode(ctx context.Context, wc proto.VGService_WatchClient, ch chan<- NodeMetrics) error {
	for {
		res, err := wc.Recv()
		switch {
		case err == io.EOF:
			return nil
		case status.Code(err) == codes.Canceled:
			return nil
		case err == nil:
		default:
			return err
		}

		for _, item := range res.Items {
			if item.ThinPool != nil {
				ch <- NodeMetrics{
					DeviceClass:        item.DeviceClass,
					FreeBytes:          item.FreeBytes,
					SizeBytes:          item.SizeBytes,
					ThinPoolSizeBytes:  item.ThinPool.SizeBytes,
					DataPercent:        item.ThinPool.DataPercent,
					MetadataPercent:    item.ThinPool.MetadataPercent,
					DeviceClassType:    TypeThin,
					OverProvisionBytes: item.ThinPool.OverprovisionBytes,
				}
			} else {
				ch <- NodeMetrics{
					DeviceClass:     item.DeviceClass,
					FreeBytes:       item.FreeBytes,
					SizeBytes:       item.SizeBytes,
					DeviceClassType: TypeThick,
				}
			}
		}

		var nodeMetadata v1.PartialObjectMetadata

		nodeMetadata.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Node"))
		if err := m.client.Get(ctx, types.NamespacedName{Name: m.nodeName}, &nodeMetadata); err != nil {
			return err
		}

		if nodeMetadata.DeletionTimestamp != nil {
			meLogger.Info("node is deleting")
			break
		}
		nodeMetadata2 := nodeMetadata.DeepCopy()

		controllerutil.AddFinalizer(nodeMetadata2, topolvm.GetNodeFinalizer())

		nodeMetadata2.Annotations[topolvm.GetCapacityKeyPrefix()+topolvm.DefaultDeviceClassAnnotationName] = strconv.FormatUint(res.FreeBytes, 10)
		for _, item := range res.Items {
			var freeSize uint64
			if item.ThinPool != nil {
				freeSize = item.ThinPool.OverprovisionBytes
			} else {
				freeSize = item.FreeBytes
			}
			nodeMetadata2.Annotations[topolvm.GetCapacityKeyPrefix()+item.DeviceClass] = strconv.FormatUint(freeSize, 10)
		}
		if err := m.client.Patch(ctx, nodeMetadata2, client.MergeFrom(&nodeMetadata)); err != nil {
			return err
		}
	}

	return nil
}
