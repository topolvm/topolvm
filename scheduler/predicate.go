package scheduler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/cybozu-go/topolvm"
	corev1 "k8s.io/api/core/v1"
)

func filterNodes(nodes corev1.NodeList, requested map[string]int64) ExtenderFilterResult {
	if len(requested) <= 0 {
		return ExtenderFilterResult{
			Nodes: &nodes,
		}
	}

	filtered := corev1.NodeList{}
	failed := FailedNodesMap{}

NODE_LOOP:
	for _, node := range nodes.Items {
		for vg, required := range requested {
			val, ok := node.Annotations[topolvm.CapacityKey+vg]
			if !ok {
				failed[node.Name] = "no capacity annotation"
				continue NODE_LOOP
			}
			capacity, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				failed[node.Name] = "bad capacity annotation: " + val
				continue NODE_LOOP
			}
			if capacity < uint64(required) {
				failed[node.Name] = "out of VG free space"
				continue NODE_LOOP
			}
		}
		filtered.Items = append(filtered.Items, node)
	}
	return ExtenderFilterResult{
		Nodes:       &filtered,
		FailedNodes: failed,
	}
}

func extractRequestedSize(pod *corev1.Pod) map[string]int64 {
	result := make(map[string]int64)
	for _, container := range pod.Spec.Containers {
		for k, v := range container.Resources.Limits {
			key := string(k)
			if !strings.HasPrefix(key, topolvm.CapacityKey) {
				continue
			}
			vg := key[len(topolvm.CapacityKey):]
			if _, ok := result[vg]; !ok {
				result[vg] = v.Value()
			}
		}
		for k, v := range container.Resources.Requests {
			key := string(k)
			if !strings.HasPrefix(key, topolvm.CapacityKey) {
				continue
			}
			vg := key[len(topolvm.CapacityKey):]
			if _, ok := result[vg]; !ok {
				result[vg] = v.Value()
			}
		}
	}

	return result
}

func (s scheduler) predicate(w http.ResponseWriter, r *http.Request) {
	var input ExtenderArgs

	reader := http.MaxBytesReader(w, r.Body, 10<<20)
	err := json.NewDecoder(reader).Decode(&input)
	if err != nil || input.Nodes == nil || input.Pod == nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	requested := extractRequestedSize(input.Pod)
	result := filterNodes(*input.Nodes, requested)
	w.Header().Set("content-type", "application/json")
	json.NewEncoder(w).Encode(result)
}
