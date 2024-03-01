package scheduler

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
)

func capacityToScore(capacity uint64, divisor float64) int {
	gb := capacity >> 30

	// Avoid logarithm of zero, which diverges to negative infinity.
	if gb == 0 {
		// If there is a non-nil capacity but we dont have at least one gigabyte, we score it with one.
		// This is because the capacityToScore precision is at the gigabyte level.
		// TODO: introduce another scheduling algorithm for byte-level precision.
		if capacity > 0 {
			return 1
		}

		return 0
	}

	converted := int(math.Log2(float64(gb) / divisor))
	switch {
	case converted < 1:
		return 1
	case converted > 10:
		return 10
	default:
		return converted
	}
}

func scoreNodes(pod *corev1.Pod, nodes []corev1.Node, defaultDivisor float64, divisors map[string]float64) []HostPriority {
	var dcs []string
	for k := range pod.Annotations {
		if strings.HasPrefix(k, topolvm.GetCapacityKeyPrefix()) {
			dcs = append(dcs, k[len(topolvm.GetCapacityKeyPrefix()):])
		}
	}
	if len(dcs) == 0 {
		return nil
	}

	result := make([]HostPriority, len(nodes))
	wg := &sync.WaitGroup{}
	wg.Add(len(nodes))
	for i := range nodes {
		r := &result[i]
		item := nodes[i]
		go func() {
			score := scoreNode(item, dcs, defaultDivisor, divisors)
			*r = HostPriority{Host: item.Name, Score: score}
			wg.Done()
		}()
	}
	wg.Wait()

	return result
}

func scoreNode(item corev1.Node, deviceClasses []string, defaultDivisor float64, divisors map[string]float64) int {
	minScore := math.MaxInt32
	for _, dc := range deviceClasses {
		if val, ok := item.Annotations[topolvm.GetCapacityKeyPrefix()+dc]; ok {
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
	if minScore == math.MaxInt32 {
		minScore = 0
	}
	return minScore
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
