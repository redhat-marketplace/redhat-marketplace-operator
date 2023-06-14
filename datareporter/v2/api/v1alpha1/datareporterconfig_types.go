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
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DataReporterConfigSpec defines the desired state of DataReporterConfig
type DataReporterConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	UserConfigs []UserConfig `json:"userConfig,omitempty"`
}

type UserConfig struct {
	// Required.
	UserName string `json:"userName,omitempty"`
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}

// DataReporterConfigStatus defines the observed state of DataReporterConfig
type DataReporterConfigStatus struct {
	// Conditions represent the latest available observations of an object's stateconfig
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:io.kubernetes.conditions"
	// +optional
	Conditions status.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DataReporterConfig is the Schema for the datareporterconfigs API
type DataReporterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DataReporterConfigSpec   `json:"spec,omitempty"`
	Status DataReporterConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DataReporterConfigList contains a list of DataReporterConfig
type DataReporterConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DataReporterConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DataReporterConfig{}, &DataReporterConfigList{})
}

const (

	// license not accepted condition, operator will not process events
	ConditionNoLicense       status.ConditionType   = "NoLicense"
	ReasonLicenseNotAccepted status.ConditionReason = "LicenseNotAccepted"

	// problem with data upload to data service
	ConditionUploadFailure status.ConditionType   = "UploadFailed"
	ReasonUploadFailed     status.ConditionReason = "UploadFailed"

	// unable to connect to data service
	ConditionConnectionFailure status.ConditionType   = "DataServiceConnectionFailed"
	ReasonConnectionFailure    status.ConditionReason = "DataServiceConnectionFailed"
)
