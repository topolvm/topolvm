package scheduler

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/cybozu-go/topolvm"
	corev1 "k8s.io/api/core/v1"
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

func scoreNodes(pod *corev1.Pod, nodes []corev1.Node, defaultDivisor float64, divisors map[string]float64) []HostPriority {
	result := make([]HostPriority, len(nodes))

	var dcs []string
	for k := range pod.Annotations {
		if strings.HasPrefix(k, topolvm.CapacityKeyPrefix) {
			dcs = append(dcs, k[len(topolvm.CapacityKeyPrefix):])
		}
	}
	if len(dcs) == 0 {
		return nil
	}

	for i, item := range nodes {
		minScore := 10
		for _, dc := range dcs {
			if val, ok := item.Annotations[topolvm.CapacityKeyPrefix+dc]; ok {
				capacity, _ := strconv.ParseUint(val, 10, 64)
				var divisor float64
				if v, ok := divisors[dc]; ok {
					divisor = v
				} else {
					divisor = defaultDivisor
				}
				score := capacityToScore(capacity, divisor)
				if score < minScore {
					minScore = score
				}
			}
		}
		result[i] = HostPriority{Host: item.Name, Score: minScore}
	}

	return result
}

func (s scheduler) prioritize(w http.ResponseWriter, r *http.Request) {
	var input ExtenderArgs

	reader := http.MaxBytesReader(w, r.Body, 10<<20)
	err := json.NewDecoder(reader).Decode(&input)
	if err != nil {
		http.Error(w, "Bad Request.", http.StatusBadRequest)
		return
	}

	result := scoreNodes(input.Pod, input.Nodes.Items, s.defaultDivisor, s.divisors)

	w.Header().Set("content-type", "application/json")
	json.NewEncoder(w).Encode(result)
}
