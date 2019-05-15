package scheduler

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"

	"github.com/cybozu-go/topolvm"
)

func capacityToScore(capacity uint64) int {
	converted := int(math.Log10(float64(capacity >> 30)))
	switch {
	case converted < 0:
		return 0
	case converted > 10:
		return 10
	default:
		return converted
	}
}

func prioritize(w http.ResponseWriter, r *http.Request) {
	var input ExtenderArgs

	reader := http.MaxBytesReader(w, r.Body, 10<<20)
	err := json.NewDecoder(reader).Decode(&input)
	if err != nil {
		http.Error(w, "Bad Request.", http.StatusBadRequest)
		return
	}

	result := make([]HostPriority, len(input.Nodes.Items))

	for i, item := range input.Nodes.Items {
		var score int
		if val, ok := item.Annotations[topolvm.CapacityKey]; ok {
			capacity, _ := strconv.ParseUint(val, 64, 10)
			score = capacityToScore(capacity)
		}
		result[i] = HostPriority{Host: item.Name, Score: score}
	}

	w.Header().Set("content-type", "application/json")
	json.NewEncoder(w).Encode(result)
}
