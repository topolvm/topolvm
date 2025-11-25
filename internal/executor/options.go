package executor

//
//import (
//	"fmt"
//
//	topolvmv1 "github.com/topolvm/topolvm/api/v1"
//	corev1 "k8s.io/api/core/v1"
//	"k8s.io/apimachinery/pkg/api/resource"
//	"sigs.k8s.io/controller-runtime/pkg/client"
//)
//
//// SnapshotExecutorOption is a functional option for configuring SnapshotExecutor.
//type SnapshotExecutorOption func(*SnapshotExecutor)
//
//// WithSnapshotImage sets a custom snapshot image for the executor.
//func WithSnapshotImage(image string) SnapshotExecutorOption {
//	return func(e *SnapshotExecutor) {
//		e.snapshotImage = image
//	}
//}
//
//// WithResourceRequirements sets custom resource requirements for the snapshot container.
//func WithResourceRequirements(requests, limits corev1.ResourceList) SnapshotExecutorOption {
//	return func(e *SnapshotExecutor) {
//		e.resourceRequests = requests
//		e.resourceLimits = limits
//	}
//}
//
//// WithSecurityContext sets a custom security context for the snapshot container.
//func WithSecurityContext(sc *corev1.SecurityContext) SnapshotExecutorOption {
//	return func(e *SnapshotExecutor) {
//		e.securityContext = sc
//	}
//}
//
//// WithCustomLabels adds custom labels to the snapshot pod.
//func WithCustomLabels(labels map[string]string) SnapshotExecutorOption {
//	return func(e *SnapshotExecutor) {
//		if e.customLabels == nil {
//			e.customLabels = make(map[string]string)
//		}
//		for k, v := range labels {
//			e.customLabels[k] = v
//		}
//	}
//}
//
//// WithCustomAnnotations adds custom annotations to the snapshot pod.
//func WithCustomAnnotations(annotations map[string]string) SnapshotExecutorOption {
//	return func(e *SnapshotExecutor) {
//		if e.customAnnotations == nil {
//			e.customAnnotations = make(map[string]string)
//		}
//		for k, v := range annotations {
//			e.customAnnotations[k] = v
//		}
//	}
//}
//
//// NewSnapshotExecutorWithOptions creates a new SnapshotExecutor with functional options.
//func NewSnapshotExecutorWithOptions(
//	client client.client,
//	logicalVolume topolvmv1.LogicalVolume,
//	opts ...SnapshotExecutorOption,
//) *SnapshotExecutor {
//	executor := NewSnapshotBackupExecutor(client, logicalVolume)
//
//	for _, opt := range opts {
//		opt(executor)
//	}
//
//	return executor
//}
//
//// Validate checks if the SnapshotExecutor has all required fields properly set.
//func (e *SnapshotExecutor) Validate() error {
//	if e.client == nil {
//		return fmt.Errorf("client is required")
//	}
//
//	if e.logicalVolume.Name == "" {
//		return fmt.Errorf("logical volume name is required")
//	}
//
//	if e.logicalVolume.Spec.NodeName == "" {
//		return fmt.Errorf("logical volume node name is required")
//	}
//
//	if e.logicalVolume.Spec.Size.IsZero() {
//		return fmt.Errorf("logical volume size must be greater than zero")
//	}
//
//	if e.snapshotImage == "" {
//		return fmt.Errorf("snapshot image is required")
//	}
//
//	return nil
//}
//
//// GetDefaultResourceRequirements returns the default resource requirements for snapshot containers.
//func GetDefaultResourceRequirements() (corev1.ResourceList, corev1.ResourceList) {
//	requests := corev1.ResourceList{
//		corev1.ResourceCPU:    resource.MustParse("100m"),
//		corev1.ResourceMemory: resource.MustParse("128Mi"),
//	}
//
//	limits := corev1.ResourceList{
//		corev1.ResourceCPU:    resource.MustParse("1"),
//		corev1.ResourceMemory: resource.MustParse("512Mi"),
//	}
//
//	return requests, limits
//}
//
//// GetDefaultSecurityContext returns the default security context for snapshot containers.
//func GetDefaultSecurityContext() *corev1.SecurityContext {
//	privileged := true
//	return &corev1.SecurityContext{
//		Privileged: &privileged,
//		Capabilities: &corev1.Capabilities{
//			Add: []corev1.Capability{
//				"SYS_ADMIN",
//			},
//		},
//	}
//}
