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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DataReporterConfigSpec defines the desired state of DataReporterConfig
type DataReporterConfigSpec struct {
	// +optional
	UserConfigs []UserConfig `json:"userConfig,omitempty"`
	// DataFilter to match incoming event payload against
	// The first DataFilter match in the array based on the Selector will be applied
	// +optional
	DataFilters []DataFilter `json:"dataFilters,omitempty"`
}

// UserConfig defines additional metadata added to a specified users report
type UserConfig struct {
	// Required.
	UserName string `json:"userName,omitempty"`
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}

// DataFilter defines json transformation and alternate event payload destinations based on selector criteria
// No Selector indicates match all
type DataFilter struct {
	// +optional
	Selector Selector `json:"selector,omitempty"`
	// +optional
	ManifestType string `json:"manifestType,omitempty"`
	// +optional
	Transformer Transformer `json:"transformer,omitempty"`
	// ConfirmDelivery determines the processor return code behavior, and behavior of event reports sent to data-service
	// If ConfirmDelivery is true, a 200 will be returned to the sender after all deliveries to flagged Destinations are successful
	// Events will not be accumulated to buffer, each report sent to data-service will contain 1 event
	// If ConfirmDelivery is false for all Destinations, a 200 will be returned to the sender once the event is confirmed as valid json
	// Events will be accumulated, each report sent to data-service will contain N events
	// +optional
	ConfirmDelivery bool `json:"confirmDelivery,omitempty"`
	// +optional
	AltDestinations []Destination `json:"altDestinations,omitempty"`
}

// Selector defines criteria for matching incoming event payload
type Selector struct {
	// matchExpressions is a list of jsonpath expressions
	// to match the selector, all jsonpath expressions must produce a result (AND)
	// +optional
	MatchExpressions []string `json:"matchExpressions,omitempty"`

	// matchUsers is a list of users that the dataFilter applies to.
	// If matchUsers is not specified, the dataFilter applies to all users
	// +optional
	MatchUsers []string `json:"matchUsers,omitempty"`
}

// Transformer defines the type of transformer to use, and where to load the transformation configuration from
type Transformer struct {
	// type is the transformation engine use
	// supported types: kazaam
	TransformerType string `json:"type,omitempty"`

	// configMapKeyRef refers to the transformation configuration residing in a ConfigMap
	ConfigMapKeyRef *corev1.ConfigMapKeySelector `json:"configMapKeyRef,omitempty" protobuf:"bytes,3,opt,name=configMapKeyRef"`
}

// Destination defines an additional endpoint to forward a transformed event payload to
type Destination struct {
	// +optional
	Transformer Transformer `json:"transformer,omitempty"`

	// url is the destination endpoint (https://hostname:port/path).
	URL string `json:"url"`

	// InsecureSkipTLSVerify skips the validity check for the server's certificate.
	// This will make your HTTPS connections insecure.
	// +optional
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify,omitempty"`

	// Sets the name of the secret that contains the headers to pass to the client
	// +optional
	Headers Headers `json:"headers,omitempty"`

	// ConfirmDelivery determines the processor return code behavior
	// If ConfirmDelivery is true, a 200 will be returned to the sender after all deliveries to flagged Destinations are successful
	// If ConfirmDelivery is false for all Destinations, a 200 will be returned to the sender once the event is confirmed as valid json
	// +optional
	ConfirmDelivery bool `json:"confirmDelivery,omitempty"`
}

// Sources of headers to append to request
type Headers struct {
	// Sets the name of the secret that contains the headers
	// +optional
	SecretRef string `json:"secretRef,omitempty"`
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
