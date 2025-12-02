package executor

import (
	"context"
	"fmt"

	snapshot_api "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/internal/mounter"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RestoreExecutor handles the creation of restore pods that perform online restores
// of logical volumes from snapshots. It copies the spec from the DaemonSet pod template
// and injects a specialized restore container.
type RestoreExecutor struct {
	client                client.Client
	logicalVolume         *topolvmv1.LogicalVolume
	snapshotLogicalVolume *topolvmv1.LogicalVolume
	mountResponse         *mounter.MountResponse
	vsClass               *snapshot_api.VolumeSnapshotClass

	namespace string
}

// NewSnapshotRestoreExecutor creates a new RestoreExecutor instance with the provided dependencies.
func NewSnapshotRestoreExecutor(
	client client.Client,
	logicalVolume *topolvmv1.LogicalVolume,
	snapshotLogicalVolume *topolvmv1.LogicalVolume,
	resp *mounter.MountResponse,
	vsClass *snapshot_api.VolumeSnapshotClass,
) *RestoreExecutor {
	return &RestoreExecutor{
		client:                client,
		logicalVolume:         logicalVolume,
		snapshotLogicalVolume: snapshotLogicalVolume,
		mountResponse:         resp,
		vsClass:               vsClass,
	}
}

// Execute creates a restore pod that will perform the online restore operation.
func (e *RestoreExecutor) Execute() error {
	objMeta := buildObjectMeta(topolvmv1.OperationRestore, e.logicalVolume)
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

	err = e.createRestorePod(pod)
	return err
}

func (e *RestoreExecutor) buildPodSpec(hostPod *corev1.Pod) (corev1.PodSpec, error) {
	daemonSet, err := getDaemonSetFromOwnerRef(e.client, hostPod)
	if err != nil {
		return corev1.PodSpec{}, err
	}

	var templateContainer corev1.Container
	if len(daemonSet.Spec.Template.Spec.Containers) > 0 {
		templateContainer = daemonSet.Spec.Template.Spec.Containers[0]
	}

	snapshotContainer := e.buildRestoreContainer(&templateContainer)
	// Copy the entire pod spec from DaemonSet template except the volume and container parts
	podSpec := daemonSet.Spec.Template.Spec.DeepCopy()
	podSpec.Containers = []corev1.Container{snapshotContainer}
	podSpec.Volumes = []corev1.Volume{{
		Name: SnapshotData,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: e.mountResponse.MountPath,
			},
		},
	}}
	podSpec.RestartPolicy = corev1.RestartPolicyNever

	// Override affinity from the actual running pod (not from DaemonSet template)
	// This is important because the pod's affinity might be dynamically set by the scheduler
	if hostPod.Spec.Affinity != nil {
		podSpec.Affinity = hostPod.Spec.Affinity.DeepCopy()
	}

	return *podSpec, nil
}

func (e *RestoreExecutor) createRestorePod(pod *corev1.Pod) error {
	existingPod := new(corev1.Pod)
	err := e.client.Get(context.Background(), client.ObjectKeyFromObject(pod), existingPod)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		if err := e.client.Create(context.Background(), pod); err != nil {
			return fmt.Errorf("failed to create Restore Ensurer pod: %w", err)
		}
		logger.Info("Created Restore Ensurer Pod", "name", pod.Name)
		return nil
	}
	logger.Info("Skipped creating Restore Ensurer Pod as it already exists", "name", pod.Name)
	return nil
}

func (e *RestoreExecutor) buildRestoreContainer(templateContainer *corev1.Container) corev1.Container {
	image := templateContainer.Image
	imagePullPolicy := corev1.PullIfNotPresent
	if templateContainer.ImagePullPolicy != "" {
		imagePullPolicy = templateContainer.ImagePullPolicy
	}

	// Get volume mounts from template container
	var volumeMounts []corev1.VolumeMount
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      SnapshotData,
		MountPath: SnapshotData,
	})

	container := corev1.Container{
		Name:            SnapshotContainerName,
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
		Command: []string{
			fmt.Sprintf("/%s", topolvmv1.TopoLVMSnapshotter),
			RestoreCommandName, // "backup" subcommand
		},
		Args:            e.buildRestoreArgs(),
		VolumeMounts:    volumeMounts,
		SecurityContext: e.buildSecurityContext(),
		Resources:       e.buildResourceRequirements(),
	}

	return container
}

func (e *RestoreExecutor) buildRestoreArgs() []string {
	defaultNamespaceIfEmpty := func(name string) string {
		if name == "" {
			return e.namespace
		}
		return name
	}
	return []string{
		fmt.Sprintf("--mount-path=%s", SnapshotData),
		fmt.Sprintf("--lv-name=%s", e.logicalVolume.Name),
		fmt.Sprintf("--node-name=%s", e.logicalVolume.Spec.NodeName),
		fmt.Sprintf("--repository=%s", e.snapshotLogicalVolume.Status.Snapshot.Repository),
		fmt.Sprintf("--snapshot-id=%s", e.snapshotLogicalVolume.Status.Snapshot.SnapshotID),
		fmt.Sprintf("--snapshot-storage-name=%s", defaultNamespaceIfEmpty(e.vsClass.Parameters[SnapshotStorageName])),
		fmt.Sprintf("--snapshot-storage-namespace=%s", defaultNamespaceIfEmpty(e.vsClass.Parameters[SnapshotStorageNamespace])),
	}
}

func (e *RestoreExecutor) buildSecurityContext() *corev1.SecurityContext {
	privileged := true
	return &corev1.SecurityContext{
		Privileged: &privileged,
	}
}

func (e *RestoreExecutor) buildResourceRequirements() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
	}
}
