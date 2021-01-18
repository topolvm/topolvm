/*
Copyright 2017 Kubernetes Authors.

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
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/container-storage-interface/spec/lib/go/csi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	// DefTestVolumeSize defines the base size of dynamically
	// provisioned volumes. 10GB by default, can be overridden by
	// setting Config.TestVolumeSize.
	DefTestVolumeSize int64 = 10 * 1024 * 1024 * 1024

	// DefTestVolumeExpand defines the size increment for volume
	// expansion. It can be overriden by setting an
	// Config.TestVolumeExpandSize, which will be taken as absolute
	// value.
	DefTestExpandIncrement int64 = 1 * 1024 * 1024 * 1024

	MaxNameLength int = 128
)

func TestVolumeSize(sc *SanityContext) int64 {
	if sc.Config.TestVolumeSize > 0 {
		return sc.Config.TestVolumeSize
	}
	return DefTestVolumeSize
}

func TestVolumeExpandSize(sc *SanityContext) int64 {
	if sc.Config.TestVolumeExpandSize > 0 {
		return sc.Config.TestVolumeExpandSize
	}
	return TestVolumeSize(sc) + DefTestExpandIncrement
}

func verifyVolumeInfo(v *csi.Volume) {
	Expect(v).NotTo(BeNil())
	Expect(v.GetVolumeId()).NotTo(BeEmpty())
}

func verifySnapshotInfo(snapshot *csi.Snapshot) {
	Expect(snapshot).NotTo(BeNil())
	Expect(snapshot.GetSnapshotId()).NotTo(BeEmpty())
	Expect(snapshot.GetSourceVolumeId()).NotTo(BeEmpty())
	Expect(snapshot.GetCreationTime()).NotTo(BeZero())
}

func isControllerCapabilitySupported(
	c csi.ControllerClient,
	capType csi.ControllerServiceCapability_RPC_Type,
) bool {

	caps, err := c.ControllerGetCapabilities(
		context.Background(),
		&csi.ControllerGetCapabilitiesRequest{})
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

var _ = DescribeSanity("Controller Service [Controller Server]", func(sc *SanityContext) {
	var (
		c csi.ControllerClient
		n csi.NodeClient

		cl *Cleanup
	)

	BeforeEach(func() {
		c = csi.NewControllerClient(sc.ControllerConn)
		n = csi.NewNodeClient(sc.Conn)

		cl = &Cleanup{
			NodeClient:       n,
			ControllerClient: c,
			Context:          sc,
		}
	})

	AfterEach(func() {
		cl.DeleteVolumes()
	})

	Describe("ControllerGetCapabilities", func() {
		It("should return appropriate capabilities", func() {
			caps, err := c.ControllerGetCapabilities(
				context.Background(),
				&csi.ControllerGetCapabilitiesRequest{})

			By("checking successful response")
			Expect(err).NotTo(HaveOccurred())
			Expect(caps).NotTo(BeNil())
			Expect(caps.GetCapabilities()).NotTo(BeNil())

			for _, cap := range caps.GetCapabilities() {
				Expect(cap.GetRpc()).NotTo(BeNil())

				switch cap.GetRpc().GetType() {
				case csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME:
				case csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME:
				case csi.ControllerServiceCapability_RPC_LIST_VOLUMES:
				case csi.ControllerServiceCapability_RPC_GET_CAPACITY:
				case csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT:
				case csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS:
				case csi.ControllerServiceCapability_RPC_PUBLISH_READONLY:
				case csi.ControllerServiceCapability_RPC_CLONE_VOLUME:
				case csi.ControllerServiceCapability_RPC_EXPAND_VOLUME:
				default:
					Fail(fmt.Sprintf("Unknown capability: %v\n", cap.GetRpc().GetType()))
				}
			}
		})
	})

	Describe("GetCapacity", func() {
		BeforeEach(func() {
			if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_GET_CAPACITY) {
				Skip("GetCapacity not supported")
			}
		})

		It("should return capacity (no optional values added)", func() {
			_, err := c.GetCapacity(
				context.Background(),
				&csi.GetCapacityRequest{})
			Expect(err).NotTo(HaveOccurred())

			// Since capacity is int64 we will not be checking it
			// The value of zero is a possible value.
		})
	})
	Describe("ListVolumes", func() {
		BeforeEach(func() {
			if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_LIST_VOLUMES) {
				Skip("ListVolumes not supported")
			}
		})

		It("should return appropriate values (no optional values added)", func() {
			vols, err := c.ListVolumes(
				context.Background(),
				&csi.ListVolumesRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(vols).NotTo(BeNil())

			for _, vol := range vols.GetEntries() {
				verifyVolumeInfo(vol.GetVolume())
			}
		})

		It("should fail when an invalid starting_token is passed", func() {
			vols, err := c.ListVolumes(
				context.Background(),
				&csi.ListVolumesRequest{
					StartingToken: "invalid-token",
				},
			)
			Expect(err).To(HaveOccurred())
			Expect(vols).To(BeNil())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.Aborted))
		})

		It("check the presence of new volumes and absence of deleted ones in the volume list", func() {
			// List Volumes before creating new volume.
			vols, err := c.ListVolumes(
				context.Background(),
				&csi.ListVolumesRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(vols).NotTo(BeNil())

			totalVols := len(vols.GetEntries())

			By("creating a volume")
			name := "sanity"

			// Create a new volume.
			req := &csi.CreateVolumeRequest{
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
				Secrets: sc.Secrets.CreateVolumeSecret,
			}

			vol, err := c.CreateVolume(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())

			// List volumes and check for the newly created volume.
			vols, err = c.ListVolumes(
				context.Background(),
				&csi.ListVolumesRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(vols).NotTo(BeNil())
			Expect(len(vols.GetEntries())).To(Equal(totalVols + 1))

			By("cleaning up deleting the volume")

			delReq := &csi.DeleteVolumeRequest{
				VolumeId: vol.GetVolume().GetVolumeId(),
				Secrets:  sc.Secrets.DeleteVolumeSecret,
			}

			_, err = c.DeleteVolume(context.Background(), delReq)
			Expect(err).NotTo(HaveOccurred())

			// List volumes and check if the deleted volume exists in the volume list.
			vols, err = c.ListVolumes(
				context.Background(),
				&csi.ListVolumesRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(vols).NotTo(BeNil())
			Expect(len(vols.GetEntries())).To(Equal(totalVols))
		})

		It("pagination should detect volumes added between pages and accept tokens when the last volume from a page is deleted", func() {
			// minVolCount is the minimum number of volumes expected to exist,
			// based on which paginated volume listing is performed.
			minVolCount := 3
			// maxEntried is the maximum entries in list volume request.
			maxEntries := 2
			// existing_vols to keep a record of the volumes that should exist
			existing_vols := map[string]bool{}

			// Get the number of existing volumes.
			vols, err := c.ListVolumes(
				context.Background(),
				&csi.ListVolumesRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(vols).NotTo(BeNil())

			initialTotalVols := len(vols.GetEntries())

			for _, vol := range vols.GetEntries() {
				existing_vols[vol.Volume.VolumeId] = true
			}

			if minVolCount <= initialTotalVols {
				minVolCount = initialTotalVols
			} else {
				// Ensure minimum minVolCount volumes exist.
				By("creating required new volumes")
				for i := initialTotalVols; i < minVolCount; i++ {
					name := "sanity" + strconv.Itoa(i)
					req := &csi.CreateVolumeRequest{
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
						Secrets: sc.Secrets.CreateVolumeSecret,
					}

					vol, err := c.CreateVolume(context.Background(), req)
					Expect(err).NotTo(HaveOccurred())
					Expect(vol).NotTo(BeNil())
					// Register the volume so it's automatically cleaned
					cl.RegisterVolume(vol.Volume.VolumeId, VolumeInfo{VolumeID: vol.Volume.VolumeId})
					existing_vols[vol.Volume.VolumeId] = true
				}
			}

			// Request list volumes with max entries maxEntries.
			vols, err = c.ListVolumes(
				context.Background(),
				&csi.ListVolumesRequest{
					MaxEntries: int32(maxEntries),
				})
			Expect(err).NotTo(HaveOccurred())
			Expect(vols).NotTo(BeNil())
			Expect(len(vols.GetEntries())).To(Equal(maxEntries))

			nextToken := vols.GetNextToken()

			By("removing all listed volumes")
			for _, vol := range vols.GetEntries() {
				Expect(existing_vols[vol.Volume.VolumeId]).To(BeTrue())
				delReq := &csi.DeleteVolumeRequest{
					VolumeId: vol.Volume.VolumeId,
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				}

				_, err := c.DeleteVolume(context.Background(), delReq)
				Expect(err).NotTo(HaveOccurred())
				vol_id := vol.Volume.VolumeId
				existing_vols[vol_id] = false
				cl.UnregisterVolume(vol_id)
			}

			By("creating a new volume")
			req := &csi.CreateVolumeRequest{
				Name: "new-addition",
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
				Secrets: sc.Secrets.CreateVolumeSecret,
			}
			vol, err := c.CreateVolume(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.Volume).NotTo(BeNil())
			existing_vols[vol.Volume.VolumeId] = true

			vols, err = c.ListVolumes(
				context.Background(),
				&csi.ListVolumesRequest{
					StartingToken: nextToken,
				})
			Expect(err).NotTo(HaveOccurred())
			Expect(vols).NotTo(BeNil())
			expected_num_volumes := minVolCount - maxEntries + 1
			// Depending on the plugin implementation we may be missing volumes, but should not get duplicates
			Expect(len(vols.GetEntries()) <= expected_num_volumes).To(BeTrue())
			for _, vol := range vols.GetEntries() {
				Expect(existing_vols[vol.Volume.VolumeId]).To(BeTrue())
				existing_vols[vol.Volume.VolumeId] = false
			}
		})
	})

	Describe("CreateVolume", func() {
		BeforeEach(func() {
			if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME) {
				Skip("CreateVolume not supported")
			}
		})

		It("should fail when no name is provided", func() {
			vol, err := c.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			cl.MaybeRegisterVolume("", vol, err)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail when no volume capabilities are provided", func() {
			name := UniqueString("sanity-controller-create-no-volume-capabilities")
			vol, err := c.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Name:       name,
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			cl.MaybeRegisterVolume(name, vol, err)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should return appropriate values SingleNodeWriter NoCapacity Type:Mount", func() {

			By("creating a volume")
			name := UniqueString("sanity-controller-create-single-no-capacity")

			vol, err := c.CreateVolume(
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
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})

		It("should return appropriate values SingleNodeWriter WithCapacity 1Gi Type:Mount", func() {

			By("creating a volume")
			name := UniqueString("sanity-controller-create-single-with-capacity")

			vol, err := c.CreateVolume(
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
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: TestVolumeSize(sc),
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			if serverError, ok := status.FromError(err); ok &&
				(serverError.Code() == codes.OutOfRange || serverError.Code() == codes.Unimplemented) {
				Skip("Required bytes not supported")
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})
			Expect(vol.GetVolume().GetCapacityBytes()).To(BeNumerically(">=", TestVolumeSize(sc)))

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})
		It("should not fail when requesting to create a volume with already existing name and same capacity.", func() {

			By("creating a volume")
			name := UniqueString("sanity-controller-create-twice")
			size := TestVolumeSize(sc)

			vol1, err := c.CreateVolume(
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
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: size,
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(vol1).NotTo(BeNil())
			Expect(vol1.GetVolume()).NotTo(BeNil())
			Expect(vol1.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol1.GetVolume().GetVolumeId()})
			Expect(vol1.GetVolume().GetCapacityBytes()).To(BeNumerically(">=", size))

			vol2, err := c.CreateVolume(
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
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: size,
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(vol2).NotTo(BeNil())
			Expect(vol2.GetVolume()).NotTo(BeNil())
			Expect(vol2.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			Expect(vol2.GetVolume().GetCapacityBytes()).To(BeNumerically(">=", size))
			Expect(vol1.GetVolume().GetVolumeId()).To(Equal(vol2.GetVolume().GetVolumeId()))

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol1.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})
		It("should fail when requesting to create a volume with already existing name and different capacity.", func() {

			By("creating a volume")
			name := UniqueString("sanity-controller-create-twice-different")
			size1 := TestVolumeSize(sc)

			vol1, err := c.CreateVolume(
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
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: size1,
						LimitBytes:    size1,
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(vol1).NotTo(BeNil())
			Expect(vol1.GetVolume()).NotTo(BeNil())
			Expect(vol1.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol1.GetVolume().GetVolumeId()})
			size2 := 2 * TestVolumeSize(sc)

			_, err = c.CreateVolume(
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
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: size2,
						LimitBytes:    size2,
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).To(HaveOccurred())
			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.AlreadyExists))

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol1.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})

		It("should not fail when creating volume with maximum-length name", func() {

			nameBytes := make([]byte, MaxNameLength)
			for i := 0; i < MaxNameLength; i++ {
				nameBytes[i] = 'a'
			}
			name := string(nameBytes)
			By("creating a volume")
			size := TestVolumeSize(sc)

			vol, err := c.CreateVolume(
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
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: size,
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})
			Expect(vol.GetVolume().GetCapacityBytes()).To(BeNumerically(">=", size))

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})

		It("should create volume from an existing source snapshot", func() {
			if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT) {
				Skip("Snapshot not supported")
			}

			By("creating a volume")
			vol1Name := UniqueString("sanity-controller-source-vol")
			vol1Req := MakeCreateVolumeReq(sc, vol1Name)
			volume1, err := c.CreateVolume(context.Background(), vol1Req)
			Expect(err).NotTo(HaveOccurred())

			By("creating a snapshot")
			snapName := UniqueString("sanity-controller-snap-from-vol")
			snapReq := MakeCreateSnapshotReq(sc, snapName, volume1.GetVolume().GetVolumeId(), nil)
			snap, err := c.CreateSnapshot(context.Background(), snapReq)
			Expect(err).NotTo(HaveOccurred())
			Expect(snap).NotTo(BeNil())
			verifySnapshotInfo(snap.GetSnapshot())

			By("creating a volume from source snapshot")
			vol2Name := UniqueString("sanity-controller-vol-from-snap")
			vol2Req := MakeCreateVolumeReq(sc, vol2Name)
			vol2Req.VolumeContentSource = &csi.VolumeContentSource{
				Type: &csi.VolumeContentSource_Snapshot{
					Snapshot: &csi.VolumeContentSource_SnapshotSource{
						SnapshotId: snap.GetSnapshot().GetSnapshotId(),
					},
				},
			}
			volume2, err := c.CreateVolume(context.Background(), vol2Req)
			Expect(err).NotTo(HaveOccurred())

			By("cleaning up deleting the volume created from snapshot")
			delVol2Req := MakeDeleteVolumeReq(sc, volume2.GetVolume().GetVolumeId())
			_, err = c.DeleteVolume(context.Background(), delVol2Req)
			Expect(err).NotTo(HaveOccurred())

			By("cleaning up deleting the snapshot")
			delSnapReq := MakeDeleteSnapshotReq(sc, snap.GetSnapshot().GetSnapshotId())
			_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
			Expect(err).NotTo(HaveOccurred())

			By("cleaning up deleting the source volume")
			delVol1Req := MakeDeleteVolumeReq(sc, volume1.GetVolume().GetVolumeId())
			_, err = c.DeleteVolume(context.Background(), delVol1Req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail when the volume source snapshot is not found", func() {
			if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT) {
				Skip("Snapshot not supported")
			}

			By("creating a volume from source snapshot")
			volName := UniqueString("sanity-controller-vol-from-snap")
			volReq := MakeCreateVolumeReq(sc, volName)
			volReq.VolumeContentSource = &csi.VolumeContentSource{
				Type: &csi.VolumeContentSource_Snapshot{
					Snapshot: &csi.VolumeContentSource_SnapshotSource{
						SnapshotId: "non-existing-snapshot-id",
					},
				},
			}
			_, err := c.CreateVolume(context.Background(), volReq)
			Expect(err).To(HaveOccurred())
			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.NotFound))
		})

		It("should create volume from an existing source volume", func() {
			if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_CLONE_VOLUME) {
				Skip("Volume Cloning not supported")
			}

			By("creating a volume")
			vol1Name := UniqueString("sanity-controller-source-vol")
			vol1Req := MakeCreateVolumeReq(sc, vol1Name)
			volume1, err := c.CreateVolume(context.Background(), vol1Req)
			Expect(err).NotTo(HaveOccurred())

			By("creating a volume from source volume")
			vol2Name := UniqueString("sanity-controller-vol-from-vol")
			vol2Req := MakeCreateVolumeReq(sc, vol2Name)
			vol2Req.VolumeContentSource = &csi.VolumeContentSource{
				Type: &csi.VolumeContentSource_Volume{
					Volume: &csi.VolumeContentSource_VolumeSource{
						VolumeId: volume1.GetVolume().GetVolumeId(),
					},
				},
			}
			volume2, err := c.CreateVolume(context.Background(), vol2Req)
			Expect(err).NotTo(HaveOccurred())

			By("cleaning up deleting the volume created from source volume")
			delVol2Req := MakeDeleteVolumeReq(sc, volume2.GetVolume().GetVolumeId())
			_, err = c.DeleteVolume(context.Background(), delVol2Req)
			Expect(err).NotTo(HaveOccurred())

			By("cleaning up deleting the source volume")
			delVol1Req := MakeDeleteVolumeReq(sc, volume1.GetVolume().GetVolumeId())
			_, err = c.DeleteVolume(context.Background(), delVol1Req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail when the volume source volume is not found", func() {
			if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_CLONE_VOLUME) {
				Skip("Volume Cloning not supported")
			}

			By("creating a volume from source snapshot")
			volName := UniqueString("sanity-controller-vol-from-snap")
			volReq := MakeCreateVolumeReq(sc, volName)
			volReq.VolumeContentSource = &csi.VolumeContentSource{
				Type: &csi.VolumeContentSource_Volume{
					Volume: &csi.VolumeContentSource_VolumeSource{
						VolumeId: sc.Config.IDGen.GenerateUniqueValidVolumeID(),
					},
				},
			}
			_, err := c.CreateVolume(context.Background(), volReq)
			Expect(err).To(HaveOccurred())
			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.NotFound))
		})
	})

	Describe("DeleteVolume", func() {
		BeforeEach(func() {
			if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME) {
				Skip("DeleteVolume not supported")
			}
		})

		It("should fail when no volume id is provided", func() {

			_, err := c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					Secrets: sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should succeed when an invalid volume id is used", func() {

			_, err := c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: sc.Config.IDGen.GenerateInvalidVolumeID(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return appropriate values (no optional values added)", func() {

			// Create Volume First
			By("creating a volume")
			name := UniqueString("sanity-controller-create-appropriate")

			vol, err := c.CreateVolume(
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
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})

			// Delete Volume
			By("deleting a volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})
	})

	Describe("ValidateVolumeCapabilities", func() {
		It("should fail when no volume id is provided", func() {

			_, err := c.ValidateVolumeCapabilities(
				context.Background(),
				&csi.ValidateVolumeCapabilitiesRequest{
					Secrets: sc.Secrets.ControllerValidateVolumeCapabilitiesSecret,
				})
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail when no volume capabilities are provided", func() {

			// Create Volume First
			By("creating a single node writer volume")
			name := UniqueString("sanity-controller-validate-nocaps")

			vol, err := c.CreateVolume(
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
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})

			_, err = c.ValidateVolumeCapabilities(
				context.Background(),
				&csi.ValidateVolumeCapabilitiesRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.ControllerValidateVolumeCapabilitiesSecret,
				})
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})

		It("should return appropriate values (no optional values added)", func() {

			// Create Volume First
			By("creating a single node writer volume")
			name := UniqueString("sanity-controller-validate")

			vol, err := c.CreateVolume(
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
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})

			// ValidateVolumeCapabilities
			By("validating volume capabilities")
			valivolcap, err := c.ValidateVolumeCapabilities(
				context.Background(),
				&csi.ValidateVolumeCapabilitiesRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
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
					Secrets: sc.Secrets.ControllerValidateVolumeCapabilitiesSecret,
				})
			Expect(err).NotTo(HaveOccurred())
			Expect(valivolcap).NotTo(BeNil())

			// If confirmation is provided then it is REQUIRED to provide
			// the volume capabilities
			if valivolcap.GetConfirmed() != nil {
				Expect(valivolcap.GetConfirmed().GetVolumeCapabilities()).NotTo(BeEmpty())
			}

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})

		It("should fail when the requested volume does not exist", func() {

			_, err := c.ValidateVolumeCapabilities(
				context.Background(),
				&csi.ValidateVolumeCapabilitiesRequest{
					VolumeId: sc.Config.IDGen.GenerateUniqueValidVolumeID(),
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
					Secrets: sc.Secrets.ControllerValidateVolumeCapabilitiesSecret,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.NotFound))
		})
	})

	Describe("ControllerPublishVolume", func() {
		BeforeEach(func() {
			if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME) {
				Skip("ControllerPublishVolume not supported")
			}
		})

		It("should fail when no volume id is provided", func() {

			_, err := c.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					Secrets: sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail when no node id is provided", func() {

			_, err := c.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					VolumeId: sc.Config.IDGen.GenerateUniqueValidVolumeID(),
					Secrets:  sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail when no volume capability is provided", func() {

			_, err := c.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					VolumeId: sc.Config.IDGen.GenerateUniqueValidVolumeID(),
					NodeId:   sc.Config.IDGen.GenerateUniqueValidNodeID(),
					Secrets:  sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		// CSI spec poses no specific requirements for the cluster/storage setups that a SP MUST support. To perform
		// meaningful checks the following test assumes that topology-aware provisioning on a single node setup is supported
		It("should return appropriate values (no optional values added)", func() {

			By("getting node information")
			ni, err := n.NodeGetInfo(
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
			name := UniqueString("sanity-controller-publish")

			vol, err := c.CreateVolume(
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
					Secrets:                   sc.Secrets.CreateVolumeSecret,
					Parameters:                sc.Config.TestVolumeParameters,
					AccessibilityRequirements: accReqs,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})

			// ControllerPublishVolume
			By("calling controllerpublish on that volume")

			conpubvol, err := c.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					NodeId:   ni.GetNodeId(),
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
					Readonly: false,
					Secrets:  sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId(), NodeID: ni.GetNodeId()})
			Expect(conpubvol).NotTo(BeNil())

			By("cleaning up unpublishing the volume")

			conunpubvol, err := c.ControllerUnpublishVolume(
				context.Background(),
				&csi.ControllerUnpublishVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					// NodeID is optional in ControllerUnpublishVolume
					NodeId:  ni.GetNodeId(),
					Secrets: sc.Secrets.ControllerUnpublishVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(conunpubvol).NotTo(BeNil())

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})

		It("should fail when publishing more volumes than the node max attach limit", func() {
			if !sc.Config.TestNodeVolumeAttachLimit {
				Skip("testnodevolumeattachlimit not enabled")
			}

			By("getting node info")
			nodeInfo, err := n.NodeGetInfo(
				context.Background(),
				&csi.NodeGetInfoRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(nodeInfo).NotTo(BeNil())

			if nodeInfo.MaxVolumesPerNode <= 0 {
				Skip("No MaxVolumesPerNode")
			}

			nid := nodeInfo.GetNodeId()
			Expect(nid).NotTo(BeEmpty())

			// Store the volume name and volume ID for later cleanup.
			createdVols := map[string]string{}
			By("creating volumes")
			for i := int64(0); i < nodeInfo.MaxVolumesPerNode; i++ {
				name := UniqueString(fmt.Sprintf("sanity-max-attach-limit-vol-%d", i))
				volID, err := CreateAndControllerPublishVolume(sc, c, name, nid)
				Expect(err).NotTo(HaveOccurred())
				cl.RegisterVolume(name, VolumeInfo{VolumeID: volID, NodeID: nid})
				createdVols[name] = volID
			}

			extraVolName := UniqueString("sanity-max-attach-limit-vol+1")
			_, err = CreateAndControllerPublishVolume(sc, c, extraVolName, nid)
			Expect(err).To(HaveOccurred())

			By("cleaning up")
			for volName, volID := range createdVols {
				err = ControllerUnpublishAndDeleteVolume(sc, c, volID, nid)
				Expect(err).NotTo(HaveOccurred())
				cl.UnregisterVolume(volName)
			}
		})

		It("should fail when the volume does not exist", func() {

			By("calling controller publish on a non-existent volume")

			conpubvol, err := c.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					VolumeId: sc.Config.IDGen.GenerateUniqueValidVolumeID(),
					NodeId:   sc.Config.IDGen.GenerateUniqueValidNodeID(),
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
					Readonly: false,
					Secrets:  sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())
			Expect(conpubvol).To(BeNil())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.NotFound))
		})

		It("should fail when the node does not exist", func() {

			// Create Volume First
			By("creating a single node writer volume")
			name := UniqueString("sanity-controller-wrong-node")

			vol, err := c.CreateVolume(
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
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})

			// ControllerPublishVolume
			By("calling controllerpublish on that volume")

			conpubvol, err := c.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					NodeId:   sc.Config.IDGen.GenerateUniqueValidNodeID(),
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
					Readonly: false,
					Secrets:  sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())
			Expect(conpubvol).To(BeNil())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.NotFound))

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})

		It("should fail when the volume is already published but is incompatible", func() {
			if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_PUBLISH_READONLY) {
				Skip("ControllerPublishVolume.readonly field not supported")
			}

			// Create Volume First
			By("creating a single node writer volume")
			name := UniqueString("sanity-controller-published-incompatible")

			vol, err := c.CreateVolume(
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
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})

			By("getting a node id")
			nid, err := n.NodeGetInfo(
				context.Background(),
				&csi.NodeGetInfoRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(nid).NotTo(BeNil())
			Expect(nid.GetNodeId()).NotTo(BeEmpty())

			// ControllerPublishVolume
			By("calling controllerpublish on that volume")

			pubReq := &csi.ControllerPublishVolumeRequest{
				VolumeId: vol.GetVolume().GetVolumeId(),
				NodeId:   nid.GetNodeId(),
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				Readonly: false,
				Secrets:  sc.Secrets.ControllerPublishVolumeSecret,
			}

			conpubvol, err := c.ControllerPublishVolume(context.Background(), pubReq)
			Expect(err).NotTo(HaveOccurred())
			Expect(conpubvol).NotTo(BeNil())

			// Publish again with different attributes.
			pubReq.Readonly = true

			conpubvol, err = c.ControllerPublishVolume(context.Background(), pubReq)
			Expect(err).To(HaveOccurred())
			Expect(conpubvol).To(BeNil())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.AlreadyExists))

			By("cleaning up unpublishing the volume")

			conunpubvol, err := c.ControllerUnpublishVolume(
				context.Background(),
				&csi.ControllerUnpublishVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					// NodeID is optional in ControllerUnpublishVolume
					NodeId:  nid.GetNodeId(),
					Secrets: sc.Secrets.ControllerUnpublishVolumeSecret,
				},
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(conunpubvol).NotTo(BeNil())

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})
	})

	Describe("ControllerUnpublishVolume", func() {
		BeforeEach(func() {
			if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME) {
				Skip("ControllerUnpublishVolume not supported")
			}
		})

		It("should fail when no volume id is provided", func() {

			_, err := c.ControllerUnpublishVolume(
				context.Background(),
				&csi.ControllerUnpublishVolumeRequest{
					Secrets: sc.Secrets.ControllerUnpublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		// CSI spec poses no specific requirements for the cluster/storage setups that a SP MUST support. To perform
		// meaningful checks the following test assumes that topology-aware provisioning on a single node setup is supported
		It("should return appropriate values (no optional values added)", func() {

			// Create Volume First
			By("creating a single node writer volume")
			name := UniqueString("sanity-controller-unpublish")

			By("getting node information")
			ni, err := n.NodeGetInfo(
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

			vol, err := c.CreateVolume(
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
					Secrets:                   sc.Secrets.CreateVolumeSecret,
					Parameters:                sc.Config.TestVolumeParameters,
					AccessibilityRequirements: accReqs,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})

			// ControllerPublishVolume
			By("calling controllerpublish on that volume")

			conpubvol, err := c.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					NodeId:   ni.GetNodeId(),
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
					Readonly: false,
					Secrets:  sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId(), NodeID: ni.GetNodeId()})
			Expect(conpubvol).NotTo(BeNil())

			// ControllerUnpublishVolume
			By("calling controllerunpublish on that volume")

			conunpubvol, err := c.ControllerUnpublishVolume(
				context.Background(),
				&csi.ControllerUnpublishVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					// NodeID is optional in ControllerUnpublishVolume
					NodeId:  ni.GetNodeId(),
					Secrets: sc.Secrets.ControllerUnpublishVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(conunpubvol).NotTo(BeNil())

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})
	})
})

var _ = DescribeSanity("ListSnapshots [Controller Server]", func(sc *SanityContext) {
	var (
		c csi.ControllerClient
	)

	BeforeEach(func() {
		c = csi.NewControllerClient(sc.ControllerConn)

		if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS) {
			Skip("ListSnapshots not supported")
		}
	})

	It("should return appropriate values (no optional values added)", func() {
		snapshots, err := c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())

		for _, snapshot := range snapshots.GetEntries() {
			verifySnapshotInfo(snapshot.GetSnapshot())
		}
	})

	It("should return snapshots that match the specified snapshot id", func() {

		By("creating a volume")
		volReq := MakeCreateVolumeReq(sc, "listSnapshots-volume-1")
		volume, err := c.CreateVolume(context.Background(), volReq)
		Expect(err).NotTo(HaveOccurred())

		By("creating a snapshot")
		snapshotReq := MakeCreateSnapshotReq(sc, "listSnapshots-snapshot-1", volume.GetVolume().GetVolumeId(), nil)
		snapshot, err := c.CreateSnapshot(context.Background(), snapshotReq)
		Expect(err).NotTo(HaveOccurred())

		snapshots, err := c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{SnapshotId: snapshot.GetSnapshot().GetSnapshotId()})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		Expect(len(snapshots.GetEntries())).To(BeNumerically("==", 1))
		verifySnapshotInfo(snapshots.GetEntries()[0].GetSnapshot())
		Expect(snapshots.GetEntries()[0].GetSnapshot().GetSnapshotId()).To(Equal(snapshot.GetSnapshot().GetSnapshotId()))

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snapshot.GetSnapshot().GetSnapshotId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetVolumeId())
		_, err = c.DeleteVolume(context.Background(), delVolReq)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return empty when the specified snapshot id does not exist", func() {

		snapshots, err := c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{SnapshotId: "none-exist-id"})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		Expect(snapshots.GetEntries()).To(BeEmpty())
	})

	It("should return snapshots that match the specified source volume id)", func() {

		By("creating a volume")
		volReq := MakeCreateVolumeReq(sc, "listSnapshots-volume-2")
		volume, err := c.CreateVolume(context.Background(), volReq)
		Expect(err).NotTo(HaveOccurred())

		By("creating a snapshot")
		snapshotReq := MakeCreateSnapshotReq(sc, "listSnapshots-snapshot-2", volume.GetVolume().GetVolumeId(), nil)
		snapshot, err := c.CreateSnapshot(context.Background(), snapshotReq)
		Expect(err).NotTo(HaveOccurred())

		snapshots, err := c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{SourceVolumeId: snapshot.GetSnapshot().GetSourceVolumeId()})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		for _, snap := range snapshots.GetEntries() {
			verifySnapshotInfo(snap.GetSnapshot())
			Expect(snap.GetSnapshot().GetSourceVolumeId()).To(Equal(snapshot.GetSnapshot().GetSourceVolumeId()))
		}

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snapshot.GetSnapshot().GetSnapshotId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetVolumeId())
		_, err = c.DeleteVolume(context.Background(), delVolReq)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return empty when the specified source volume id does not exist", func() {

		snapshots, err := c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{SourceVolumeId: sc.Config.IDGen.GenerateUniqueValidVolumeID()})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		Expect(snapshots.GetEntries()).To(BeEmpty())
	})

	It("check the presence of new snapshots in the snapshot list", func() {
		// List Snapshots before creating new snapshots.
		snapshots, err := c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())

		totalSnapshots := len(snapshots.GetEntries())

		By("creating a volume")
		volReq := MakeCreateVolumeReq(sc, "listSnapshots-volume-3")
		volume, err := c.CreateVolume(context.Background(), volReq)
		Expect(err).NotTo(HaveOccurred())

		By("creating a snapshot")
		snapReq := MakeCreateSnapshotReq(sc, "listSnapshots-snapshot-3", volume.GetVolume().GetVolumeId(), nil)
		snapshot, err := c.CreateSnapshot(context.Background(), snapReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshot).NotTo(BeNil())
		verifySnapshotInfo(snapshot.GetSnapshot())

		snapshots, err = c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		Expect(len(snapshots.GetEntries())).To(Equal(totalSnapshots + 1))

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snapshot.GetSnapshot().GetSnapshotId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetVolumeId())
		_, err = c.DeleteVolume(context.Background(), delVolReq)
		Expect(err).NotTo(HaveOccurred())

		// List snapshots and check if the deleted snapshot exists in the snapshot list.
		snapshots, err = c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		Expect(len(snapshots.GetEntries())).To(Equal(totalSnapshots))
	})

	It("should return next token when a limited number of entries are requested", func() {
		// minSnapshotCount is the minimum number of snapshots expected to exist,
		// based on which paginated snapshot listing is performed.
		minSnapshotCount := 5
		// maxEntried is the maximum entries in list snapshot request.
		maxEntries := 2
		// currentTotalVols is the total number of volumes at a given time. It
		// is used to verify that all the snapshots have been listed.
		currentTotalSnapshots := 0

		// Get the number of existing volumes.
		snapshots, err := c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())

		initialTotalSnapshots := len(snapshots.GetEntries())
		currentTotalSnapshots = initialTotalSnapshots

		createVols := make([]*csi.Volume, 0)
		createSnapshots := make([]*csi.Snapshot, 0)

		// Ensure minimum minVolCount volumes exist.
		if initialTotalSnapshots < minSnapshotCount {

			By("creating required new volumes")
			requiredSnapshots := minSnapshotCount - initialTotalSnapshots

			for i := 1; i <= requiredSnapshots; i++ {
				volReq := MakeCreateVolumeReq(sc, "volume"+strconv.Itoa(i))
				volume, err := c.CreateVolume(context.Background(), volReq)
				Expect(err).NotTo(HaveOccurred())
				Expect(volume).NotTo(BeNil())
				createVols = append(createVols, volume.GetVolume())

				snapReq := MakeCreateSnapshotReq(sc, "snapshot"+strconv.Itoa(i), volume.GetVolume().GetVolumeId(), nil)
				snapshot, err := c.CreateSnapshot(context.Background(), snapReq)
				Expect(err).NotTo(HaveOccurred())
				Expect(snapshot).NotTo(BeNil())
				verifySnapshotInfo(snapshot.GetSnapshot())
				createSnapshots = append(createSnapshots, snapshot.GetSnapshot())
			}

			// Update the current total snapshots count.
			currentTotalSnapshots += requiredSnapshots
		}

		// Request list snapshots with max entries maxEntries.
		snapshots, err = c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{
				MaxEntries: int32(maxEntries),
			})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())

		nextToken := snapshots.GetNextToken()

		Expect(len(snapshots.GetEntries())).To(Equal(maxEntries))

		// Request list snapshots with starting_token and no max entries.
		snapshots, err = c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{
				StartingToken: nextToken,
			})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())

		// Ensure that all the remaining entries are returned at once.
		Expect(len(snapshots.GetEntries())).To(Equal(currentTotalSnapshots - maxEntries))

		if initialTotalSnapshots < minSnapshotCount {

			By("cleaning up deleting the snapshots")

			for _, snap := range createSnapshots {
				delSnapReq := MakeDeleteSnapshotReq(sc, snap.GetSnapshotId())
				_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
				Expect(err).NotTo(HaveOccurred())
			}

			By("cleaning up deleting the volumes")

			for _, vol := range createVols {
				delVolReq := MakeDeleteVolumeReq(sc, vol.GetVolumeId())
				_, err = c.DeleteVolume(context.Background(), delVolReq)
				Expect(err).NotTo(HaveOccurred())
			}
		}
	})

})

var _ = DescribeSanity("DeleteSnapshot [Controller Server]", func(sc *SanityContext) {
	var (
		c csi.ControllerClient
	)

	BeforeEach(func() {
		c = csi.NewControllerClient(sc.ControllerConn)

		if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT) {
			Skip("DeleteSnapshot not supported")
		}
	})

	It("should fail when no snapshot id is provided", func() {

		req := &csi.DeleteSnapshotRequest{}

		if sc.Secrets != nil {
			req.Secrets = sc.Secrets.DeleteSnapshotSecret
		}

		_, err := c.DeleteSnapshot(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should succeed when an invalid snapshot id is used", func() {

		req := MakeDeleteSnapshotReq(sc, "reallyfakesnapshotid")
		_, err := c.DeleteSnapshot(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return appropriate values (no optional values added)", func() {

		By("creating a volume")
		volReq := MakeCreateVolumeReq(sc, "DeleteSnapshot-volume-1")
		volume, err := c.CreateVolume(context.Background(), volReq)
		Expect(err).NotTo(HaveOccurred())

		// Create Snapshot First
		By("creating a snapshot")
		snapshotReq := MakeCreateSnapshotReq(sc, "DeleteSnapshot-snapshot-1", volume.GetVolume().GetVolumeId(), nil)
		snapshot, err := c.CreateSnapshot(context.Background(), snapshotReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshot).NotTo(BeNil())
		verifySnapshotInfo(snapshot.GetSnapshot())

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snapshot.GetSnapshot().GetSnapshotId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetVolumeId())
		_, err = c.DeleteVolume(context.Background(), delVolReq)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = DescribeSanity("CreateSnapshot [Controller Server]", func(sc *SanityContext) {
	var (
		c csi.ControllerClient
	)

	BeforeEach(func() {
		c = csi.NewControllerClient(sc.ControllerConn)

		if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT) {
			Skip("CreateSnapshot not supported")
		}
	})

	It("should fail when no name is provided", func() {

		req := &csi.CreateSnapshotRequest{
			SourceVolumeId: "testId",
		}

		if sc.Secrets != nil {
			req.Secrets = sc.Secrets.CreateSnapshotSecret
		}

		_, err := c.CreateSnapshot(context.Background(), req)
		Expect(err).To(HaveOccurred())
		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should fail when no source volume id is provided", func() {

		req := &csi.CreateSnapshotRequest{
			Name: "name",
		}

		if sc.Secrets != nil {
			req.Secrets = sc.Secrets.CreateSnapshotSecret
		}

		_, err := c.CreateSnapshot(context.Background(), req)
		Expect(err).To(HaveOccurred())
		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should not fail when requesting to create a snapshot with already existing name and same SourceVolumeId.", func() {

		By("creating a volume")
		volReq := MakeCreateVolumeReq(sc, "CreateSnapshot-volume-1")
		volume, err := c.CreateVolume(context.Background(), volReq)
		Expect(err).NotTo(HaveOccurred())

		By("creating a snapshot")
		snapReq1 := MakeCreateSnapshotReq(sc, "CreateSnapshot-snapshot-1", volume.GetVolume().GetVolumeId(), nil)
		snap1, err := c.CreateSnapshot(context.Background(), snapReq1)
		Expect(err).NotTo(HaveOccurred())
		Expect(snap1).NotTo(BeNil())
		verifySnapshotInfo(snap1.GetSnapshot())

		snap2, err := c.CreateSnapshot(context.Background(), snapReq1)
		Expect(err).NotTo(HaveOccurred())
		Expect(snap2).NotTo(BeNil())
		verifySnapshotInfo(snap2.GetSnapshot())

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snap1.GetSnapshot().GetSnapshotId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetVolumeId())
		_, err = c.DeleteVolume(context.Background(), delVolReq)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should fail when requesting to create a snapshot with already existing name and different SourceVolumeId.", func() {

		By("creating a volume")
		volume, err := c.CreateVolume(context.Background(), MakeCreateVolumeReq(sc, "CreateSnapshot-volume-2"))
		Expect(err).ToNot(HaveOccurred())

		By("creating a snapshot with the created volume source id")
		req1 := MakeCreateSnapshotReq(sc, "CreateSnapshot-snapshot-2", volume.GetVolume().GetVolumeId(), nil)
		snap1, err := c.CreateSnapshot(context.Background(), req1)
		Expect(err).NotTo(HaveOccurred())
		Expect(snap1).NotTo(BeNil())
		verifySnapshotInfo(snap1.GetSnapshot())

		volume2, err := c.CreateVolume(context.Background(), MakeCreateVolumeReq(sc, "CreateSnapshot-volume-3"))
		Expect(err).ToNot(HaveOccurred())

		By("creating a snapshot with the same name but different volume source id")
		req2 := MakeCreateSnapshotReq(sc, "CreateSnapshot-snapshot-2", volume2.GetVolume().GetVolumeId(), nil)
		_, err = c.CreateSnapshot(context.Background(), req2)
		Expect(err).To(HaveOccurred())
		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.AlreadyExists))

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snap1.GetSnapshot().GetSnapshotId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetVolumeId())
		_, err = c.DeleteVolume(context.Background(), delVolReq)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should not fail when creating snapshot with maximum-length name", func() {

		By("creating a volume")
		volReq := MakeCreateVolumeReq(sc, "CreateSnapshot-volume-3")
		volume, err := c.CreateVolume(context.Background(), volReq)
		Expect(err).NotTo(HaveOccurred())

		nameBytes := make([]byte, MaxNameLength)
		for i := 0; i < MaxNameLength; i++ {
			nameBytes[i] = 'a'
		}
		name := string(nameBytes)

		By("creating a snapshot")
		snapReq1 := MakeCreateSnapshotReq(sc, name, volume.GetVolume().GetVolumeId(), nil)
		snap1, err := c.CreateSnapshot(context.Background(), snapReq1)
		Expect(err).NotTo(HaveOccurred())
		Expect(snap1).NotTo(BeNil())
		verifySnapshotInfo(snap1.GetSnapshot())

		snap2, err := c.CreateSnapshot(context.Background(), snapReq1)
		Expect(err).NotTo(HaveOccurred())
		Expect(snap2).NotTo(BeNil())
		verifySnapshotInfo(snap2.GetSnapshot())

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snap1.GetSnapshot().GetSnapshotId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetVolumeId())
		_, err = c.DeleteVolume(context.Background(), delVolReq)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = DescribeSanity("ExpandVolume [Controller Server]", func(sc *SanityContext) {
	var (
		c  csi.ControllerClient
		cl *Cleanup
	)

	BeforeEach(func() {
		c = csi.NewControllerClient(sc.ControllerConn)
		if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_EXPAND_VOLUME) {
			Skip("ControllerExpandVolume not supported")
		}
		cl = &Cleanup{
			ControllerClient: c,
			Context:          sc,
		}
	})
	AfterEach(func() {
		cl.DeleteVolumes()
	})
	It("should fail if no volume id is given", func() {
		expReq := &csi.ControllerExpandVolumeRequest{
			VolumeId: "",
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: TestVolumeExpandSize(sc),
			},
		}
		rsp, err := c.ControllerExpandVolume(context.Background(), expReq)
		Expect(err).To(HaveOccurred())
		Expect(rsp).To(BeNil())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should fail if no capacity range is given", func() {
		expReq := &csi.ControllerExpandVolumeRequest{
			VolumeId: "",
		}
		rsp, err := c.ControllerExpandVolume(context.Background(), expReq)
		Expect(err).To(HaveOccurred())
		Expect(rsp).To(BeNil())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should work", func() {

		By("creating a new volume")
		name := UniqueString("sanity-expand-volume")

		// Create a new volume.
		req := &csi.CreateVolumeRequest{
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
			Secrets: sc.Secrets.CreateVolumeSecret,
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: TestVolumeSize(sc),
			},
		}

		vol, err := c.CreateVolume(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(vol).NotTo(BeNil())
		Expect(vol.GetVolume()).NotTo(BeNil())
		Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
		cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})
		By("expanding the volume")
		expReq := &csi.ControllerExpandVolumeRequest{
			VolumeId: vol.GetVolume().GetVolumeId(),
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: TestVolumeExpandSize(sc),
			},
		}
		rsp, err := c.ControllerExpandVolume(context.Background(), expReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(rsp).NotTo(BeNil())
		Expect(rsp.GetCapacityBytes()).To(Equal(TestVolumeExpandSize(sc)))

		By("cleaning up deleting the volume")
		_, err = c.DeleteVolume(
			context.Background(),
			&csi.DeleteVolumeRequest{
				VolumeId: vol.GetVolume().GetVolumeId(),
				Secrets:  sc.Secrets.DeleteVolumeSecret,
			},
		)
		Expect(err).NotTo(HaveOccurred())
		cl.UnregisterVolume(name)
	})
})

func MakeCreateVolumeReq(sc *SanityContext, name string) *csi.CreateVolumeRequest {
	size1 := TestVolumeSize(sc)

	req := &csi.CreateVolumeRequest{
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
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: size1,
			LimitBytes:    size1,
		},
		Parameters: sc.Config.TestVolumeParameters,
	}

	if sc.Secrets != nil {
		req.Secrets = sc.Secrets.CreateVolumeSecret
	}

	return req
}

func MakeCreateSnapshotReq(sc *SanityContext, name, sourceVolumeId string, parameters map[string]string) *csi.CreateSnapshotRequest {
	req := &csi.CreateSnapshotRequest{
		Name:           name,
		SourceVolumeId: sourceVolumeId,
		Parameters:     parameters,
	}

	if sc.Secrets != nil {
		req.Secrets = sc.Secrets.CreateSnapshotSecret
	}

	return req
}

func MakeDeleteSnapshotReq(sc *SanityContext, id string) *csi.DeleteSnapshotRequest {
	delSnapReq := &csi.DeleteSnapshotRequest{
		SnapshotId: id,
	}

	if sc.Secrets != nil {
		delSnapReq.Secrets = sc.Secrets.DeleteSnapshotSecret
	}

	return delSnapReq
}

func MakeDeleteVolumeReq(sc *SanityContext, id string) *csi.DeleteVolumeRequest {
	delVolReq := &csi.DeleteVolumeRequest{
		VolumeId: id,
	}

	if sc.Secrets != nil {
		delVolReq.Secrets = sc.Secrets.DeleteVolumeSecret
	}

	return delVolReq
}

// MakeControllerPublishVolumeReq creates and returns a ControllerPublishVolumeRequest.
func MakeControllerPublishVolumeReq(sc *SanityContext, volID, nodeID string) *csi.ControllerPublishVolumeRequest {
	return &csi.ControllerPublishVolumeRequest{
		VolumeId: volID,
		NodeId:   nodeID,
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
		Readonly: false,
		Secrets:  sc.Secrets.ControllerPublishVolumeSecret,
	}
}

// MakeControllerUnpublishVolumeReq creates and returns a ControllerUnpublishVolumeRequest.
func MakeControllerUnpublishVolumeReq(sc *SanityContext, volID, nodeID string) *csi.ControllerUnpublishVolumeRequest {
	return &csi.ControllerUnpublishVolumeRequest{
		VolumeId: volID,
		NodeId:   nodeID,
		Secrets:  sc.Secrets.ControllerUnpublishVolumeSecret,
	}
}

// CreateAndControllerPublishVolume creates and controller publishes a volume given a volume name and node ID.
func CreateAndControllerPublishVolume(sc *SanityContext, c csi.ControllerClient, volName, nodeID string) (volID string, err error) {
	vol, err := c.CreateVolume(context.Background(), MakeCreateVolumeReq(sc, volName))
	Expect(err).NotTo(HaveOccurred())
	Expect(vol).NotTo(BeNil())
	Expect(vol.GetVolume()).NotTo(BeNil())
	Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())

	_, err = c.ControllerPublishVolume(
		context.Background(),
		MakeControllerPublishVolumeReq(sc, vol.GetVolume().GetVolumeId(), nodeID),
	)
	return vol.GetVolume().GetVolumeId(), err
}

// ControllerUnpublishAndDeleteVolume controller unpublishes and deletes a volume, given volume ID and node ID.
func ControllerUnpublishAndDeleteVolume(sc *SanityContext, c csi.ControllerClient, volID, nodeID string) error {
	_, err := c.ControllerUnpublishVolume(
		context.Background(),
		MakeControllerUnpublishVolumeReq(sc, volID, nodeID),
	)
	Expect(err).NotTo(HaveOccurred())

	_, err = c.DeleteVolume(
		context.Background(),
		MakeDeleteVolumeReq(sc, volID),
	)
	Expect(err).NotTo(HaveOccurred())
	return err
}
