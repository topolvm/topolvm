package scheduler

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testNode(name string, cap1Gb, cap2Gb, cap3Gb int64) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				topolvm.GetCapacityKeyPrefix() + "dc1": fmt.Sprintf("%d", cap1Gb<<30),
				topolvm.GetCapacityKeyPrefix() + "dc2": fmt.Sprintf("%d", cap2Gb<<30),
				topolvm.GetCapacityKeyPrefix() + "dc3": fmt.Sprintf("%d", cap3Gb<<30),
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
					testNode("10.1.1.1", 5, 10, 10),
					testNode("10.1.1.2", 1, 10, 10),
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "10.1.1.3",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "10.1.1.4",
							Annotations: map[string]string{
								topolvm.GetCapacityKeyPrefix() + "dc1": "foo",
							},
						},
					},
				},
			},
			requested: map[string]int64{
				"dc1": 2 << 30,
			},
			expect: ExtenderFilterResult{
				Nodes: &corev1.NodeList{
					Items: []corev1.Node{
						testNode("10.1.1.1", 5, 10, 10),
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
					testNode("10.1.1.1", 10, 20, 30),
					testNode("10.1.1.2", 1, 5, 10),
					testNode("10.1.1.3", 100, 5, 100),
				},
			},
			requested: map[string]int64{
				"dc1": 5 << 30,
				"dc2": 10 << 30,
				"dc3": 20 << 30,
			},
			expect: ExtenderFilterResult{
				Nodes: &corev1.NodeList{
					Items: []corev1.Node{
						testNode("10.1.1.1", 5, 10, 10),
					},
				},
				FailedNodes: FailedNodesMap{
					"10.1.1.2": "out of VG free space",
					"10.1.1.3": "out of VG free space",
				},
			},
		},
		{
			nodes: corev1.NodeList{
				Items: []corev1.Node{
					testNode("10.1.1.1", 5, 10, 10),
				},
			},
			requested: map[string]int64{
				"dc1": 0,
			},
			expect: ExtenderFilterResult{
				Nodes: &corev1.NodeList{
					Items: []corev1.Node{
						testNode("10.1.1.1", 5, 10, 10),
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
		expected map[string]int64
	}{
		{
			input: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						topolvm.GetCapacityKeyPrefix() + "dc1": strconv.Itoa(5 << 30),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									topolvm.GetCapacityResource(): *resource.NewQuantity(1, resource.BinarySI),
								},
								Limits: corev1.ResourceList{
									topolvm.GetCapacityResource(): *resource.NewQuantity(1, resource.BinarySI),
								},
							},
						},
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									topolvm.GetCapacityResource(): *resource.NewQuantity(1, resource.BinarySI),
								},
								Limits: corev1.ResourceList{
									topolvm.GetCapacityResource(): *resource.NewQuantity(1, resource.BinarySI),
								},
							},
						},
					},
				},
			},
			expected: map[string]int64{
				"dc1": 5 << 30,
			},
		},
		{
			input: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						topolvm.GetCapacityKeyPrefix() + "dc1": strconv.Itoa(3 << 30),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									topolvm.GetCapacityResource(): *resource.NewQuantity(1, resource.BinarySI),
								},
							},
						},
					},
				},
			},
			expected: map[string]int64{
				"dc1": 3 << 30,
			},
		},
	}

	for _, tt := range testCases {
		result := extractRequestedSize(tt.input)
		for vg, cap := range tt.expected {
			if v, ok := result[vg]; ok {
				if v != cap {
					t.Errorf("expected extractRequestedSize() to be %d, but actual %d", cap, v)
				}
			} else {
				t.Errorf("clould not find target: %v", result)
			}
		}
	}
}
