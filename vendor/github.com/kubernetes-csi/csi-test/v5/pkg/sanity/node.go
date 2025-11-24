/*
Copyright 2017 The Kubernetes Authors.

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

func isNodeCapabilitySupported(c csi.NodeClient,
	capType csi.NodeServiceCapability_RPC_Type,
) bool {

	caps, err := c.NodeGetCapabilities(
		context.Background(),
		&csi.NodeGetCapabilitiesRequest{})
	Expect(err).NotTo(HaveOccurred())
	Expect(caps).NotTo(BeNil())

	for _, cap := range caps.GetCapabilities() {
		Expect(cap.GetRpc()).NotTo(BeNil())
		if cap.GetRpc().GetType() == capType {
			return true
		}
	}
	return false
}

func isPluginCapabilitySupported(c csi.IdentityClient,
	capType csi.PluginCapability_Service_Type,
) bool {

	caps, err := c.GetPluginCapabilities(
		context.Background(),
		&csi.GetPluginCapabilitiesRequest{})
	Expect(err).NotTo(HaveOccurred())
	Expect(caps).NotTo(BeNil())

	for _, cap := range caps.GetCapabilities() {
		if cap.GetService() != nil && cap.GetService().GetType() == capType {
			return true
		}
	}
	return false
}

func runControllerTest(sc *TestContext, r *Resources, controllerPublishSupported bool, nodeStageSupported bool, nodeVolumeStatsSupported bool, count int) {

	name := UniqueString(fmt.Sprintf("sanity-node-full-%d", count))

	By("getting node information")
	ni, err := r.NodeGetInfo(
		context.Background(),
		&csi.NodeGetInfoRequest{})
	Expect(err).NotTo(HaveOccurred())
	Expect(ni).NotTo(BeNil())
	Expect(ni.GetNodeId()).NotTo(BeEmpty())

	var accReqs *csi.TopologyRequirement
	if ni.AccessibleTopology != nil {
		// Topology requirements are honored if provided by the driver
		accReqs = &csi.TopologyRequirement{
			Requisite: []*csi.Topology{ni.AccessibleTopology},
		}
	}

	// Create Volume First
	By("creating a single node writer volume")
	vol := r.MustCreateVolume(
		context.Background(),
		&csi.CreateVolumeRequest{
			Name: name,
			VolumeCapabilities: []*csi.VolumeCapability{
				TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
			},
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: TestVolumeSize(sc),
			},
			Secrets:                   sc.Secrets.CreateVolumeSecret,
			Parameters:                sc.Config.TestVolumeParameters,
			AccessibilityRequirements: accReqs,
		},
	)

	var conpubvol *csi.ControllerPublishVolumeResponse
	if controllerPublishSupported {
		By("controller publishing volume")

		conpubvol, err = r.ControllerPublishVolume(
			context.Background(),
			&csi.ControllerPublishVolumeRequest{
				VolumeId:         vol.GetVolume().GetVolumeId(),
				NodeId:           ni.GetNodeId(),
				VolumeCapability: TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
				VolumeContext:    vol.GetVolume().GetVolumeContext(),
				Readonly:         false,
				Secrets:          sc.Secrets.ControllerPublishVolumeSecret,
			},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(conpubvol).NotTo(BeNil())
	}
	// NodeStageVolume
	if nodeStageSupported {
		for i := 0; i < count; i++ {
			By("node staging volume")
			nodestagevol, err := r.NodeStageVolume(
				context.Background(),
				&csi.NodeStageVolumeRequest{
					VolumeId:          vol.GetVolume().GetVolumeId(),
					VolumeCapability:  TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					StagingTargetPath: sc.StagingPath,
					VolumeContext:     vol.GetVolume().GetVolumeContext(),
					PublishContext:    conpubvol.GetPublishContext(),
					Secrets:           sc.Secrets.NodeStageVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodestagevol).NotTo(BeNil())
		}
	}
	// NodePublishVolume
	var stagingPath string
	if nodeStageSupported {
		stagingPath = sc.StagingPath
	}
	for i := 0; i < count; i++ {
		By("publishing the volume on a node")
		nodepubvol, err := r.NodePublishVolume(
			context.Background(),
			&csi.NodePublishVolumeRequest{
				VolumeId:          vol.GetVolume().GetVolumeId(),
				TargetPath:        sc.TargetPath + "/target",
				StagingTargetPath: stagingPath,
				VolumeCapability:  TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
				VolumeContext:     vol.GetVolume().GetVolumeContext(),
				PublishContext:    conpubvol.GetPublishContext(),
				Secrets:           sc.Secrets.NodePublishVolumeSecret,
			},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(nodepubvol).NotTo(BeNil())
	}

	// NodeGetVolumeStats
	if nodeVolumeStatsSupported {
		By("Get node volume stats")
		statsResp, err := r.NodeGetVolumeStats(
			context.Background(),
			&csi.NodeGetVolumeStatsRequest{
				VolumeId:   vol.GetVolume().GetVolumeId(),
				VolumePath: sc.TargetPath + "/target",
			},
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(statsResp.GetUsage()).ToNot(BeNil())
	}
}

var _ = DescribeSanity("Node Service", func(sc *TestContext) {
	var (
		r *Resources

		providesControllerService    bool
		controllerPublishSupported   bool
		nodeStageSupported           bool
		nodeVolumeStatsSupported     bool
		nodeExpansionSupported       bool
		controllerExpansionSupported bool
	)

	createVolume := func(volumeName string) *csi.CreateVolumeResponse {
		By("creating a single node writer volume for expansion")
		return r.MustCreateVolume(
			context.Background(),
			&csi.CreateVolumeRequest{
				Name: volumeName,
				VolumeCapabilities: []*csi.VolumeCapability{
					TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
				},
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: TestVolumeSize(sc),
				},
				Secrets:    sc.Secrets.CreateVolumeSecret,
				Parameters: sc.Config.TestVolumeParameters,
			},
		)
	}

	controllerPublishVolume := func(volumeName string, vol *csi.CreateVolumeResponse, nid *csi.NodeGetInfoResponse) *csi.ControllerPublishVolumeResponse {
		var conpubvol *csi.ControllerPublishVolumeResponse
		if controllerPublishSupported {
			By("controller publishing volume")

			conpubvol = r.MustControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					VolumeId:         vol.GetVolume().GetVolumeId(),
					NodeId:           nid.GetNodeId(),
					VolumeCapability: TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					VolumeContext:    vol.GetVolume().GetVolumeContext(),
					Readonly:         false,
					Secrets:          sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
		}
		return conpubvol
	}

	nodeStageVolume := func(volumeName string, vol *csi.CreateVolumeResponse, conpubvol *csi.ControllerPublishVolumeResponse) *csi.NodeStageVolumeResponse {
		// NodeStageVolume
		if nodeStageSupported {
			By("node staging volume")
			nodeStageRequest := &csi.NodeStageVolumeRequest{
				VolumeId:          vol.GetVolume().GetVolumeId(),
				VolumeCapability:  TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
				StagingTargetPath: sc.StagingPath,
				VolumeContext:     vol.GetVolume().GetVolumeContext(),
				Secrets:           sc.Secrets.NodeStageVolumeSecret,
			}
			if conpubvol != nil {
				nodeStageRequest.PublishContext = conpubvol.GetPublishContext()
			}
			nodestagevol, err := r.NodeStageVolume(
				context.Background(),
				nodeStageRequest,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodestagevol).NotTo(BeNil())
			return nodestagevol
		}
		return nil
	}

	nodePublishVolume := func(volumeName string, vol *csi.CreateVolumeResponse, conpubvol *csi.ControllerPublishVolumeResponse) *csi.NodePublishVolumeResponse {
		By("publishing the volume on a node")
		var stagingPath string
		if nodeStageSupported {
			stagingPath = sc.StagingPath
		}
		nodePublishRequest := &csi.NodePublishVolumeRequest{
			VolumeId:          vol.GetVolume().GetVolumeId(),
			TargetPath:        sc.TargetPath + "/target",
			StagingTargetPath: stagingPath,
			VolumeCapability:  TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
			VolumeContext:     vol.GetVolume().GetVolumeContext(),
			Secrets:           sc.Secrets.NodePublishVolumeSecret,
		}

		if conpubvol != nil {
			nodePublishRequest.PublishContext = conpubvol.GetPublishContext()
		}

		nodepubvol, err := r.NodePublishVolume(
			context.Background(),
			nodePublishRequest,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(nodepubvol).NotTo(BeNil())
		return nodepubvol
	}

	BeforeEach(func() {
		cl := csi.NewControllerClient(sc.ControllerConn)
		n := csi.NewNodeClient(sc.Conn)

		i := csi.NewIdentityClient(sc.Conn)
		req := &csi.GetPluginCapabilitiesRequest{}
		res, err := i.GetPluginCapabilities(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res).NotTo(BeNil())
		for _, cap := range res.GetCapabilities() {
			switch cap.GetType().(type) {
			case *csi.PluginCapability_Service_:
				switch cap.GetService().GetType() {
				case csi.PluginCapability_Service_CONTROLLER_SERVICE:
					providesControllerService = true
				}
			}
		}
		if providesControllerService {
			controllerPublishSupported = isControllerCapabilitySupported(
				cl,
				csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME)
		}
		nodeStageSupported = isNodeCapabilitySupported(n, csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME)
		nodeVolumeStatsSupported = isNodeCapabilitySupported(n, csi.NodeServiceCapability_RPC_GET_VOLUME_STATS)
		nodeExpansionSupported = isNodeCapabilitySupported(n, csi.NodeServiceCapability_RPC_EXPAND_VOLUME)
		controllerExpansionSupported = isControllerCapabilitySupported(cl, csi.ControllerServiceCapability_RPC_EXPAND_VOLUME)
		r = &Resources{
			Context:          sc,
			ControllerClient: cl,
			NodeClient:       n,
		}
	})

	AfterEach(func() {
		r.Cleanup()
	})

	Describe("NodeGetCapabilities", func() {
		It("should return appropriate capabilities", func() {
			caps, err := r.NodeGetCapabilities(
				context.Background(),
				&csi.NodeGetCapabilitiesRequest{})

			By("checking successful response")
			Expect(err).NotTo(HaveOccurred())
			Expect(caps).NotTo(BeNil())

			for _, cap := range caps.GetCapabilities() {
				Expect(cap.GetRpc()).NotTo(BeNil())

				switch cap.GetRpc().GetType() {
				case csi.NodeServiceCapability_RPC_UNKNOWN:
				case csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME:
				case csi.NodeServiceCapability_RPC_GET_VOLUME_STATS:
				case csi.NodeServiceCapability_RPC_EXPAND_VOLUME:
				case csi.NodeServiceCapability_RPC_VOLUME_CONDITION:
				case csi.NodeServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER:
				case csi.NodeServiceCapability_RPC_VOLUME_MOUNT_GROUP:
				default:
					Fail(fmt.Sprintf("Unknown capability: %v\n", cap.GetRpc().GetType()))
				}
			}
		})
	})

	Describe("NodeGetInfo", func() {
		var (
			i                                csi.IdentityClient
			accessibilityConstraintSupported bool
		)

		BeforeEach(func() {
			i = csi.NewIdentityClient(sc.Conn)
			accessibilityConstraintSupported = isPluginCapabilitySupported(i, csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS)
		})

		It("should return appropriate values", func() {
			ninfo, err := r.NodeGetInfo(
				context.Background(),
				&csi.NodeGetInfoRequest{})

			Expect(err).NotTo(HaveOccurred())
			Expect(ninfo).NotTo(BeNil())
			Expect(ninfo.GetNodeId()).NotTo(BeEmpty())
			Expect(ninfo.GetMaxVolumesPerNode()).NotTo(BeNumerically("<", 0))

			if accessibilityConstraintSupported {
				Expect(ninfo.GetAccessibleTopology()).NotTo(BeNil())
			}
		})
	})

	Describe("NodePublishVolume", func() {
		It("should fail when no volume id is provided", func() {
			rsp, err := r.NodePublishVolume(
				context.Background(),
				&csi.NodePublishVolumeRequest{
					Secrets: sc.Secrets.NodePublishVolumeSecret,
				},
			)
			ExpectErrorCode(rsp, err, codes.InvalidArgument)
		})

		It("should fail when no target path is provided", func() {
			rsp, err := r.NodePublishVolume(
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId: sc.Config.IDGen.GenerateUniqueValidVolumeID(),
					Secrets:  sc.Secrets.NodePublishVolumeSecret,
				},
			)
			ExpectErrorCode(rsp, err, codes.InvalidArgument)
		})

		It("should fail when no volume capability is provided", func() {
			rsp, err := r.NodePublishVolume(
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:         sc.Config.IDGen.GenerateUniqueValidVolumeID(),
					VolumeCapability: nil,
					TargetPath:       sc.TargetPath + "/target",
					Secrets:          sc.Secrets.NodePublishVolumeSecret,
				},
			)
			ExpectErrorCode(rsp, err, codes.InvalidArgument)
		})
	})

	Describe("NodeUnpublishVolume", func() {
		It("should fail when no volume id is provided", func() {

			rsp, err := r.NodeUnpublishVolume(
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{})
			ExpectErrorCode(rsp, err, codes.InvalidArgument)
		})

		It("should fail when no target path is provided", func() {

			rsp, err := r.NodeUnpublishVolume(
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId: sc.Config.IDGen.GenerateUniqueValidVolumeID(),
				})
			ExpectErrorCode(rsp, err, codes.InvalidArgument)
		})

		It("should remove target path", func() {
			// This test may break for consumers that are using
			// custom target path functions if they have not yet
			// implemented similar functionality to check if the
			// path exists. Skip this test if there is a custom
			// command or function provided to create the path,
			// but not yet provided to check the path.
			if sc.Config.CreateTargetPathCmd != "" && sc.Config.CheckPathCmd == "" {
				Skip("CreateTargetPathCmd was set, but CheckPathCmd was not. Please update your testing configuration to enable CheckPathCmd.")
			}
			if sc.Config.CreateTargetDir != nil && sc.Config.CheckPath == nil {
				Skip("CreateTargetDir was set, but CheckPath was not. Please update your testing configuration to enable CheckPath.")
			}

			name := UniqueString("sanity-node-unpublish-volume")
			vol := createVolume(name)
			volid := vol.GetVolume().GetVolumeId()
			volpath := sc.TargetPath + "/target"

			By("Getting a node id")
			nid, err := r.NodeGetInfo(
				context.Background(),
				&csi.NodeGetInfoRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(nid).NotTo(BeNil())
			Expect(nid.GetNodeId()).NotTo(BeEmpty())

			By("Staging and publishing a volume")
			conpubvol := controllerPublishVolume(name, vol, nid)
			_ = nodeStageVolume(name, vol, conpubvol)
			_ = nodePublishVolume(name, vol, conpubvol)

			// Verify that the path exists before calling
			// NodeUnpublishVolume.
			By("Checking the target path exists")
			pa, err := CheckPath(volpath, sc.Config)
			Expect(err).NotTo(HaveOccurred(), "checking path %q", volpath)
			Expect(pa).NotTo(Equal(PathIsNotFound), "path %q should have been created by CSI driver and the test config should be enabling testing for that path", volpath)

			By("Unpublishing the volume")
			_, err = r.NodeUnpublishVolume(
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   volid,
					TargetPath: volpath,
				},
			)
			Expect(err).NotTo(HaveOccurred())

			// The CSI spec states that the SP MUST delete
			// the file or directory it created at this path
			// as part of NodeUnpublishVolume.
			By("Checking the target path was removed")
			pa, err = CheckPath(volpath, sc.Config)
			Expect(err).NotTo(HaveOccurred(), "checking path %q", volpath)
			Expect(pa).To(Equal(PathIsNotFound), "path %q should have been removed by the CSI driver during NodeUnpublishVolume", volpath)
		})
	})

	Describe("NodeStageVolume", func() {
		var (
			device string
		)

		BeforeEach(func() {
			if !nodeStageSupported {
				Skip("NodeStageVolume not supported")
			}

			device = "/dev/mock"
		})

		It("should fail when no volume id is provided", func() {
			rsp, err := r.NodeStageVolume(
				context.Background(),
				&csi.NodeStageVolumeRequest{
					StagingTargetPath: sc.StagingPath,
					VolumeCapability:  TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					PublishContext: map[string]string{
						"device": device,
					},
					Secrets: sc.Secrets.NodeStageVolumeSecret,
				},
			)
			ExpectErrorCode(rsp, err, codes.InvalidArgument)
		})

		It("should fail when no staging target path is provided", func() {
			rsp, err := r.NodeStageVolume(
				context.Background(),
				&csi.NodeStageVolumeRequest{
					VolumeId:         sc.Config.IDGen.GenerateUniqueValidVolumeID(),
					VolumeCapability: TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					PublishContext: map[string]string{
						"device": device,
					},
					Secrets: sc.Secrets.NodeStageVolumeSecret,
				},
			)
			ExpectErrorCode(rsp, err, codes.InvalidArgument)
		})

		It("should fail when no volume capability is provided", func() {

			// Create Volume First
			By("creating a single node writer volume")
			name := UniqueString("sanity-node-stage-nocaps")

			vol := r.MustCreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Name: name,
					VolumeCapabilities: []*csi.VolumeCapability{
						{
							AccessType: &csi.VolumeCapability_Mount{
								Mount: &csi.VolumeCapability_MountVolume{},
							},
							AccessMode: &csi.VolumeCapability_AccessMode{
								Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
							},
						},
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)

			rsp, err := r.NodeStageVolume(
				context.Background(),
				&csi.NodeStageVolumeRequest{
					VolumeId:          vol.GetVolume().GetVolumeId(),
					StagingTargetPath: sc.StagingPath,
					PublishContext: map[string]string{
						"device": device,
					},
					Secrets: sc.Secrets.NodeStageVolumeSecret,
				},
			)
			ExpectErrorCode(rsp, err, codes.InvalidArgument)
		})
	})

	Describe("NodeUnstageVolume", func() {
		BeforeEach(func() {
			if !nodeStageSupported {
				Skip("NodeUnstageVolume not supported")
			}
		})

		It("should fail when no volume id is provided", func() {

			rsp, err := r.NodeUnstageVolume(
				context.Background(),
				&csi.NodeUnstageVolumeRequest{
					StagingTargetPath: sc.StagingPath,
				})
			ExpectErrorCode(rsp, err, codes.InvalidArgument)
		})

		It("should fail when no staging target path is provided", func() {

			rsp, err := r.NodeUnstageVolume(
				context.Background(),
				&csi.NodeUnstageVolumeRequest{
					VolumeId: sc.Config.IDGen.GenerateUniqueValidVolumeID(),
				})
			ExpectErrorCode(rsp, err, codes.InvalidArgument)
		})
	})

	Describe("NodeGetVolumeStats", func() {
		BeforeEach(func() {
			if !nodeVolumeStatsSupported {
				Skip("NodeGetVolume not supported")
			}
		})

		It("should fail when no volume id is provided", func() {
			rsp, err := r.NodeGetVolumeStats(
				context.Background(),
				&csi.NodeGetVolumeStatsRequest{
					VolumePath: "some/path",
				},
			)
			ExpectErrorCode(rsp, err, codes.InvalidArgument)
		})

		It("should fail when no volume path is provided", func() {
			rsp, err := r.NodeGetVolumeStats(
				context.Background(),
				&csi.NodeGetVolumeStatsRequest{
					VolumeId: sc.Config.IDGen.GenerateUniqueValidVolumeID(),
				},
			)
			ExpectErrorCode(rsp, err, codes.InvalidArgument)
		})

		It("should fail when volume is not found", func() {
			rsp, err := r.NodeGetVolumeStats(
				context.Background(),
				&csi.NodeGetVolumeStatsRequest{
					VolumeId:   sc.Config.IDGen.GenerateUniqueValidVolumeID(),
					VolumePath: "some/path",
				},
			)
			ExpectErrorCode(rsp, err, codes.NotFound)
		})

		It("should fail when volume does not exist on the specified path", func() {
			name := UniqueString("sanity-node-get-volume-stats")

			vol := createVolume(name)

			By("getting a node id")
			nid, err := r.NodeGetInfo(
				context.Background(),
				&csi.NodeGetInfoRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(nid).NotTo(BeNil())
			Expect(nid.GetNodeId()).NotTo(BeEmpty())

			conpubvol := controllerPublishVolume(name, vol, nid)

			// NodeStageVolume
			_ = nodeStageVolume(name, vol, conpubvol)

			// NodePublishVolume
			_ = nodePublishVolume(name, vol, conpubvol)

			// NodeGetVolumeStats
			By("Get node volume stats")
			rsp, err := r.NodeGetVolumeStats(
				context.Background(),
				&csi.NodeGetVolumeStatsRequest{
					VolumeId:   vol.GetVolume().GetVolumeId(),
					VolumePath: "some/path",
				},
			)
			ExpectErrorCode(rsp, err, codes.NotFound)
		})

	})

	Describe("NodeExpandVolume", func() {
		BeforeEach(func() {
			if !nodeExpansionSupported {
				Skip("NodeExpandVolume not supported")
			}

		})

		It("should fail when no volume id is provided", func() {
			rsp, err := r.NodeExpandVolume(
				context.Background(),
				&csi.NodeExpandVolumeRequest{
					VolumePath:       sc.TargetPath,
					VolumeCapability: TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
				},
			)
			ExpectErrorCode(rsp, err, codes.InvalidArgument)
		})

		It("should fail when no volume path is provided", func() {
			name := UniqueString("sanity-node-expand-volume-valid-id")

			vol := createVolume(name)

			rsp, err := r.NodeExpandVolume(
				context.Background(),
				&csi.NodeExpandVolumeRequest{
					VolumeId:         vol.GetVolume().VolumeId,
					VolumeCapability: TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
				},
			)
			ExpectErrorCode(rsp, err, codes.InvalidArgument)
		})

		It("should fail when volume is not found", func() {
			rsp, err := r.NodeExpandVolume(
				context.Background(),
				&csi.NodeExpandVolumeRequest{
					VolumeId:   sc.Config.IDGen.GenerateUniqueValidVolumeID(),
					VolumePath: "some/path",
				},
			)
			ExpectErrorCode(rsp, err, codes.NotFound)
		})

		It("should work if node-expand is called after node-publish", func() {
			name := UniqueString("sanity-node-expand-volume")

			// Created volumes are automatically cleaned up via cl.DeleteVolumes
			vol := createVolume(name)

			if controllerExpansionSupported {
				By("controller expanding the volume")
				expReq := &csi.ControllerExpandVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: TestVolumeExpandSize(sc),
					},
					Secrets: sc.Secrets.ControllerExpandVolumeSecret,
				}
				rsp, err := r.ControllerExpandVolume(context.Background(), expReq)
				Expect(err).NotTo(HaveOccurred())
				Expect(rsp).NotTo(BeNil())
				Expect(rsp.GetCapacityBytes()).To(Equal(TestVolumeExpandSize(sc)))
			}

			By("getting a node id")
			nid, err := r.NodeGetInfo(
				context.Background(),
				&csi.NodeGetInfoRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(nid).NotTo(BeNil())
			Expect(nid.GetNodeId()).NotTo(BeEmpty())

			conpubvol := controllerPublishVolume(name, vol, nid)

			// NodeStageVolume
			_ = nodeStageVolume(name, vol, conpubvol)

			// NodePublishVolume
			_ = nodePublishVolume(name, vol, conpubvol)

			By("expanding the volume on a node")
			_, err = r.NodeExpandVolume(
				context.Background(),
				&csi.NodeExpandVolumeRequest{
					VolumeId:   vol.GetVolume().GetVolumeId(),
					VolumePath: sc.TargetPath + "/target",
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: TestVolumeExpandSize(sc),
					},
				},
			)
			Expect(err).ToNot(HaveOccurred(), "while expanding volume on node")
		})
	})

	// CSI spec poses no specific requirements for the cluster/storage setups that a SP MUST support. To perform
	// meaningful checks the following test assumes that topology-aware provisioning on a single node setup is supported
	It("should work", func() {
		if !providesControllerService {
			Skip("Controller Service not provided: CreateVolume not supported")
		}
		By("runControllerTest")
		runControllerTest(sc, r, controllerPublishSupported, nodeStageSupported, nodeVolumeStatsSupported, 1)
	})
	It("should be idempotent", func() {
		if !providesControllerService {
			Skip("Controller Service not provided: CreateVolume not supported")
		}
		if sc.Config.IdempotentCount <= 0 {
			Skip("Config.IdempotentCount is zero or negative, skip tests")
		}
		count := sc.Config.IdempotentCount
		By("runControllerTest with Idempotent count")
		runControllerTest(sc, r, controllerPublishSupported, nodeStageSupported, nodeVolumeStatsSupported, count)
	})
})
