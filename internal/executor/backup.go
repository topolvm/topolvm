package executor

import (
	"context"
	"fmt"

	snapshot_api "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/internal/mounter"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	logger = ctrl.Log.WithName("SnapshotExecutor")
)

// SnapshotExecutor handles the creation of snapshot pods that perform online snapshots
// of logical volumes. It copies the spec from the DaemonSet pod template and injects
// a specialized snapshot container.
type SnapshotExecutor struct {
	client        client.Client
	logicalVolume *topolvmv1.LogicalVolume
	mountResponse *mounter.MountResponse
	vsContent     *snapshot_api.VolumeSnapshotContent
	vsClass       *snapshot_api.VolumeSnapshotClass

	namespace     string
	targetPVCInfo types.NamespacedName
}

// NewSnapshotBackupExecutor creates a new SnapshotExecutor instance with the provided dependencies.
func NewSnapshotBackupExecutor(
	client client.Client,
	logicalVolume *topolvmv1.LogicalVolume,
	resp *mounter.MountResponse,
	vsContent *snapshot_api.VolumeSnapshotContent,
	vsClass *snapshot_api.VolumeSnapshotClass,
) *SnapshotExecutor {
	return &SnapshotExecutor{
		client:        client,
		logicalVolume: logicalVolume,
		mountResponse: resp,
		vsClass:       vsClass,
		vsContent:     vsContent,
	}
}

// Execute creates a snapshot pod that will perform the online snapshot operation.
func (e *SnapshotExecutor) Execute() error {
	objMeta := buildObjectMeta(topolvmv1.OperationBackup, e.logicalVolume)
	hostPod, err := getHostPod(e.client)
	if err != nil {
		return err
	}

	err = e.setTargetedPVCInfo()
	if err != nil {
		return err
	}
	podSpec, err := e.buildPodSpec(hostPod)
	if err != nil {
		return err
	}

	pod := &corev1.Pod{
		ObjectMeta: objMeta,
		Spec:       podSpec,
	}

	err = e.createSnapshotPod(pod)
	return err
}

func (e *SnapshotExecutor) setTargetedPVCInfo() error {
	if e.vsContent.Spec.VolumeSnapshotRef.Kind != "VolumeSnapshot" ||
		e.vsContent.Spec.VolumeSnapshotRef.Name == "" ||
		e.vsContent.Spec.VolumeSnapshotRef.Namespace == "" {
		return fmt.Errorf("invalid VolumeSnapshotRef: %v", e.vsContent.Spec.VolumeSnapshotRef)
	}

	vs := snapshot_api.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      e.vsContent.Spec.VolumeSnapshotRef.Name,
			Namespace: e.vsContent.Spec.VolumeSnapshotRef.Namespace,
		},
	}
	if err := e.client.Get(context.Background(), client.ObjectKeyFromObject(&vs), &vs); err != nil {
		return fmt.Errorf("failed to get VolumeSnapshot %s/%s: %w", vs.Namespace, vs.Name, err)
	}

	if vs.Spec.Source.PersistentVolumeClaimName == nil {
		return fmt.Errorf("invalid VolumeSnapshotRef: %v", e.vsContent.Spec.VolumeSnapshotRef)
	}
	e.targetPVCInfo = types.NamespacedName{
		Namespace: vs.Namespace,
		Name:      *vs.Spec.Source.PersistentVolumeClaimName,
	}
	return nil
}

func (e *SnapshotExecutor) createSnapshotPod(pod *corev1.Pod) error {
	existingPod := new(corev1.Pod)
	err := e.client.Get(context.Background(), client.ObjectKeyFromObject(pod), existingPod)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		if err := e.client.Create(context.Background(), pod); err != nil {
			return fmt.Errorf("failed to create Snapshot Ensurer pod: %w", err)
		}
		logger.Info("Created Snapshot Ensurer Pod", "name", pod.Name)
		return nil
	}
	logger.Info("Skipped creating Snapshot Ensurer Pod as it already exists", "name", pod.Name)
	return nil
}

// buildPodSpec constructs the pod spec by copying from the DaemonSet's pod template
// (via owner reference) and replacing containers with the snapshot container.
// The affinity is taken from the actual running pod, not the DaemonSet template.
func (e *SnapshotExecutor) buildPodSpec(hostPod *corev1.Pod) (corev1.PodSpec, error) {
	daemonSet, err := getDaemonSetFromOwnerRef(e.client, hostPod)
	if err != nil {
		return corev1.PodSpec{}, err
	}

	var templateContainer corev1.Container
	if len(daemonSet.Spec.Template.Spec.Containers) > 0 {
		templateContainer = daemonSet.Spec.Template.Spec.Containers[0]
	}

	snapshotContainer := e.buildSnapshotContainer(&templateContainer)
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

// getDaemonSetFromOwnerRef retrieves the DaemonSet from the pod's owner reference.
func getDaemonSetFromOwnerRef(rClient client.Client, pod *corev1.Pod) (*appsv1.DaemonSet, error) {
	// Find the DaemonSet owner reference
	var daemonSetRef *metav1.OwnerReference
	for i := range pod.OwnerReferences {
		if pod.OwnerReferences[i].Kind == "DaemonSet" && pod.OwnerReferences[i].APIVersion == "apps/v1" {
			daemonSetRef = &pod.OwnerReferences[i]
			break
		}
	}

	if daemonSetRef == nil {
		return nil, fmt.Errorf("pod %s/%s does not have a DaemonSet owner reference", pod.Namespace, pod.Name)
	}

	// Fetch the DaemonSet
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      daemonSetRef.Name,
			Namespace: pod.Namespace,
		},
	}

	if err := rClient.Get(context.Background(), client.ObjectKeyFromObject(daemonSet), daemonSet); err != nil {
		return nil, fmt.Errorf("failed to get DaemonSet %s/%s: %w", pod.Namespace, daemonSetRef.Name, err)
	}

	return daemonSet, nil
}

// buildSnapshotContainer creates a container configured to execute the online snapshot command.
// It uses the image and configuration from the DaemonSet template container.
func (e *SnapshotExecutor) buildSnapshotContainer(templateContainer *corev1.Container) corev1.Container {
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
			BackupCommandName, // "backup" subcommand
		},
		Args:            e.buildSnapshotArgs(),
		Env:             e.buildSnapshotEnv(templateContainer),
		VolumeMounts:    volumeMounts,
		SecurityContext: e.buildSecurityContext(),
		Resources:       e.buildResourceRequirements(),
	}

	return container
}

// buildSnapshotArgs constructs the command-line arguments for the snapshot command.
func (e *SnapshotExecutor) buildSnapshotArgs() []string {
	defaultNamespaceIfEmpty := func(name string) string {
		if name == "" {
			return e.namespace
		}
		return name
	}
	return []string{
		fmt.Sprintf("--lv-name=%s", e.logicalVolume.Name),
		fmt.Sprintf("--node-name=%s", e.logicalVolume.Spec.NodeName),
		fmt.Sprintf("--mount-path=%s", SnapshotData),
		fmt.Sprintf("--targeted-pvc-namespace=%s", e.targetPVCInfo.Namespace),
		fmt.Sprintf("--targeted-pvc-name=%s", e.targetPVCInfo.Name),
		fmt.Sprintf("--snapshot-storage-name=%s", defaultNamespaceIfEmpty(e.vsClass.Parameters[SnapshotStorageName])),
		fmt.Sprintf("--snapshot-storage-namespace=%s", defaultNamespaceIfEmpty(e.vsClass.Parameters[SnapshotStorageNamespace])),
	}
}

// buildSnapshotEnv constructs the environment variables for the snapshot container.
func (e *SnapshotExecutor) buildSnapshotEnv(templateContainer *corev1.Container) []corev1.EnvVar {
	var env []corev1.EnvVar
	return env
}

// buildSecurityContext creates an appropriate security context for the snapshot container.
func (e *SnapshotExecutor) buildSecurityContext() *corev1.SecurityContext {
	privileged := true
	return &corev1.SecurityContext{
		Privileged: &privileged,
		Capabilities: &corev1.Capabilities{
			Add: []corev1.Capability{
				"SYS_ADMIN",
			},
		},
	}
}

// buildResourceRequirements defines resource requests and limits for the snapshot container.
// Uses reasonable defaults for snapshot operations.
func (e *SnapshotExecutor) buildResourceRequirements() corev1.ResourceRequirements {
	requests := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("100m"),
		corev1.ResourceMemory: resource.MustParse("128Mi"),
	}
	limits := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("1"),
		corev1.ResourceMemory: resource.MustParse("512Mi"),
	}
	return corev1.ResourceRequirements{
		Requests: requests,
		Limits:   limits,
	}
}
