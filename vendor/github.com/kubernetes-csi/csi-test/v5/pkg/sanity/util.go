/*
Copyright 2019 The Kubernetes Authors.

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
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
)

// IDGenerator generates valid and invalid Volume and Node IDs to be used in
// tests
type IDGenerator interface {
	// GenerateUniqueValidVolumeID must generate a unique Volume ID that the CSI
	// Driver considers in valid form
	GenerateUniqueValidVolumeID() string

	// GenerateInvalidVolumeID must output a Volume ID that the CSI Driver MAY
	// consider invalid. Some drivers may not have requirements on IDs in which
	// case this method should output any non-empty ID
	GenerateInvalidVolumeID() string

	// GenerateUniqueValidNodeID must generate a unique Node ID that the CSI
	// Driver considers in valid form
	GenerateUniqueValidNodeID() string

	// GenerateInvalidNodeID must output a Node ID that the CSI Driver MAY
	// consider invalid. Some drivers may not have requirements on IDs in which
	// case this method should output any non-empty ID
	GenerateInvalidNodeID() string
}

var _ IDGenerator = &DefaultIDGenerator{}

type DefaultIDGenerator struct {
}

func (d DefaultIDGenerator) GenerateUniqueValidVolumeID() string {
	return fmt.Sprintf("fake-vol-id-%s", uuid.New().String()[:10])
}

func (d DefaultIDGenerator) GenerateInvalidVolumeID() string {
	return "fake-vol-id"
}

func (d DefaultIDGenerator) GenerateUniqueValidNodeID() string {
	return fmt.Sprintf("fake-node-id-%s", uuid.New().String()[:10])
}

func (d DefaultIDGenerator) GenerateInvalidNodeID() string {
	return "fake-node-id"
}

// ExpectErrorCode confirms that the correct error code was returned
func ExpectErrorCode(response any, err error, code codes.Code) {
	Expect(err).To(HaveOccurred())
	Expect(response).To(BeNil())

	serverError, ok := status.FromError(err)
	Expect(ok).To(BeTrue())
	Expect(serverError.Code()).To(Equal(code), "unexpected error: %s", serverError.Message())
}
