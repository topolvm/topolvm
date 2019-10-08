/*
Copyright 2018 Intel Corporation

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
	"log"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"

	. "github.com/onsi/ginkgo"
)

// VolumeInfo keeps track of the information needed to delete a volume.
type VolumeInfo struct {
	// Node on which the volume was published, empty if none
	// or publishing is not supported.
	NodeID string

	// Volume ID assigned by CreateVolume.
	VolumeID string
}

// Cleanup keeps track of resources, in particular volumes, which need
// to be freed when testing is done. All methods can be called concurrently.
type Cleanup struct {
	Context                    *SanityContext
	ControllerClient           csi.ControllerClient
	NodeClient                 csi.NodeClient
	ControllerPublishSupported bool
	NodeStageSupported         bool

	// Maps from volume name to the node ID for which the volume
	// is published and the volume ID.
	volumes map[string]VolumeInfo
	mutex   sync.Mutex
}

// RegisterVolume adds or updates an entry for the volume with the
// given name.
func (cl *Cleanup) RegisterVolume(name string, info VolumeInfo) {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()
	if cl.volumes == nil {
		cl.volumes = make(map[string]VolumeInfo)
	}
	cl.volumes[name] = info
}

// MaybeRegisterVolume adds or updates an entry for the volume with
// the given name if CreateVolume was successful.
func (cl *Cleanup) MaybeRegisterVolume(name string, vol *csi.CreateVolumeResponse, err error) {
	if err == nil && vol.GetVolume().GetVolumeId() != "" {
		cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})
	}
}

// UnregisterVolume removes the entry for the volume with the
// given name, thus preventing all cleanup operations for it.
func (cl *Cleanup) UnregisterVolume(name string) {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()
	cl.unregisterVolume(name)
}
func (cl *Cleanup) unregisterVolume(name string) {
	if cl.volumes != nil {
		delete(cl.volumes, name)
	}
}

// DeleteVolumes stops using the registered volumes and tries to delete all of them.
func (cl *Cleanup) DeleteVolumes() {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()
	if cl.volumes == nil {
		return
	}
	logger := log.New(GinkgoWriter, "cleanup: ", 0)
	ctx := context.Background()

	for name, info := range cl.volumes {
		logger.Printf("deleting %s = %s", name, info.VolumeID)
		if _, err := cl.NodeClient.NodeUnpublishVolume(
			ctx,
			&csi.NodeUnpublishVolumeRequest{
				VolumeId:   info.VolumeID,
				TargetPath: cl.Context.TargetPath + "/target",
			},
		); err != nil {
			logger.Printf("warning: NodeUnpublishVolume: %s", err)
		}

		if cl.NodeStageSupported {
			if _, err := cl.NodeClient.NodeUnstageVolume(
				ctx,
				&csi.NodeUnstageVolumeRequest{
					VolumeId:          info.VolumeID,
					StagingTargetPath: cl.Context.StagingPath,
				},
			); err != nil {
				logger.Printf("warning: NodeUnstageVolume: %s", err)
			}
		}

		if cl.ControllerPublishSupported && info.NodeID != "" {
			if _, err := cl.ControllerClient.ControllerUnpublishVolume(
				ctx,
				&csi.ControllerUnpublishVolumeRequest{
					VolumeId: info.VolumeID,
					NodeId:   info.NodeID,
					Secrets:  cl.Context.Secrets.ControllerUnpublishVolumeSecret,
				},
			); err != nil {
				logger.Printf("warning: ControllerUnpublishVolume: %s", err)
			}
		}

		if _, err := cl.ControllerClient.DeleteVolume(
			ctx,
			&csi.DeleteVolumeRequest{
				VolumeId: info.VolumeID,
				Secrets:  cl.Context.Secrets.DeleteVolumeSecret,
			},
		); err != nil {
			logger.Printf("error: DeleteVolume: %s", err)
		}

		cl.unregisterVolume(name)
	}
}
