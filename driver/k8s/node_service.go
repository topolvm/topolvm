package k8s

import (
	"context"
	"errors"
	"strconv"

	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// ErrNodeNotFound represents the error that node is not found.
var ErrNodeNotFound = errors.New("node not found")
var ErrDeviceClassNotFound = errors.New("device class not found")

// NodeService represents node service.
type NodeService struct {
	// it is safe to use cache reader because updating node annotations is periodic.
	reader client.Reader
}

// NewNodeService returns NodeService.
func NewNodeService(mgr manager.Manager) *NodeService {
	return &NodeService{reader: mgr.GetClient()}
}

func (s NodeService) getNodes(ctx context.Context) (*corev1.NodeList, error) {
	nl := new(corev1.NodeList)
	err := s.reader.List(ctx, nl)
	if err != nil {
		return nil, err
	}
	return nl, nil
}

func (s NodeService) extractCapacityFromAnnotation(node *corev1.Node, deviceClass string) (int64, error) {
	if deviceClass == topolvm.DefaultDeviceClassName {
		deviceClass = topolvm.DefaultDeviceClassAnnotationName
	}
	c, ok := node.Annotations[topolvm.GetCapacityKeyPrefix()+deviceClass]
	if !ok {
		return 0, ErrDeviceClassNotFound
	}
	return strconv.ParseInt(c, 10, 64)
}

// GetCapacityByName returns VG capacity of specified node by name.
func (s NodeService) GetCapacityByName(ctx context.Context, name, deviceClass string) (int64, error) {
	n := new(corev1.Node)
	err := s.reader.Get(ctx, client.ObjectKey{Name: name}, n)
	if err != nil {
		return 0, err
	}

	return s.extractCapacityFromAnnotation(n, deviceClass)
}

// GetCapacityByTopologyLabel returns VG capacity of specified node by TopoLVM's topology label.
func (s NodeService) GetCapacityByTopologyLabel(ctx context.Context, topology, dc string) (int64, error) {
	nl, err := s.getNodes(ctx)
	if err != nil {
		return 0, err
	}

	for _, node := range nl.Items {
		if v, ok := node.Labels[topolvm.GetTopologyNodeKey()]; ok {
			if v != topology {
				continue
			}
			return s.extractCapacityFromAnnotation(&node, dc)
		}
	}

	return 0, ErrNodeNotFound
}

// GetTotalCapacity returns total VG capacity of all nodes.
func (s NodeService) GetTotalCapacity(ctx context.Context, dc string) (int64, error) {
	nl, err := s.getNodes(ctx)
	if err != nil {
		return 0, err
	}

	capacity := int64(0)
	for _, node := range nl.Items {
		c, _ := s.extractCapacityFromAnnotation(&node, dc)
		capacity += c
	}
	return capacity, nil
}

// GetMaxCapacity returns max VG capacity among nodes.
func (s NodeService) GetMaxCapacity(ctx context.Context, deviceClass string) (string, int64, error) {
	nl, err := s.getNodes(ctx)
	if err != nil {
		return "", 0, err
	}
	var nodeName string
	var maxCapacity int64
	for _, node := range nl.Items {
		c, _ := s.extractCapacityFromAnnotation(&node, deviceClass)
		if maxCapacity < c {
			maxCapacity = c
			nodeName = node.Name
		}
	}
	return nodeName, maxCapacity, nil
}
