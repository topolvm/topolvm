package scheduler

import (
	"encoding/json"
	"net/http"
)

func predicate(w http.ResponseWriter, r *http.Request) {
	var input ExtenderArgs

	reader := http.MaxBytesReader(w, r.Body, 10<<20)
	err := json.NewDecoder(reader).Decode(&input)
	if err != nil {
		http.Error(w, "Bad Request.", http.StatusBadRequest)
		return
	}

	filteredNodes := input.Nodes
	filteredNodeNames := input.NodeNames
	failedNodesMap := FailedNodesMap{}

	result := ExtenderFilterResult{
		Nodes:       filteredNodes,
		NodeNames:   filteredNodeNames,
		FailedNodes: failedNodesMap,
	}

	w.Header().Set("content-type", "application/json")
	json.NewEncoder(w).Encode(result)
}
