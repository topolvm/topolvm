package scheduler

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/cybozu-go/topolvm"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testNode(name string, capGb int64) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				topolvm.CapacityKey: fmt.Sprintf("%d", capGb<<30),
			},
		},
	}
}

func TestFilterNodes(t *testing.T) {
	testCases := []struct {
		nodes     corev1.NodeList
		requested int64
		spare     uint64
		expect    ExtenderFilterResult
	}{
		{
			nodes: corev1.NodeList{
				Items: []corev1.Node{
					testNode("10.1.1.1", 5),
				},
			},
			requested: 1073741824,
			spare:     1073741824,
			expect: ExtenderFilterResult{
				Nodes: &corev1.NodeList{
					Items: []corev1.Node{
						testNode("10.1.1.1", 5),
					},
				},
				FailedNodes: FailedNodesMap{},
			},
		},
	}

	for _, tt := range testCases {
		result := filterNodes(tt.nodes, tt.requested, tt.spare)
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
