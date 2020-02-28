package driver

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

// ErrNodeCapacityNotAnnotated represents that node is found but capacity is not annotated.
var ErrNodeCapacityNotAnnotated = fmt.Errorf("%s is not found", topolvm.CapacityKey)

// NodeResourceService represents node resource service.
type NodeResourceService struct {
	client.Client
}

// NewNodeResourceService returns NodeResourceService.
func NewNodeResourceService(mgr manager.Manager) *NodeResourceService {
	return &NodeResourceService{Client: mgr.GetClient()}
}

func (s NodeResourceService) listNodes(ctx context.Context) (*corev1.NodeList, error) {
	nl := new(corev1.NodeList)
	err := s.List(ctx, nl)
	if err != nil {
		return nil, err
	}
	return nl, nil
}

// GetCapacity returns VG capacity of specified node.
func (s NodeResourceService) GetCapacity(ctx context.Context, requestNodeNumber string) (int64, error) {
	nl, err := s.listNodes(ctx)
	if err != nil {
		return 0, err
	}

	capacity := int64(0)
	if len(requestNodeNumber) == 0 {
		for _, node := range nl.Items {
			c, _ := s.getNodeCapacityFromAnnotation(&node)
			capacity += c
		}
		return capacity, nil
	}

	for _, node := range nl.Items {
		if nodeNumber, ok := node.Labels[topolvm.TopologyNodeKey]; ok {
			if requestNodeNumber != nodeNumber {
				continue
			}
			return s.getNodeCapacityFromAnnotation(&node)
		}
	}

	return 0, errors.New("capacity not found")
}

// GetMaxCapacity returns max VG capacity among nodes.
func (s NodeResourceService) GetMaxCapacity(ctx context.Context) (string, int64, error) {
	nl, err := s.listNodes(ctx)
	if err != nil {
		return "", 0, err
	}
	var nodeName string
	var maxCapacity int64
	for _, node := range nl.Items {
		c, _ := s.getNodeCapacityFromAnnotation(&node)
		if maxCapacity < c {
			maxCapacity = c
			nodeName = node.Name
		}
	}
	return nodeName, maxCapacity, nil
}

// GetNodeCapacity returns node's capacity
func (s NodeResourceService) GetNodeCapacity(ctx context.Context, name string) (int64, error) {
	n := new(corev1.Node)
	err := s.Get(ctx, client.ObjectKey{Name: name}, n)
	if err != nil {
		return 0, err
	}

	return s.getNodeCapacityFromAnnotation(n)
}

func (s NodeResourceService) getNodeCapacityFromAnnotation(node *corev1.Node) (int64, error) {
	c, ok := node.Annotations[topolvm.CapacityKey]
	if !ok {
		return 0, ErrNodeCapacityNotAnnotated
	}
	return strconv.ParseInt(c, 10, 64)
}
