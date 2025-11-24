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
	"regexp"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/container-storage-interface/spec/lib/go/csi"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = DescribeSanity("Identity Service", func(sc *TestContext) {
	var (
		cs map[string]csi.IdentityClient
	)

	BeforeEach(func() {
		cs = make(map[string]csi.IdentityClient)
		cs["Node Service"] = csi.NewIdentityClient(sc.Conn)
		if sc.ControllerConn != nil && sc.ControllerConn != sc.Conn {
			cs["Controller Service"] = csi.NewIdentityClient(sc.ControllerConn)
		}
	})

	Describe("GetPluginCapabilities", func() {
		It("should return appropriate capabilities", func() {
			for name, c := range cs {
				req := &csi.GetPluginCapabilitiesRequest{}
				res, err := c.GetPluginCapabilities(context.Background(), req)
				Expect(err).NotTo(HaveOccurred())
				Expect(res).NotTo(BeNil())

				By(fmt.Sprintf("[%s] checking successful response", name))
				for _, cap := range res.GetCapabilities() {
					switch cap.GetType().(type) {
					case *csi.PluginCapability_Service_:
						switch cap.GetService().GetType() {
						case csi.PluginCapability_Service_CONTROLLER_SERVICE:
						case csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS:
						case csi.PluginCapability_Service_GROUP_CONTROLLER_SERVICE:
						default:
							Fail(fmt.Sprintf("Unknown service: %v\n", cap.GetService().GetType()))
						}
					case *csi.PluginCapability_VolumeExpansion_:
						switch cap.GetVolumeExpansion().GetType() {
						case csi.PluginCapability_VolumeExpansion_ONLINE:
						case csi.PluginCapability_VolumeExpansion_OFFLINE:
						default:
							Fail(fmt.Sprintf("Unknown volume expansion mode: %v\n", cap.GetVolumeExpansion().GetType()))
						}
					default:
						Fail(fmt.Sprintf("Unknown capability: %v\n", cap.GetType()))
					}
				}
			}
		})

	})

	Describe("Probe", func() {
		It("should return appropriate information", func() {
			for name, c := range cs {
				req := &csi.ProbeRequest{}
				res, err := c.Probe(context.Background(), req)
				Expect(err).NotTo(HaveOccurred())
				Expect(res).NotTo(BeNil())

				By(fmt.Sprintf("[%s] verifying return status", name))
				serverError, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(serverError.Code() == codes.FailedPrecondition ||
					serverError.Code() == codes.OK).To(BeTrue(), "unexpected error: %s", serverError.Message())

				if res.GetReady() != nil {
					Expect(res.GetReady().GetValue() || !res.GetReady().GetValue()).To(BeTrue())
				}
			}
		})
	})

	Describe("GetPluginInfo", func() {
		It("should return appropriate information", func() {
			for name, c := range cs {
				req := &csi.GetPluginInfoRequest{}
				res, err := c.GetPluginInfo(context.Background(), req)
				Expect(err).NotTo(HaveOccurred())
				Expect(res).NotTo(BeNil())

				By(fmt.Sprintf("[%s] verifying name size and characters", name))
				Expect(res.GetName()).ToNot(HaveLen(0))
				Expect(len(res.GetName())).To(BeNumerically("<=", 63))
				Expect(regexp.
					MustCompile(`^[a-zA-Z][A-Za-z0-9-\\.\\_]{0,61}[a-zA-Z]$`).
					MatchString(res.GetName())).To(BeTrue())
			}
		})
	})
})
