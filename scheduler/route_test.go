package scheduler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"

	"github.com/cybozu-go/topolvm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var extenderArgs = ExtenderArgs{
	Pod: &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				topolvm.CapacityKey + "-myvg1": strconv.Itoa(3 << 30),
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							topolvm.CapacityResource: *resource.NewQuantity(1, resource.BinarySI),
						},
					},
				},
			},
		},
	},
	Nodes: &corev1.NodeList{
		Items: []corev1.Node{
			testNode("10.1.1.1", 2),
			testNode("10.1.1.2", 5),
		},
	},
}

func testPredicate(t *testing.T) {
	t.Parallel()

	handler, err := NewHandler(1)
	if err != nil {
		t.Fatal(err)
	}

	input, err := json.Marshal(extenderArgs)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/predicate", bytes.NewReader(input))
	handler.ServeHTTP(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Error("resp.StatusCode != http.StatusOK:", resp.StatusCode)
	}

	result := new(ExtenderFilterResult)
	err = json.NewDecoder(resp.Body).Decode(result)
	if err != nil {
		t.Fatal(err)
	}

	if result.Nodes == nil || len(result.Nodes.Items) != 1 || result.Nodes.Items[0].Name != "10.1.1.2" {
		t.Errorf("wrong result.Nodes: %#v", result.Nodes)
	}
	if _, ok := result.FailedNodes["10.1.1.1"]; !ok {
		t.Error("result.FailedNodes does not contain 10.1.1.1")
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/predicate", nil)
	handler.ServeHTTP(w, r)

	resp = w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Error("resp.StatusCode != http.StatusBadRequest:", resp.StatusCode)
	}
}

func testPrioritize(t *testing.T) {
	t.Parallel()

	handler, err := NewHandler(1)
	if err != nil {
		t.Fatal(err)
	}

	input, err := json.Marshal(extenderArgs)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/prioritize", bytes.NewReader(input))
	handler.ServeHTTP(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Error("resp.StatusCode != http.StatusOK:", resp.StatusCode)
	}

	result := HostPriorityList{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		t.Fatal(err)
	}

	expected := HostPriorityList{
		{
			Host:  "10.1.1.1",
			Score: 1,
		},
		{
			Host:  "10.1.1.2",
			Score: 2,
		},
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("wrong Hostprioritylist; expected: %#v, actual: %#v", expected, result)
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/prioritize", nil)
	handler.ServeHTTP(w, r)

	resp = w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Error("resp.StatusCode != http.StatusBadRequest:", resp.StatusCode)
	}
}

func TestRoute(t *testing.T) {
	t.Run("predicate", testPredicate)
	t.Run("prioritize", testPrioritize)
}
