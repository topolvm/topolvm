package v2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	. "github.com/onsi/gomega"

	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ContentMode string

const (
	ContentModeFile  ContentMode = "file"
	ContentModeBlock ContentMode = "block"
)

// PodRunner is a helper to run commands in pods via remote execution. similar to kubectl exec
type PodRunner struct {
	*rest.Config
	kubernetes.Interface
	runtime.ParameterCodec
}

func NewPodRunner(config *rest.Config, sch *runtime.Scheme) (*PodRunner, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	codec := runtime.NewParameterCodec(sch)
	return &PodRunner{Interface: clientset, Config: config, ParameterCodec: codec}, nil
}

// ExecOptions passed to ExecWithOptions
type ExecOptions struct {
	Command       []string
	Namespace     string
	PodName       string
	ContainerName string
	Stdin         io.Reader
	CaptureStdout bool
	CaptureStderr bool
	// If false, whitespace in std{err,out} will be removed.
	PreserveWhitespace bool
}

// ExecWithOptions executes a command in the specified container,
// returning stdout, stderr and error. `options` allowed for
// additional parameters to be passed.
func (t *PodRunner) ExecWithOptions(ctx context.Context, options ExecOptions) (string, string, error) {
	log.FromContext(ctx).Info("exec", "options", options)
	const tty = false

	req := t.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(options.PodName).
		Namespace(options.Namespace).
		SubResource("exec").
		Param("container", options.ContainerName)

	req.VersionedParams(&k8sv1.PodExecOptions{
		Container: options.ContainerName,
		Command:   options.Command,
		Stdin:     options.Stdin != nil,
		Stdout:    options.CaptureStdout,
		Stderr:    options.CaptureStderr,
		TTY:       tty,
	}, t.ParameterCodec)

	var stdout, stderr bytes.Buffer

	exec, err := remotecommand.NewSPDYExecutor(t.Config, http.MethodPost, req.URL())
	if err != nil {
		return "", "", err
	}

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  options.Stdin,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    tty,
	})
	if options.PreserveWhitespace {
		return stdout.String(), stderr.String(), err
	}
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

func (t *PodRunner) ExecCommand(ctx context.Context, pod, namespace, container string, command []string) (string, string, error) {
	return t.ExecWithOptions(ctx, ExecOptions{
		Command:            command,
		Namespace:          namespace,
		PodName:            pod,
		ContainerName:      container,
		Stdin:              nil,
		CaptureStdout:      true,
		CaptureStderr:      true,
		PreserveWhitespace: false,
	})
}

// ExecShellCommand executes the command in container through a shell.
func (t *PodRunner) ExecShellCommand(ctx context.Context, pod, namespace, container string, command string) (string, string, error) {
	stdout, stderr, err := t.ExecCommand(ctx, pod, namespace, container, []string{"sh", "-c", command})
	if err != nil {
		return stdout, stderr, fmt.Errorf("failed to execute shell command in pod %v, container %v: %v",
			pod, container, err)
	}
	return stdout, stderr, nil
}

// ExecCommandInFirstPodContainer executes the command in a given pod in the first container.
func (t *PodRunner) ExecCommandInFirstPodContainer(ctx context.Context, pod *k8sv1.Pod, cmd string) (string, string, error) {
	if len(pod.Spec.Containers) < 1 {
		return "", "", fmt.Errorf("found %v containers, but expected at least 1", pod.Spec.Containers)
	}
	return t.ExecShellCommand(ctx, pod.Name, pod.Namespace, pod.Spec.Containers[0].Name, cmd)
}

// WriteDataInPod writes the data to pod.
func (t *PodRunner) WriteDataInPod(ctx context.Context, pod *k8sv1.Pod, content string, mode ContentMode) error {
	Expect(pod.Spec.Containers).NotTo(BeEmpty())
	var filePath string
	if mode == "file" {
		filePath = pod.Spec.Containers[0].VolumeMounts[0].MountPath + "/test"
	} else {
		filePath = pod.Spec.Containers[0].VolumeDevices[0].DevicePath
	}

	command := fmt.Sprintf("echo \"%s\" >> %s", content, filePath)
	_, stderr, err := t.ExecCommandInFirstPodContainer(ctx, pod, command)
	if err != nil {
		return err
	}
	if len(stderr) != 0 {
		return errors.New(stderr)
	}
	return err
}

// GetDataInPod uses either cat or head to retrieve data from a file or a block-device.
// While cat for files is canonical to read, using head for block devices is a hack that we use to avoid
// having to partition the block device. Instead we read directly from the device. This only works
// as long as the read data is exactly one line, if it is more this code will break.
// For testing purposes this will be fine to verify block integrity though.
func (t *PodRunner) GetDataInPod(ctx context.Context, pod *k8sv1.Pod, mode ContentMode) (string, error) {
	var command string
	if mode == ContentModeFile {
		// if we use a file we can simply get the file content
		command = fmt.Sprintf("cat %s/test", pod.Spec.Containers[0].VolumeMounts[0].MountPath)
	} else {
		// HACK: for a block device, since we don't format, we have to limit the lines, otherwise we will read
		// the entire disk content as bytes, which will be bad. Instead, limit to one line.
		command = fmt.Sprintf("head -n1 %s", pod.Spec.Containers[0].VolumeDevices[0].DevicePath)
	}
	stdout, stderr, err := t.ExecCommandInFirstPodContainer(ctx, pod, command)
	if err != nil {
		return "", err
	}
	if len(stderr) != 0 {
		return "", errors.New(stderr)
	}
	return stdout, err
}

func (t *PodRunner) GetProcMountLineForFirstVolumeMount(ctx context.Context, pod *k8sv1.Pod) (string, error) {
	path := pod.Spec.Containers[0].VolumeMounts[0].MountPath

	_, _, err := t.ExecCommandInFirstPodContainer(ctx, pod, fmt.Sprintf("mountpoint -d %s", path))
	if err != nil {
		return "", fmt.Errorf("could not verify mountpoint for %s: %w", path, err)
	}

	stdout, stderr, err := t.ExecCommandInFirstPodContainer(ctx, pod, fmt.Sprintf("grep %s /proc/mounts", path))
	if err != nil {
		return "", fmt.Errorf("could not find %s in /proc/mounts: %w", path, err)
	}
	if len(stderr) != 0 {
		return "", errors.New(stderr)
	}
	return stdout, nil
}
