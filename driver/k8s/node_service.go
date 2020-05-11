package k8s

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/cybozu-go/topolvm"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// ErrNodeNotFound represents the error that node is not found.
var ErrNodeNotFound = errors.New("node not found")

// NodeService represents node service.
type NodeService struct {
	client.Client
}

// NewNodeService returns NodeService.
func NewNodeService(mgr manager.Manager) *NodeService {
	return &NodeService{Client: mgr.GetClient()}
}

func (s NodeService) getNodes(ctx context.Context) (*corev1.NodeList, error) {
	nl := new(corev1.NodeList)
	err := s.List(ctx, nl)
	if err != nil {
		return nil, err
	}
	return nl, nil
}

func (s NodeService) extractCapacityFromAnnotation(node *corev1.Node, vgName string) (int64, error) {
	c, ok := node.Annotations[topolvm.CapacityKey+vgName]
	if !ok {
		return 0, fmt.Errorf("%s is not found", topolvm.CapacityKey+vgName)
	}
	return strconv.ParseInt(c, 10, 64)
}

// GetCapacityByName returns VG capacity of specified node by name.
func (s NodeService) GetCapacityByName(ctx context.Context, name, vgName string) (int64, error) {
	n := new(corev1.Node)
	err := s.Get(ctx, client.ObjectKey{Name: name}, n)
	if err != nil {
		return 0, err
	}

	return s.extractCapacityFromAnnotation(n, vgName)
}

// GetCapacityByTopologyLabel returns VG capacity of specified node by TopoLVM's topology label.
func (s NodeService) GetCapacityByTopologyLabel(ctx context.Context, topology, vg string) (int64, error) {
	nl, err := s.getNodes(ctx)
	if err != nil {
		return 0, err
	}

	for _, node := range nl.Items {
		if v, ok := node.Labels[topolvm.TopologyNodeKey]; ok {
			if v != topology {
				continue
			}
			return s.extractCapacityFromAnnotation(&node, vg)
		}
	}

	return 0, ErrNodeNotFound
}

// GetTotalCapacity returns total VG capacity of all nodes.
func (s NodeService) GetTotalCapacity(ctx context.Context, vg string) (int64, error) {
	nl, err := s.getNodes(ctx)
	if err != nil {
		return 0, err
	}

	capacity := int64(0)
	for _, node := range nl.Items {
		c, _ := s.extractCapacityFromAnnotation(&node, vg)
		capacity += c
	}
	return capacity, nil
}

// GetMaxCapacity returns max VG capacity among nodes.
func (s NodeService) GetMaxCapacity(ctx context.Context, vgName string) (string, int64, error) {
	nl, err := s.getNodes(ctx)
	if err != nil {
		return "", 0, err
	}
	var nodeName string
	var maxCapacity int64
	for _, node := range nl.Items {
		c, _ := s.extractCapacityFromAnnotation(&node, vgName)
		if maxCapacity < c {
			maxCapacity = c
			nodeName = node.Name
		}
	}
	return nodeName, maxCapacity, nil
}
