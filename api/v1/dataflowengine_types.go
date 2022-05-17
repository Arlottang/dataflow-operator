/*
Copyright 2022.

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
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DataflowEngineSpec defines the desired state of DataflowEngine
type DataflowEngineSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	//FrameStandalone FrameStandaloneSpec `json:"frameStandalone"`

	Name string `json:"name"`

	Image string `json:"image,omitempty"`

	Ports int32 `json:"ports"`
}

// DataflowEngineStatus defines the observed state of DataflowEngine
type DataflowEngineStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DataflowEngine is the Schema for the dataflowengines API
type DataflowEngine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DataflowEngineSpec   `json:"spec,omitempty"`
	Status DataflowEngineStatus `json:"status,omitempty"`
}

func (in *DataflowEngine) String() string {
	return fmt.Sprintf("Namespance [%s], Name [%s], Image [%s], FrameStandalonePort [%d]",
		in.Namespace,
		in.Spec.Name,
		in.Spec.Image,
		in.Spec.Ports)
}

//+kubebuilder:object:root=true

// DataflowEngineList contains a list of DataflowEngine
type DataflowEngineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DataflowEngine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DataflowEngine{}, &DataflowEngineList{})
}
