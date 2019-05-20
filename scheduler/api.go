package scheduler

import (
	apiv1 "k8s.io/api/core/v1"
)

// ExtenderArgs is copied from https://godoc.org/k8s.io/kubernetes/pkg/scheduler/api/v1#ExtenderArgs
type ExtenderArgs struct {
	// Pod being scheduled
	Pod *apiv1.Pod `json:"pod"`
	// List of candidate nodes where the pod can be scheduled; to be populated
	// only if ExtenderConfig.NodeCacheCapable == false
	Nodes *apiv1.NodeList `json:"nodes,omitempty"`
	// List of candidate node names where the pod can be scheduled; to be
	// populated only if ExtenderConfig.NodeCacheCapable == true
	NodeNames *[]string `json:"nodenames,omitempty"`
}

// HostPriority is copied from https://godoc.org/k8s.io/kubernetes/pkg/scheduler/api/v1#HostPriority
type HostPriority struct {
	// Name of the host
	Host string `json:"host"`
	// Score associated with the host
	Score int `json:"score"`
}

// HostPriorityList is copied from https://godoc.org/k8s.io/kubernetes/pkg/scheduler/api/v1#HostPriorityList
type HostPriorityList []HostPriority

// ExtenderFilterResult is copied from https://godoc.org/k8s.io/kubernetes/pkg/scheduler/api/v1#ExtenderFilterResult
type ExtenderFilterResult struct {
	// Filtered set of nodes where the pod can be scheduled; to be populated
	// only if ExtenderConfig.NodeCacheCapable == false
	Nodes *apiv1.NodeList `json:"nodes,omitempty"`
	// Filtered set of nodes where the pod can be scheduled; to be populated
	// only if ExtenderConfig.NodeCacheCapable == true
	NodeNames *[]string `json:"nodenames,omitempty"`
	// Filtered out nodes where the pod can't be scheduled and the failure messages
	FailedNodes FailedNodesMap `json:"failedNodes,omitempty"`
	// Error message indicating failure
	Error string `json:"error,omitempty"`
}

// FailedNodesMap is copied from https://godoc.org/k8s.io/kubernetes/pkg/scheduler/api/v1#FailedNodesMap
type FailedNodesMap map[string]string
