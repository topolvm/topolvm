package runners

import (
	"context"
	"io"
	"strconv"

	"github.com/cybozu-go/topolvm"
	"github.com/cybozu-go/topolvm/lvmd/proto"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const metricsNamespace = "topolvm"

var meLogger = logf.Log.WithName("runners").WithName("metrics_exporter")

// NodeMetrics is a set of metrics of a TopoLVM Node.
type NodeMetrics struct {
	FreeBytes uint64
}

type metricsExporter struct {
	client.Client
	nodeName       string
	vgService      proto.VGServiceClient
	availableBytes prometheus.Gauge
}

var _ manager.LeaderElectionRunnable = &metricsExporter{}

// NewMetricsExporter creates controller-runtime's manager.Runnable to run
// a metrics exporter for a node.
func NewMetricsExporter(conn *grpc.ClientConn, mgr manager.Manager, nodeName string) manager.Runnable {
	availableBytes := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "volumegroup",
		Name:        "available_bytes",
		Help:        "LVM VG available bytes under lvmd management",
		ConstLabels: prometheus.Labels{"node": nodeName},
	})
	metrics.Registry.MustRegister(availableBytes)

	return &metricsExporter{
		Client:         mgr.GetClient(),
		nodeName:       nodeName,
		vgService:      proto.NewVGServiceClient(conn),
		availableBytes: availableBytes,
	}
}

// Start implements controller-runtime's manager.Runnable.
func (m *metricsExporter) Start(ch <-chan struct{}) error {
	metricsCh := make(chan NodeMetrics)
	go func() {
		for {
			select {
			case <-ch:
				return
			case met := <-metricsCh:
				m.availableBytes.Set(float64(met.FreeBytes))
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ch
		cancel()
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

		var availableBytes uint64
		for _, item := range res.Items {
			availableBytes += item.FreeBytes
		}
		ch <- NodeMetrics{
			FreeBytes: availableBytes,
		}

		var node corev1.Node
		if err := m.Get(ctx, types.NamespacedName{Name: m.nodeName}, &node); err != nil {
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

		for _, item := range res.Items {
			node2.Annotations[topolvm.CapacityKey + "-" + item.VgName] = strconv.FormatUint(item.FreeBytes, 10)
		}
		if err := m.Patch(ctx, node2, client.MergeFrom(&node)); err != nil {
			return err
		}
	}

	return nil
}
