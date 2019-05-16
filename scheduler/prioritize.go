package scheduler

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"

	"github.com/cybozu-go/topolvm"
)

func capacityToScore(capacity uint64, divisor float64) int {
	gb := capacity >> 30

	// avoid logarithm of zero, which diverges to negative infinity.
	if gb == 0 {
		return 0
	}

	converted := int(math.Log2(float64(gb) / divisor))
	switch {
	case converted < 0:
		return 0
	case converted > 10:
		return 10
	default:
		return converted
	}
}

func (s scheduler) prioritize(w http.ResponseWriter, r *http.Request) {
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
			capacity, _ := strconv.ParseUint(val, 10, 64)
			score = capacityToScore(capacity, s.divisor)
		}
		result[i] = HostPriority{Host: item.Name, Score: score}
	}

	w.Header().Set("content-type", "application/json")
	json.NewEncoder(w).Encode(result)
}
