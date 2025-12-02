package executor

import (
	"context"
	"fmt"

	snapshot_api "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeleteExecutor handles the creation of delete pods that perform snapshot deletion
// operations using restic forget. It copies the spec from the DaemonSet pod template
// and injects a specialized delete container.
type DeleteExecutor struct {
	client        client.Client
	logicalVolume *topolvmv1.LogicalVolume
	vsClass       *snapshot_api.VolumeSnapshotClass

	namespace string
}

// NewSnapshotDeleteExecutor creates a new DeleteExecutor instance with the provided dependencies.
func NewSnapshotDeleteExecutor(
	client client.Client,
	logicalVolume *topolvmv1.LogicalVolume,
	vsClass *snapshot_api.VolumeSnapshotClass,
) *DeleteExecutor {
	return &DeleteExecutor{
		client:        client,
		logicalVolume: logicalVolume,
		vsClass:       vsClass,
	}
}

// Execute creates a delete pod that will perform the snapshot deletion operation.
func (e *DeleteExecutor) Execute() error {
	objMeta := buildObjectMeta(topolvmv1.OperationDelete, e.logicalVolume)
	hostPod, err := getHostPod(e.client)
	if err != nil {
		return err
	}

	e.namespace = hostPod.Namespace
	podSpec, err := e.buildPodSpec(hostPod)
	if err != nil {
		return err
	}

	pod := &corev1.Pod{
		ObjectMeta: objMeta,
		Spec:       podSpec,
	}

	err = e.createDeletePod(pod)
	return err
}

func (e *DeleteExecutor) buildPodSpec(hostPod *corev1.Pod) (corev1.PodSpec, error) {
	daemonSet, err := getDaemonSetFromOwnerRef(e.client, hostPod)
	if err != nil {
		return corev1.PodSpec{}, err
	}

	var templateContainer corev1.Container
	if len(daemonSet.Spec.Template.Spec.Containers) > 0 {
		templateContainer = daemonSet.Spec.Template.Spec.Containers[0]
	}

	deleteContainer := e.buildDeleteContainer(&templateContainer)

	// Copy the entire pod spec from DaemonSet template except the volume and container parts
	podSpec := daemonSet.Spec.Template.Spec.DeepCopy()
	podSpec.Containers = []corev1.Container{deleteContainer}
	// Delete operation doesn't need to mount the volume data
	podSpec.Volumes = []corev1.Volume{}
	podSpec.RestartPolicy = corev1.RestartPolicyNever

	// Override affinity from the actual running pod (not from DaemonSet template)
	// This is important because the pod's affinity might be dynamically set by the scheduler
	if hostPod.Spec.Affinity != nil {
		podSpec.Affinity = hostPod.Spec.Affinity.DeepCopy()
	}

	return *podSpec, nil
}

func (e *DeleteExecutor) createDeletePod(pod *corev1.Pod) error {
	existingPod := new(corev1.Pod)
	err := e.client.Get(context.Background(), client.ObjectKeyFromObject(pod), existingPod)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err := e.client.Create(context.Background(), pod); err != nil {
			return fmt.Errorf("failed to create Delete Executor pod: %w", err)
		}
		logger.Info("Created Delete Executor Pod", "name", pod.Name)
		return nil
	}
	logger.Info("Skipped creating Delete Executor Pod as it already exists", "name", pod.Name)
	return nil
}

func (e *DeleteExecutor) buildDeleteContainer(templateContainer *corev1.Container) corev1.Container {
	image := templateContainer.Image
	imagePullPolicy := corev1.PullIfNotPresent
	if templateContainer.ImagePullPolicy != "" {
		imagePullPolicy = templateContainer.ImagePullPolicy
	}

	container := corev1.Container{
		Name:            DeleteContainerName,
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
		Command: []string{
			fmt.Sprintf("/%s", topolvmv1.TopoLVMSnapshotter),
			DeleteCommandName, // "delete" subcommand
		},
		Args:            e.buildDeleteArgs(),
		VolumeMounts:    []corev1.VolumeMount{}, // No volume mounts needed for delete
		SecurityContext: e.buildSecurityContext(),
		Resources:       e.buildResourceRequirements(),
	}

	return container
}

func (e *DeleteExecutor) buildDeleteArgs() []string {
	defaultNamespaceIfEmpty := func(name string) string {
		if name == "" {
			return e.namespace
		}
		return name
	}

	// Validate that we have snapshot information
	if e.logicalVolume.Status.Snapshot == nil {
		logger.Error(nil, "LogicalVolume has no snapshot status", "lv", e.logicalVolume.Name)
		return []string{}
	}

	args := []string{
		fmt.Sprintf("--lv-name=%s", e.logicalVolume.Name),
		fmt.Sprintf("--repository=%s", e.logicalVolume.Status.Snapshot.Repository),
		fmt.Sprintf("--snapshot-storage-name=%s", defaultNamespaceIfEmpty(e.vsClass.Parameters[SnapshotStorageName])),
		fmt.Sprintf("--snapshot-storage-namespace=%s", defaultNamespaceIfEmpty(e.vsClass.Parameters[SnapshotStorageNamespace])),
	}

	return args
}

func (e *DeleteExecutor) buildSecurityContext() *corev1.SecurityContext {
	privileged := false
	return &corev1.SecurityContext{
		Privileged: &privileged,
	}
}

func (e *DeleteExecutor) buildResourceRequirements() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("50m"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}
}
