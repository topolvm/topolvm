package runners

import (
	"context"
	"io"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/topolvm/topolvm"
	"github.com/topolvm/topolvm/lvmd/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	tpSizeBytes     *prometheus.GaugeVec
	dataPercent     *prometheus.GaugeVec
	metadataPercent *prometheus.GaugeVec
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
func NewMetricsExporter(conn *grpc.ClientConn, mgr manager.Manager, nodeName string) manager.Runnable {

	// metrics available under volumegroup subsystem
	availableBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "volumegroup",
		Name:        "available_bytes",
		Help:        "LVM VG available bytes under lvmd management",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"device_class"})
	metrics.Registry.MustRegister(availableBytes)

	sizeBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "volumegroup",
		Name:        "size_bytes",
		Help:        "LVM VG size bytes under lvmd management",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"device_class"})
	metrics.Registry.MustRegister(sizeBytes)

	// metrics available under thinpool subsystem
	tpSizeBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "thinpool",
		Name:        "size_bytes",
		Help:        "LVM VG Thin Pool raw size bytes",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"device_class"})
	metrics.Registry.MustRegister(tpSizeBytes)

	dataPercent := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "thinpool",
		Name:        "data_percent",
		Help:        "LVM VG Thin Pool data usage percent",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"device_class"})
	metrics.Registry.MustRegister(dataPercent)

	metadataPercent := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "thinpool",
		Name:        "metadata_percent",
		Help:        "LVM VG Thin Pool metadata usage percent",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"device_class"})
	metrics.Registry.MustRegister(metadataPercent)

	return &metricsExporter{
		client:         mgr.GetClient(),
		nodeName:       nodeName,
		vgService:      proto.NewVGServiceClient(conn),
		availableBytes: availableBytes,
		sizeBytes:      sizeBytes,
		thinPool: &thinPoolMetricsExporter{
			tpSizeBytes:     tpSizeBytes,
			dataPercent:     dataPercent,
			metadataPercent: metadataPercent,
		},
	}
}

// Start implements controller-runtime's manager.Runnable.
func (m *metricsExporter) Start(ctx context.Context) error {
	metricsCh := make(chan NodeMetrics)
	go func() {
		for {
			select {
			case <-ctx.Done():
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
					DeviceClass:       item.DeviceClass,
					FreeBytes:         item.FreeBytes,
					SizeBytes:         item.SizeBytes,
					ThinPoolSizeBytes: item.ThinPool.SizeBytes,
					DataPercent:       item.ThinPool.DataPercent,
					MetadataPercent:   item.ThinPool.MetadataPercent,
					DeviceClassType:   TypeThin,
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

		var node corev1.Node
		if err := m.client.Get(ctx, types.NamespacedName{Name: m.nodeName}, &node); err != nil {
			return err
		}

		if node.DeletionTimestamp != nil {
			meLogger.Info("node is deleting")
			break
		}

		node2 := node.DeepCopy()

		var hasFinalizer bool
		for _, fin := range node.Finalizers {
			if fin == topolvm.NodeFinalizer {
				hasFinalizer = true
				break
			}
		}
		if !hasFinalizer {
			node2.Finalizers = append(node2.Finalizers, topolvm.NodeFinalizer)
		}

		node2.Annotations[topolvm.CapacityKeyPrefix+topolvm.DefaultDeviceClassAnnotationName] = strconv.FormatUint(res.FreeBytes, 10)
		for _, item := range res.Items {
			var freeSize uint64
			if item.ThinPool != nil {
				freeSize = item.ThinPool.OverprovisionBytes
			} else {
				freeSize = item.FreeBytes
			}
			node2.Annotations[topolvm.CapacityKeyPrefix+item.DeviceClass] = strconv.FormatUint(freeSize, 10)
		}
		if err := m.client.Patch(ctx, node2, client.MergeFrom(&node)); err != nil {
			return err
		}
	}

	return nil
}
