package lvmetrics

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type NodeUpdator struct {
	k8sClient  *kubernetes.ClientSet
	nodeName	string
}

func NewNodeUpdator(nodeName string) (*NodeUpdator, error) {
	config, err := nil, nil
	return nil, nil
}