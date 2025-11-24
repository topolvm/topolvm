/*
Copyright 2023 The Kubernetes Authors.

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

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func isGroupControllerCapabilitySupported(
	c csi.GroupControllerClient,
	capType csi.GroupControllerServiceCapability_RPC_Type,
) bool {

	caps, err := c.GroupControllerGetCapabilities(
		context.Background(),
		&csi.GroupControllerGetCapabilitiesRequest{})
	Expect(err).NotTo(HaveOccurred())
	Expect(caps).NotTo(BeNil())
	Expect(caps.GetCapabilities()).NotTo(BeNil())

	for _, cap := range caps.GetCapabilities() {
		Expect(cap.GetRpc()).NotTo(BeNil())
		if cap.GetRpc().GetType() == capType {
			return true
		}
	}
	return false
}

var _ = DescribeSanity("GroupController Service [GroupController Server]", func(sc *TestContext) {
	var r *Resources

	BeforeEach(func() {
		var providesGroupControllerService bool

		i := csi.NewIdentityClient(sc.Conn)
		req := &csi.GetPluginCapabilitiesRequest{}
		res, err := i.GetPluginCapabilities(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res).NotTo(BeNil())

		for _, cap := range res.GetCapabilities() {
			switch cap.GetType().(type) {
			case *csi.PluginCapability_Service_:
				switch cap.GetService().GetType() {
				case csi.PluginCapability_Service_GROUP_CONTROLLER_SERVICE:
					providesGroupControllerService = true
				}
			}
		}

		if !providesGroupControllerService {
			Skip("GroupControllerService not supported")
		}

		r = &Resources{
			Context:               sc,
			ControllerClient:      csi.NewControllerClient(sc.ControllerConn),
			GroupControllerClient: csi.NewGroupControllerClient(sc.ControllerConn),
			NodeClient:            csi.NewNodeClient(sc.Conn),
		}
	})

	AfterEach(func() {
		r.Cleanup()
	})

	Describe("GroupControllerGetCapabilities", func() {
		It("should return appropriate capabilities", func() {
			caps, err := r.GroupControllerGetCapabilities(
				context.Background(),
				&csi.GroupControllerGetCapabilitiesRequest{})

			By("checking successful response")
			Expect(err).NotTo(HaveOccurred())
			Expect(caps).NotTo(BeNil())
			Expect(caps.GetCapabilities()).NotTo(BeNil())

			for _, cap := range caps.GetCapabilities() {
				Expect(cap.GetRpc()).NotTo(BeNil())

				switch cap.GetRpc().GetType() {
				case csi.GroupControllerServiceCapability_RPC_CREATE_DELETE_GET_VOLUME_GROUP_SNAPSHOT:
				default:
					Fail(fmt.Sprintf("Unknown capability: %v\n", cap.GetRpc().GetType()))
				}
			}
		})
	})
})

var _ = DescribeSanity("GroupController Service [GroupController VolumeGroupSnapshots]", func(sc *TestContext) {
	var r *Resources

	BeforeEach(func() {
		var providesGroupControllerService bool

		i := csi.NewIdentityClient(sc.Conn)
		req := &csi.GetPluginCapabilitiesRequest{}
		res, err := i.GetPluginCapabilities(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res).NotTo(BeNil())

		for _, cap := range res.GetCapabilities() {
			switch cap.GetType().(type) {
			case *csi.PluginCapability_Service_:
				switch cap.GetService().GetType() {
				case csi.PluginCapability_Service_GROUP_CONTROLLER_SERVICE:
					providesGroupControllerService = true
				}
			}
		}

		if !providesGroupControllerService {
			Skip("GroupControllerService not supported")
		}

		r = &Resources{
			Context:               sc,
			ControllerClient:      csi.NewControllerClient(sc.ControllerConn),
			GroupControllerClient: csi.NewGroupControllerClient(sc.ControllerConn),
			NodeClient:            csi.NewNodeClient(sc.Conn),
		}

		if !isGroupControllerCapabilitySupported(r, csi.GroupControllerServiceCapability_RPC_CREATE_DELETE_GET_VOLUME_GROUP_SNAPSHOT) {
			Skip("VolumeGroupSnapshots not supported")
		}
	})

	AfterEach(func() {
		r.Cleanup()
	})

	Describe("CreateVolumeGroupSnapshot", func() {
		It("should fail when no name is provided", func() {
			rsp, err := r.CreateVolumeGroupSnapshot(
				context.Background(),
				&csi.CreateVolumeGroupSnapshotRequest{
					Secrets: sc.Secrets.CreateSnapshotSecret,
				},
			)
			ExpectErrorCode(rsp, err, codes.InvalidArgument)
		})
	})

	Describe("GetVolumeGroupSnapshot", func() {
		It("should fail when no volume id is provided", func() {
			rsp, err := r.GetVolumeGroupSnapshot(
				context.Background(),
				&csi.GetVolumeGroupSnapshotRequest{
					Secrets: sc.Secrets.ListSnapshotsSecret,
				},
			)
			ExpectErrorCode(rsp, err, codes.InvalidArgument)
		})

		It("should fail when an invalid volume id is used", func() {
			rsp, err := r.GetVolumeGroupSnapshot(
				context.Background(),
				&csi.GetVolumeGroupSnapshotRequest{
					GroupSnapshotId: sc.Config.IDGen.GenerateInvalidVolumeID(),
					Secrets:         sc.Secrets.ListSnapshotsSecret,
				},
			)
			ExpectErrorCode(rsp, err, codes.NotFound)
		})
	})

	Describe("DeleteVolumeGroupSnapshot", func() {
		It("should fail when no volume id is provided", func() {
			rsp, err := r.DeleteVolumeGroupSnapshot(
				context.Background(),
				&csi.DeleteVolumeGroupSnapshotRequest{
					Secrets: sc.Secrets.DeleteSnapshotSecret,
				},
			)
			ExpectErrorCode(rsp, err, codes.InvalidArgument)
		})

		It("should succeed when an invalid volume id is used", func() {
			_, err := r.DeleteVolumeGroupSnapshot(
				context.Background(),
				&csi.DeleteVolumeGroupSnapshotRequest{
					GroupSnapshotId: sc.Config.IDGen.GenerateInvalidVolumeID(),
					Secrets:         sc.Secrets.DeleteSnapshotSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
