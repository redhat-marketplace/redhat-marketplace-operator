/*
Copyright 2023 IBM Co..

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ComponentConfigSpec defines the desired state of ComponentConfig
type ComponentConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of ComponentConfig. Edit componentconfig_types.go to remove/update
	Timeout string `json:"timeout,omitempty"`
}

// ComponentConfigStatus defines the observed state of ComponentConfig
type ComponentConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ComponentConfig is the Schema for the componentconfigs API
type ComponentConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentConfigSpec   `json:"spec,omitempty"`
	Status ComponentConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ComponentConfigList contains a list of ComponentConfig
type ComponentConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComponentConfig{}, &ComponentConfigList{})
}
