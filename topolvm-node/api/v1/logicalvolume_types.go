/*

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

package v1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LogicalVolumeSpec defines the desired state of LogicalVolume
type LogicalVolumeSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Name     string            `json:"name"`
	NodeName string            `json:"nodeName"`
	Size     resource.Quantity `json:"size"`
}

// LogicalVolumeStatus defines the observed state of LogicalVolume
type LogicalVolumeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	VolumeID string `json:"volumeID"`
	Phase    string `json:"phase"`
	Message  string `json:"message"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// LogicalVolume is the Schema for the logicalvolumes API
type LogicalVolume struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LogicalVolumeSpec   `json:"spec,omitempty"`
	Status LogicalVolumeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LogicalVolumeList contains a list of LogicalVolume
type LogicalVolumeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LogicalVolume `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LogicalVolume{}, &LogicalVolumeList{})
}
