package controller

import (
	"fmt"
	"time"

	internalController "github.com/topolvm/topolvm/internal/controller"
	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultLVMOperationTimeout is how long each lvmd call made by the
// LogicalVolume reconciler may take unless WithLVMOperationTimeout is given.
const DefaultLVMOperationTimeout = internalController.DefaultLVMOperationTimeout

// MinLVMOperationTimeout is the shortest timeout allowed, other than zero.
// A shorter one could stop LVM commands that are still waiting for udev to
// finish its work: systemd-udevd allows each event 180s by default.
const MinLVMOperationTimeout = DefaultLVMOperationTimeout

// LogicalVolumeReconcilerOption configures the LogicalVolume reconciler.
type LogicalVolumeReconcilerOption = internalController.LogicalVolumeReconcilerOption

// WithLVMOperationTimeout sets how long each lvmd call may take before it is
// cancelled. Zero disables the timeout. Any other value must be at least
// MinLVMOperationTimeout, or SetupLogicalVolumeReconcilerWithServices fails.
func WithLVMOperationTimeout(timeout time.Duration) LogicalVolumeReconcilerOption {
	return internalController.WithLVMOperationTimeout(timeout)
}

// ValidateLVMOperationTimeout returns an error unless timeout is zero, which
// disables it, or at least MinLVMOperationTimeout.
func ValidateLVMOperationTimeout(timeout time.Duration) error {
	if timeout == 0 || timeout >= MinLVMOperationTimeout {
		return nil
	}
	return fmt.Errorf("LVM operation timeout %s is not allowed: use 0 to disable it, or at least %s, "+
		"so that LVM commands still waiting for udev are not stopped", timeout, MinLVMOperationTimeout)
}

// SetupLogicalVolumeReconcilerWithServices creates LogicalVolumeReconciler and sets up with manager.
func SetupLogicalVolumeReconcilerWithServices(
	mgr ctrl.Manager,
	client client.Client,
	nodeName string,
	vgService proto.VGServiceClient,
	lvService proto.LVServiceClient,
	opts ...LogicalVolumeReconcilerOption,
) error {
	reconciler := internalController.NewLogicalVolumeReconcilerWithServices(
		client, nodeName, vgService, lvService, opts...)
	if err := ValidateLVMOperationTimeout(reconciler.LVMOperationTimeout()); err != nil {
		return err
	}
	return reconciler.SetupWithManager(mgr)
}
