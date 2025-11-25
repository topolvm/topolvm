package executor

import (
	"context"
	"fmt"
	"os"
	"strings"

	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func buildObjectMeta(operation topolvmv1.OperationType, lv *topolvmv1.LogicalVolume) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        fmt.Sprintf("snapshot-%s-%s", strings.ToLower(string(operation)), lv.Name),
		Namespace:   getNamespace(),
		Labels:      buildLabels(lv),
		Annotations: buildAnnotations(lv),
		OwnerReferences: []metav1.OwnerReference{
			*metav1.NewControllerRef(lv, topolvmv1.GroupVersion.WithKind("LogicalVolume")),
		},
	}
}

func getNamespace() string {
	namespace := os.Getenv(EnvHostNamespace)
	if namespace == "" {
		namespace = "topolvm-system"
	}
	return namespace
}

func buildLabels(lv *topolvmv1.LogicalVolume) map[string]string {
	labels := map[string]string{
		LabelAppKey:           LabelAppValue,
		LabelLogicalVolumeKey: lv.Name,
		LabelSnapshotPodKey:   "true",
	}

	return labels
}

// buildAnnotations constructs annotations for the snapshot pod.
func buildAnnotations(lv *topolvmv1.LogicalVolume) map[string]string {
	annotations := map[string]string{
		"topolvm.io/snapshot-source":  lv.Spec.Source,
		"topolvm.io/device-class":     lv.Spec.DeviceClass,
		"topolvm.io/snapshot-version": "v1",
	}
	return annotations
}

func getHostPod(rClient client.Client) (*corev1.Pod, error) {
	hostPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      os.Getenv(EnvHostName),
			Namespace: os.Getenv(EnvHostNamespace),
		},
	}

	if err := rClient.Get(context.Background(), client.ObjectKeyFromObject(hostPod), hostPod); err != nil {
		return nil, fmt.Errorf("failed to get host pod: %w", err)
	}

	return hostPod, nil
}
