package scheduler

import (
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
		{1, 1, 0},
		{128 << 30, 1, 7},
		{128 << 30, 2, 6},
		{128 << 30, 0.5, 8},
		{^uint64(0), 1, 10},
	}

	for _, tt := range testCases {
		score := capacityToScore(tt.input, tt.divisor)
		if score != tt.expect {
			t.Errorf("score incorrect: input=%d expect=%d actual=%d",
				tt.input,
				tt.expect,
				score,
			)
		}
	}
}

func TestScoreNodes(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				topolvm.GetCapacityKeyPrefix() + "ssd":  "64",
				topolvm.GetCapacityKeyPrefix() + "hdd1": "64",
				topolvm.GetCapacityKeyPrefix() + "hdd2": "64",
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
					topolvm.GetCapacityKeyPrefix() + "ssd": "foo",
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
		"ssd":  4,
		"hdd1": 10,
	}
	result := scoreNodes(pod, input, defaultDivisor, divisors)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected scoreNodes() to be %#v, but actual %#v", expected, result)
	}
}
