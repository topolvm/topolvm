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

// NodeResourceService represents node resource service.
type NodeResourceService struct {
	client.Client
}

// NewNodeResourceService returns NodeResourceService.
func NewNodeResourceService(mgr manager.Manager) *NodeResourceService {
	return &NodeResourceService{Client: mgr.GetClient()}
}

func (s NodeResourceService) getNodes(ctx context.Context) (*corev1.NodeList, error) {
	nl := new(corev1.NodeList)
	err := s.List(ctx, nl)
	if err != nil {
		return nil, err
	}
	return nl, nil
}

func (s NodeResourceService) extractCapacityFromAnnotation(node *corev1.Node) (int64, error) {
	c, ok := node.Annotations[topolvm.CapacityKey]
	if !ok {
		return 0, fmt.Errorf("%s is not found", topolvm.CapacityKey)
	}
	return strconv.ParseInt(c, 10, 64)
}

// GetCapacityByName returns VG capacity of specified node by name.
func (s NodeResourceService) GetCapacityByName(ctx context.Context, name string) (int64, error) {
	n := new(corev1.Node)
	err := s.Get(ctx, client.ObjectKey{Name: name}, n)
	if err != nil {
		return 0, err
	}

	return s.extractCapacityFromAnnotation(n)
}

// GetCapacityByNodeNumber returns VG capacity of specified node by node number.
func (s NodeResourceService) GetCapacityByNodeNumber(ctx context.Context, requestNodeNumber string) (int64, error) {
	nl, err := s.getNodes(ctx)
	if err != nil {
		return 0, err
	}

	for _, node := range nl.Items {
		if nodeNumber, ok := node.Labels[topolvm.TopologyNodeKey]; ok {
			if requestNodeNumber != nodeNumber {
				continue
			}
			return s.extractCapacityFromAnnotation(&node)
		}
	}

	return 0, errors.New("capacity not found")
}

// GetTotalCapacity returns VG capacity of specified node by node number.
func (s NodeResourceService) GetTotalCapacity(ctx context.Context) (int64, error) {
	nl, err := s.getNodes(ctx)
	if err != nil {
		return 0, err
	}

	capacity := int64(0)
	for _, node := range nl.Items {
		c, _ := s.extractCapacityFromAnnotation(&node)
		capacity += c
	}
	return capacity, nil
}

// GetMaxCapacity returns max VG capacity among nodes.
func (s NodeResourceService) GetMaxCapacity(ctx context.Context) (string, int64, error) {
	nl, err := s.getNodes(ctx)
	if err != nil {
		return "", 0, err
	}
	var nodeName string
	var maxCapacity int64
	for _, node := range nl.Items {
		c, _ := s.extractCapacityFromAnnotation(&node)
		if maxCapacity < c {
			maxCapacity = c
			nodeName = node.Name
		}
	}
	return nodeName, maxCapacity, nil
}
