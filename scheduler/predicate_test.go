package scheduler

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/cybozu-go/topolvm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testNode(name string, capGb int64) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				topolvm.CapacityKey + "myvg1": fmt.Sprintf("%d", capGb<<30),
			},
		},
	}
}

func TestFilterNodes(t *testing.T) {
	testCases := []struct {
		nodes     corev1.NodeList
		requested map[string]int64
		expect    ExtenderFilterResult
	}{
		{
			nodes: corev1.NodeList{
				Items: []corev1.Node{
					testNode("10.1.1.1", 5),
					testNode("10.1.1.2", 1),
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "10.1.1.3",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "10.1.1.4",
							Annotations: map[string]string{
								topolvm.CapacityKey + "myvg1": "foo",
							},
						},
					},
				},
			},
			requested: map[string]int64{
				"myvg1": 2 << 30,
			},
			expect: ExtenderFilterResult{
				Nodes: &corev1.NodeList{
					Items: []corev1.Node{
						testNode("10.1.1.1", 5),
					},
				},
				FailedNodes: FailedNodesMap{
					"10.1.1.2": "out of VG free space",
					"10.1.1.3": "no capacity annotation",
					"10.1.1.4": "bad capacity annotation: foo",
				},
			},
		},
		{
			nodes: corev1.NodeList{
				Items: []corev1.Node{
					testNode("10.1.1.1", 5),
				},
			},
			requested: map[string]int64{
				"myvg1": 0,
			},
			expect: ExtenderFilterResult{
				Nodes: &corev1.NodeList{
					Items: []corev1.Node{
						testNode("10.1.1.1", 5),
					},
				},
				FailedNodes: map[string]string{},
			},
		},
	}

	for _, tt := range testCases {
		result := filterNodes(tt.nodes, tt.requested)
		if len(result.Nodes.Items) != len(tt.expect.Nodes.Items) {
			t.Fatalf("not match length of filtered NodeList: expect=%d actual=%d", len(tt.expect.Nodes.Items), len(result.Nodes.Items))
		}

		for i, n := range result.Nodes.Items {
			if n.Name != tt.expect.Nodes.Items[i].Name {
				t.Errorf("not match node name: expect=%s actual=%s", tt.expect.Nodes.Items[i].Name, n.Name)
			}
		}

		if !reflect.DeepEqual(result.FailedNodes, tt.expect.FailedNodes) {
			t.Errorf("not match FailedNodes: expect=%v actual=%v", tt.expect.FailedNodes, result.FailedNodes)
		}
	}
}

func TestExtractRequestedSize(t *testing.T) {
	testCases := []struct {
		input    *corev1.Pod
		expected int64
	}{
		{
			input: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									topolvm.CapacityResource("myvg1"): *resource.NewQuantity(5<<30, resource.BinarySI),
								},
								Requests: corev1.ResourceList{
									topolvm.CapacityResource("myvg1"): *resource.NewQuantity(3<<30, resource.BinarySI),
								},
							},
						},
						{
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									topolvm.CapacityResource("myvg1"): *resource.NewQuantity(2<<30, resource.BinarySI),
								},
								Requests: corev1.ResourceList{
									topolvm.CapacityResource("myvg1"): *resource.NewQuantity(1<<30, resource.BinarySI),
								},
							},
						},
					},
				},
			},
			expected: 5 << 30,
		},
		{
			input: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									topolvm.CapacityResource("myvg1"): *resource.NewQuantity(3<<30, resource.BinarySI),
								},
							},
						},
					},
				},
			},
			expected: 3 << 30,
		},
	}

	for _, tt := range testCases {
		result := extractRequestedSize(tt.input)
		if v, ok := result["myvg1"]; ok {
			if v != tt.expected {
				t.Errorf("expected extractRequestedSize() to be %d, but actual %d", tt.expected, v)
			}
		} else {
			t.Errorf("clould not find target: %v", result)
		}
	}
}
