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

	vgs := make(map[string]struct{})
	for _, container := range pod.Spec.Containers {
		for k := range container.Resources.Requests {
			key := string(k)
			if strings.HasPrefix(key, topolvm.CapacityKey) {
				vgs[key[len(topolvm.CapacityKey):]] = struct{}{}
			}
		}
		for k := range container.Resources.Limits {
			key := string(k)
			if strings.HasPrefix(key, topolvm.CapacityKey) {
				vgs[key[len(topolvm.CapacityKey):]] = struct{}{}
			}
		}
	}
	if len(vgs) == 0 {
		return nil
	}

	for i, item := range nodes {
		var score int
		for vg := range vgs {
			if val, ok := item.Annotations[topolvm.CapacityKey+vg]; ok {
				capacity, _ := strconv.ParseUint(val, 10, 64)
				var divisor float64
				if v, ok := divisors[vg]; ok {
					divisor = v
				} else {
					divisor = defaultDivisor
				}
				score += capacityToScore(capacity, divisor)
			}
		}
		result[i] = HostPriority{Host: item.Name, Score: score}
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
