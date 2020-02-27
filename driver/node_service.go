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

type nodeResourceService struct {
	client.Client
}

func newNodeResourceService(mgr manager.Manager) *nodeResourceService {
	return &nodeResourceService{Client: mgr.GetClient()}
}

func (s nodeResourceService) listNodes(ctx context.Context) (*corev1.NodeList, error) {
	nl := new(corev1.NodeList)
	err := s.List(ctx, nl)
	if err != nil {
		return nil, err
	}
	return nl, nil
}

// GetCapacity returns VG capacity of specified node.
func (s nodeResourceService) GetCapacity(ctx context.Context, requestNodeNumber string) (int64, error) {
	nl, err := s.listNodes(ctx)
	if err != nil {
		return 0, err
	}

	capacity := int64(0)
	if len(requestNodeNumber) == 0 {
		for _, node := range nl.Items {
			capacity += s.getNodeCapacity(node)
		}
		return capacity, nil
	}

	for _, node := range nl.Items {
		if nodeNumber, ok := node.Labels[topolvm.TopologyNodeKey]; ok {
			if requestNodeNumber != nodeNumber {
				continue
			}
			c, ok := node.Annotations[topolvm.CapacityKey]
			if !ok {
				return 0, fmt.Errorf("%s is not found", topolvm.CapacityKey)
			}
			return strconv.ParseInt(c, 10, 64)
		}
	}

	return 0, errors.New("capacity not found")
}

// GetMaxCapacity returns max VG capacity among nodes.
func (s nodeResourceService) GetMaxCapacity(ctx context.Context) (string, int64, error) {
	nl, err := s.listNodes(ctx)
	if err != nil {
		return "", 0, err
	}
	var nodeName string
	var maxCapacity int64
	for _, node := range nl.Items {
		c := s.getNodeCapacity(node)

		if maxCapacity < c {
			maxCapacity = c
			nodeName = node.Name
		}
	}
	return nodeName, maxCapacity, nil
}

func (s nodeResourceService) getNodeCapacity(node corev1.Node) int64 {
	c, ok := node.Annotations[topolvm.CapacityKey]
	if !ok {
		return 0
	}
	val, _ := strconv.ParseInt(c, 10, 64)
	return val
}

func (s nodeResourceService) getNode(ctx context.Context, name string) (*corev1.Node, error) {
	nl, err := s.listNodes(ctx)
	if err != nil {
		return nil, err
	}
	for _, node := range nl.Items {
		if node.Name == name {
			return &node, nil
		}
	}
	return nil, errors.New("node not found")
}
