package runners

import (
	"context"
	"regexp"
	"time"

	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	orphanedVolumeCleanupInterval = 10 * time.Minute
	orphanedVolumeListPageSize    = 500
)

// pvcUIDSuffix matches the PVC UID that external-provisioner appends to the
// volume name it passes to CreateVolume, such as "pvc-<UID>".
var pvcUIDSuffix = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

//+kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=get;list
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list
//+kubebuilder:rbac:groups=topolvm.io,resources=logicalvolumes,verbs=get;list;watch;delete

type orphanedVolumeCleaner struct {
	client      client.Client
	apiReader   client.Reader
	gracePeriod time.Duration
	now         func() time.Time
}

var _ manager.LeaderElectionRunnable = &orphanedVolumeCleaner{}

// NewOrphanedVolumeCleaner returns a manager.Runnable that periodically deletes
// LogicalVolumes that were provisioned on a node, are older than gracePeriod,
// and have neither a PersistentVolume nor a PersistentVolumeClaim.
//
// external-provisioner can leave such LogicalVolumes behind when a PVC is
// deleted while CreateVolume is in progress and the provisioner restarts.
// See https://github.com/kubernetes-csi/external-provisioner/issues/486.
func NewOrphanedVolumeCleaner(c client.Client, apiReader client.Reader, gracePeriod time.Duration) manager.Runnable {
	return &orphanedVolumeCleaner{
		client:      c,
		apiReader:   apiReader,
		gracePeriod: gracePeriod,
		now:         time.Now,
	}
}

// Start implements controller-runtime's manager.Runnable.
func (c *orphanedVolumeCleaner) Start(ctx context.Context) error {
	logger := ctrl.Log.WithName("orphaned-volume-cleaner")
	tick := time.NewTicker(orphanedVolumeCleanupInterval)
	defer tick.Stop()

	for {
		if err := c.cleanup(ctx); err != nil {
			logger.Error(err, "failed to clean up orphaned LogicalVolumes")
		}
		select {
		case <-tick.C:
		case <-ctx.Done():
			return nil
		}
	}
}

// NeedLeaderElection implements controller-runtime's manager.LeaderElectionRunnable.
func (c *orphanedVolumeCleaner) NeedLeaderElection() bool {
	return true
}

func (c *orphanedVolumeCleaner) cleanup(ctx context.Context) error {
	logger := ctrl.Log.WithName("orphaned-volume-cleaner")

	lvs := new(topolvmv1.LogicalVolumeList)
	if err := c.client.List(ctx, lvs); err != nil {
		return err
	}
	pvHandles, err := c.listPVHandles(ctx)
	if err != nil {
		return err
	}
	pvcUIDs, err := c.listPVCUIDs(ctx)
	if err != nil {
		return err
	}

	now := c.now()
	for i := range lvs.Items {
		lv := &lvs.Items[i]
		if !c.isOrphaned(lv, pvHandles, pvcUIDs, now) {
			continue
		}
		logger.Info("deleting orphaned LogicalVolume",
			"name", lv.Name,
			"volume_id", lv.Status.VolumeID,
			"node", lv.Spec.NodeName,
			"created", lv.CreationTimestamp.Time)
		if err := c.client.Delete(ctx, lv); err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "failed to delete orphaned LogicalVolume", "name", lv.Name)
		}
	}
	return nil
}

func (c *orphanedVolumeCleaner) isOrphaned(lv *topolvmv1.LogicalVolume, pvHandles, pvcUIDs map[string]struct{}, now time.Time) bool {
	// Skip volumes that are being deleted or were never created on a node.
	if lv.DeletionTimestamp != nil || lv.Status.VolumeID == "" {
		return false
	}
	// Skip snapshots and clones, which reference a source volume.
	if lv.Spec.Source != "" {
		return false
	}
	if now.Sub(lv.CreationTimestamp.Time) < c.gracePeriod {
		return false
	}
	if _, ok := pvHandles[lv.Status.VolumeID]; ok {
		return false
	}
	// Only delete when the PVC can be identified from the volume name and is
	// gone. Without a recognizable UID, keep the volume.
	uid := pvcUIDSuffix.FindString(lv.Spec.Name)
	if uid == "" {
		return false
	}
	_, ok := pvcUIDs[uid]
	return !ok
}

func (c *orphanedVolumeCleaner) listPVHandles(ctx context.Context) (map[string]struct{}, error) {
	handles := make(map[string]struct{})
	continueToken := ""
	for {
		pvs := new(corev1.PersistentVolumeList)
		err := c.apiReader.List(ctx, pvs, client.Limit(orphanedVolumeListPageSize), client.Continue(continueToken))
		if err != nil {
			return nil, err
		}
		for i := range pvs.Items {
			csi := pvs.Items[i].Spec.CSI
			if csi != nil && csi.Driver == topolvm.GetPluginName() {
				handles[csi.VolumeHandle] = struct{}{}
			}
		}
		continueToken = pvs.Continue
		if continueToken == "" {
			return handles, nil
		}
	}
}

func (c *orphanedVolumeCleaner) listPVCUIDs(ctx context.Context) (map[string]struct{}, error) {
	uids := make(map[string]struct{})
	continueToken := ""
	for {
		pvcs := new(metav1.PartialObjectMetadataList)
		pvcs.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("PersistentVolumeClaimList"))
		err := c.apiReader.List(ctx, pvcs, client.Limit(orphanedVolumeListPageSize), client.Continue(continueToken))
		if err != nil {
			return nil, err
		}
		for i := range pvcs.Items {
			uids[string(pvcs.Items[i].UID)] = struct{}{}
		}
		continueToken = pvcs.Continue
		if continueToken == "" {
			return uids, nil
		}
	}
}
