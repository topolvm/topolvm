/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sanity

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/container-storage-interface/spec/lib/go/csi"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/klog/v2"
)

// resourceInfo represents a resource (i.e., a volume or a snapshot).
type resourceInfo struct {
	id   string
	data interface{}
}

// volumeInfo keeps track of the information needed to delete a volume.
type volumeInfo struct {
	// Node on which the volume was published, empty if none
	// or publishing is not supported.
	NodeID string
}

// snapshotInfo keeps track of the information needed to delete a snapshot.
type snapshotInfo struct{}

// Resources keeps track of resources, in particular volumes and snapshots, that
// need to be freed when testing is done. It implements both ControllerClient
// and NodeClient and should be used as the only interaction point to either
// APIs. That way, Resources can ensure that resources are marked for cleanup as
// necessary.
// All methods can be called concurrently.
type Resources struct {
	Context *TestContext
	// ControllerClient is meant for struct-internal usage. It should only be
	// invoked directly if automatic cleanup is not desired and cannot be
	// avoided otherwise.
	csi.ControllerClient
	// GroupControllerClient is meant for struct-internal usage. It should only be
	// invoked directly if automatic cleanup is not desired and cannot be
	// avoided otherwise.
	csi.GroupControllerClient
	// NodeClient is meant for struct-internal usage. It should only be invoked
	// directly if automatic cleanup is not desired and cannot be avoided
	// otherwise.
	csi.NodeClient

	// mutex protects access to managedResourceInfos.
	mutex                sync.Mutex
	managedResourceInfos []resourceInfo
}

// ControllerClient interface wrappers

// MustCreateVolume is like CreateVolume but asserts that the volume was
// successfully created.
func (cl *Resources) MustCreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) *csi.CreateVolumeResponse {
	GinkgoHelper()
	vol, err := cl.CreateVolume(ctx, req)
	Expect(err).NotTo(HaveOccurred(), "volume create failed")
	Expect(vol).NotTo(BeNil(), "volume response is nil")
	Expect(vol.GetVolume()).NotTo(BeNil(), "volume in response is nil")
	Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty(), "volume ID in response is missing")
	return vol
}

// CreateVolume proxies to a Controller service implementation and registers the
// volume for cleanup.
func (cl *Resources) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest, opts ...grpc.CallOption) (*csi.CreateVolumeResponse, error) {
	GinkgoHelper()
	vol, err := cl.ControllerClient.CreateVolume(ctx, req, opts...)
	if err == nil && vol != nil && vol.GetVolume().GetVolumeId() != "" {
		cl.registerVolume(vol.GetVolume().GetVolumeId(), volumeInfo{})
	}
	return vol, err
}

// DeleteVolume proxies to a Controller service implementation and unregisters
// the volume from cleanup.
func (cl *Resources) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest, opts ...grpc.CallOption) (*csi.DeleteVolumeResponse, error) {
	GinkgoHelper()
	vol, err := cl.ControllerClient.DeleteVolume(ctx, req, opts...)
	if err == nil {
		cl.unregisterResource(req.VolumeId)
	}
	return vol, err
}

// MustControllerPublishVolume is like ControllerPublishVolume but asserts that
// the volume was successfully controller-published.
func (cl *Resources) MustControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) *csi.ControllerPublishVolumeResponse {
	GinkgoHelper()
	conpubvol, err := cl.ControllerPublishVolume(ctx, req)
	Expect(err).NotTo(HaveOccurred(), "controller publish volume failed")
	Expect(conpubvol).NotTo(BeNil(), "controller publish volume response is nil")
	return conpubvol
}

// ControllerPublishVolume proxies to a Controller service implementation and
// adds the node ID to the corresponding volume for cleanup.
func (cl *Resources) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest, opts ...grpc.CallOption) (*csi.ControllerPublishVolumeResponse, error) {
	GinkgoHelper()
	conpubvol, err := cl.ControllerClient.ControllerPublishVolume(ctx, req, opts...)
	if err == nil && req.VolumeId != "" && req.NodeId != "" {
		cl.registerVolume(req.VolumeId, volumeInfo{NodeID: req.NodeId})
	}
	return conpubvol, err
}

// registerVolume adds or updates an entry for given volume.
func (cl *Resources) registerVolume(id string, info volumeInfo) {
	GinkgoHelper()
	Expect(id).NotTo(BeEmpty(), "volume ID is empty")
	Expect(info).NotTo(BeNil(), "volume info is nil")
	cl.mutex.Lock()
	defer cl.mutex.Unlock()
	klog.V(4).Infof("registering volume ID %s", id)
	cl.managedResourceInfos = append(cl.managedResourceInfos, resourceInfo{
		id:   id,
		data: info,
	})
}

// MustCreateSnapshot is like CreateSnapshot but asserts that the snapshot was
// successfully created.
func (cl *Resources) MustCreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) *csi.CreateSnapshotResponse {
	GinkgoHelper()
	snap, err := cl.CreateSnapshot(ctx, req)
	Expect(err).NotTo(HaveOccurred(), "create snapshot failed")
	Expect(snap).NotTo(BeNil(), "create snasphot response is nil")
	verifySnapshotInfo(snap.GetSnapshot())
	return snap
}

// MustCreateSnapshotFromVolumeRequest creates a volume from the given
// CreateVolumeRequest and a snapshot subsequently. It registers the volume and
// snapshot and asserts that both were created successfully.
func (cl *Resources) MustCreateSnapshotFromVolumeRequest(ctx context.Context, req *csi.CreateVolumeRequest, snapshotName string) (*csi.CreateSnapshotResponse, *csi.CreateVolumeResponse) {
	GinkgoHelper()
	vol := cl.MustCreateVolume(ctx, req)
	snap := cl.MustCreateSnapshot(ctx, MakeCreateSnapshotReq(cl.Context, snapshotName, vol.Volume.VolumeId))
	return snap, vol
}

// CreateSnapshot proxies to a Controller service implementation and registers
// the snapshot for cleanup.
func (cl *Resources) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest, opts ...grpc.CallOption) (*csi.CreateSnapshotResponse, error) {
	GinkgoHelper()
	snap, err := cl.ControllerClient.CreateSnapshot(ctx, req, opts...)
	if err == nil && snap.GetSnapshot().GetSnapshotId() != "" {
		cl.registerSnapshot(snap.Snapshot.SnapshotId)
	}
	return snap, err
}

// DeleteSnapshot proxies to a Controller service implementation and unregisters
// the snapshot from cleanup.
func (cl *Resources) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest, opts ...grpc.CallOption) (*csi.DeleteSnapshotResponse, error) {
	GinkgoHelper()
	snap, err := cl.ControllerClient.DeleteSnapshot(ctx, req, opts...)
	if err == nil && req.SnapshotId != "" {
		cl.unregisterResource(req.SnapshotId)
	}
	return snap, err
}

func (cl *Resources) registerSnapshot(id string) {
	GinkgoHelper()
	cl.mutex.Lock()
	defer cl.mutex.Unlock()
	Expect(id).NotTo(BeEmpty(), "ID for register snapshot is missing")
	klog.V(4).Infof("registering snapshot ID %s", id)
	cl.managedResourceInfos = append(cl.managedResourceInfos, resourceInfo{
		id:   id,
		data: snapshotInfo{},
	})
}

func (cl *Resources) unregisterResource(id string) {
	GinkgoHelper()
	cl.mutex.Lock()
	defer cl.mutex.Unlock()
	Expect(id).NotTo(BeEmpty(), "ID for unregister resource is missing")
	// Find resource info with the given ID and remove it.
	for i, resInfo := range cl.managedResourceInfos {
		if resInfo.id == id {
			klog.V(4).Infof("unregistering resource ID %s", id)
			cl.managedResourceInfos = append(cl.managedResourceInfos[:i], cl.managedResourceInfos[i+1:]...)
			return
		}
	}
}

// Cleanup calls unpublish methods as needed and deletes all managed resources.
func (cl *Resources) Cleanup() {
	GinkgoHelper()
	klog.V(4).Info("cleaning up all registered resources")
	cl.mutex.Lock()
	defer cl.mutex.Unlock()
	ctx := context.Background()

	// Clean up resources in LIFO order to account for dependency order.
	var errs []error
	for i := len(cl.managedResourceInfos) - 1; i >= 0; i-- {
		resInfo := cl.managedResourceInfos[i]
		id := resInfo.id
		switch resType := resInfo.data.(type) {
		case volumeInfo:
			errs = append(errs, cl.cleanupVolume(ctx, id, resType)...)
		case snapshotInfo:
			errs = append(errs, cl.cleanupSnapshot(ctx, id)...)
		default:
			Fail(fmt.Sprintf("unknown resource type: %T", resType), 1)
		}
	}

	Expect(errs).To(BeEmpty(), "resource cleanup failed")

	klog.V(4).Info("clearing managed resources list")
	cl.managedResourceInfos = []resourceInfo{}
}

func (cl *Resources) cleanupVolume(ctx context.Context, volumeID string, info volumeInfo) (errs []error) {
	klog.V(4).Infof("deleting volume ID %s", volumeID)
	if cl.NodeClient != nil {
		if _, err := cl.NodeUnpublishVolume(
			ctx,
			&csi.NodeUnpublishVolumeRequest{
				VolumeId:   volumeID,
				TargetPath: cl.Context.TargetPath + "/target",
			},
		); isRelevantError(err) {
			errs = append(errs, fmt.Errorf("NodeUnpublishVolume for volume ID %s failed: %s", volumeID, err))
		}

		if isNodeCapabilitySupported(cl, csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME) {
			if _, err := cl.NodeUnstageVolume(
				ctx,
				&csi.NodeUnstageVolumeRequest{
					VolumeId:          volumeID,
					StagingTargetPath: cl.Context.StagingPath,
				},
			); isRelevantError(err) {
				errs = append(errs, fmt.Errorf("NodeUnstageVolume for volume ID %s failed: %s", volumeID, err))
			}
		}
	}

	if info.NodeID != "" {
		if _, err := cl.ControllerClient.ControllerUnpublishVolume(
			ctx,
			&csi.ControllerUnpublishVolumeRequest{
				VolumeId: volumeID,
				NodeId:   info.NodeID,
				Secrets:  cl.Context.Secrets.ControllerUnpublishVolumeSecret,
			},
		); err != nil {
			errs = append(errs, fmt.Errorf("ControllerUnpublishVolume for volume ID %s failed: %s", volumeID, err))
		}
	}

	if _, err := cl.ControllerClient.DeleteVolume(
		ctx,
		&csi.DeleteVolumeRequest{
			VolumeId: volumeID,
			Secrets:  cl.Context.Secrets.DeleteVolumeSecret,
		},
	); err != nil {
		errs = append(errs, fmt.Errorf("DeleteVolume for volume ID %s failed: %s", volumeID, err))
	}

	return errs
}

func (cl *Resources) cleanupSnapshot(ctx context.Context, snapshotID string) []error {
	klog.Infof("deleting snapshot ID %s", snapshotID)
	if _, err := cl.ControllerClient.DeleteSnapshot(
		ctx,
		&csi.DeleteSnapshotRequest{
			SnapshotId: snapshotID,
			Secrets:    cl.Context.Secrets.DeleteSnapshotSecret,
		},
	); err != nil {
		return []error{fmt.Errorf("DeleteSnapshot for snapshot ID %s failed: %s", snapshotID, err)}
	}

	return nil
}

func isRelevantError(err error) bool {
	return err != nil && status.Code(err) != codes.NotFound
}
