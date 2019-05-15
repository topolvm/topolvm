package lvmetrics

import (
	"bytes"
	"strconv"

	"github.com/cybozu-go/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

// CapacityKey is a key of Node annotation that represents VG free space.
const CapacityKey = "topolvm.cybozu.com/capacity"

var resourceEncoder runtime.Encoder = json.NewSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, false)

func encodeToJSON(obj runtime.Object) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := resourceEncoder.Encode(obj, buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// NodePatcher patches node annotations
type NodePatcher struct {
	k8sClient *kubernetes.Clientset
	nodeName  string
}

// NodeMetrics represents nodes metrics
type NodeMetrics struct {
	FreeBytes uint64
}

// Annotate adds annotations to node
func (n *NodeMetrics) Annotate(node *corev1.Node) {
	node.Annotations[CapacityKey] = strconv.FormatUint(n.FreeBytes, 10)
}

// NewNodePatcher creates NodePatcher
func NewNodePatcher(nodeName string) (*NodePatcher, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &NodePatcher{clientset, nodeName}, nil
}

// Patch updates node annotations with patch
func (n *NodePatcher) Patch(met *NodeMetrics) error {
	node, err := n.k8sClient.CoreV1().Nodes().Get(n.nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	original, err := encodeToJSON(node)
	if err != nil {
		return err
	}

	met.Annotate(node)

	modified, err := encodeToJSON(node)
	if err != nil {
		return err
	}

	if bytes.Equal(original, modified) {
		log.Debug("no diff", nil)
		return nil
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(original, modified, node)
	if err != nil {
		return err
	}

	log.Info("update annotations", map[string]interface{}{
		"node":     n.nodeName,
		"capacity": met.FreeBytes,
	})

	_, err = n.k8sClient.CoreV1().Nodes().Patch(n.nodeName, types.StrategicMergePatchType, patch)
	return err
}
