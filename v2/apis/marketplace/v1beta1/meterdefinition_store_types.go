/*
Copyright 2020 IBM Co..

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

package v1beta1

import (
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MeterDefinitionStore defines the meter workloads used to enable pay for
// use billing.
// +kubebuilder:object:root=true
//
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:path=meterdefinitionstores,scope=Namespaced
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Meterdefinition Stores"
// +genclient
type MeterdefinitionStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MeterdefinitionStoreSpec   `json:"spec,omitempty"`
	Status MeterdefinitionStoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MeterdefinitionStoreList contains a list of MeterdefinitionStores
type MeterdefinitionStoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MeterdefinitionStore `json:"items"`
}

// x defines ...
// +k8s:openapi-gen=true
type MeterdefinitionStoreSpec struct {
	// Group defines the operator group of the meter
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:text"
	Group string `json:"group"`
	
	// Entry defines an entry to the meterdefinitionstore
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:text"
	Entry EntryMapping `json:"entry"`
	
}

type EntryMapping map[string]EntryKey

type EntryKey struct {
	Version string `json:"version"`
	MeterDefinitionList []MeterDefinitionList `json:"meterdefinitions"`
}

// MeterdefinitionStoreStatus defines the observed state of MeterdefinitionStore
// +k8s:openapi-gen=true
// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
type MeterdefinitionStoreStatus struct {
	// Conditions represent the latest available observations of an object's state
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.x-descriptors="urn:alm:descriptor:io.kubernetes.conditions"
	// +optional
	Conditions status.Conditions `json:"conditions,omitempty"`

}


