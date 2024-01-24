package scheduler

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCapacityToScore(t *testing.T) {
	testCases := []struct {
		input   uint64
		divisor float64
		expect  int
	}{
		{0, 1, 0},
		{1, 1, 1},       // even one byte will lead to a score of at least 1
		{1 << 30, 1, 1}, // converted < 1 but still gets a score of 1 because of the minimum
		{128 << 30, 1, 7},
		{128 << 30, 2, 6},
		{128 << 30, 0.5, 8},
		{^uint64(0), 1, 10},
	}

	for i, tt := range testCases {
		t.Run(fmt.Sprintf("test: %d", i), func(t *testing.T) {
			score := capacityToScore(tt.input, tt.divisor)
			if score != tt.expect {
				t.Errorf("score incorrect: input=%d expect=%d actual=%d",
					tt.input,
					tt.expect,
					score,
				)
			}
		})
	}
}

func TestScoreNodes(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				topolvm.GetCapacityKeyPrefix() + "dc1": "64",
				topolvm.GetCapacityKeyPrefix() + "dc2": "64",
				topolvm.GetCapacityKeyPrefix() + "dc3": "64",
			},
		},
	}
	input := []corev1.Node{
		testNode("10.1.1.1", 128, 128, 128),
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "10.1.1.2",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "10.1.1.3",
				Annotations: map[string]string{
					topolvm.GetCapacityKeyPrefix() + "dc1": "foo",
				},
			},
		},
	}
	expected := []HostPriority{
		{
			Host:  "10.1.1.1",
			Score: 3,
		},
		{
			Host:  "10.1.1.2",
			Score: 0,
		},
		{
			Host:  "10.1.1.3",
			Score: 0,
		},
	}

	defaultDivisor := 2.0
	divisors := map[string]float64{
		"dc1": 4,
		"dc2": 10,
	}
	result := scoreNodes(pod, input, defaultDivisor, divisors)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected scoreNodes() to be %#v, but actual %#v", expected, result)
	}
}
