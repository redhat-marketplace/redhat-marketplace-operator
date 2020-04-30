// Copyright 2020 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"github.com/operator-framework/operator-sdk/pkg/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: Add open API validation

// MeterDefinitionSpec defines the desired metering spec
// +k8s:openapi-gen=true
type MeterDefinitionSpec struct {

	// MeterDomain defines the primary CRD domain of the meter
	MeterDomain string `json:"meterDomain"`

	// MeterVersion defines the primary CRD version of the meter
	MeterVersion string `json:"meterVersion"`

	// MeterKind defines the primary CRD kind of the meter
	MeterKind string `json:"meterKind"`

	// ServiceLabels of the meterics you want to track.
	ServiceMeterLabels []string `json:"serviceMeterLabels,omitempty"`

	// PodLabels of the prometheus metrics you want to track.
	PodMeterLabels []string `json:"podMeterLabels,omitempty"`

	// ServiceMonitors to be selected for target discovery.
	ServiceMonitorSelector *metav1.LabelSelector `json:"serviceMonitorSelector,omitempty"`

	// Namespaces to be selected for ServiceMonitor discovery. If nil, only
	// check own namespace.
	ServiceMonitorNamespaceSelector *metav1.LabelSelector `json:"serviceMonitorNamespaceSelector,omitempty"`

	// PodSelectors to select pods for metering
	PodSelector *metav1.LabelSelector `json:"podMonitorSelector,omitempty"`

	// PodNamespaceSelector to select namespaces for pods for metering
	PodNamespaceSelector *metav1.LabelSelector `json:"podMonitorNamespaceSelector,omitempty"`
}

// MeterDefinitionStatus defines the observed state of MeterDefinition
// +k8s:openapi-gen=true
type MeterDefinitionStatus struct {

	// Conditions represent the latest available observations of an object's state
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Conditions status.Conditions `json:"conditions"`

	// ServiceLabels of the meterics you want to track.
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	ServiceLabels []string `json:"serviceLabels"`

	// PodLabels of the prometheus kube-state metrics you want to track.
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	PodLabels []string `json:"podLabels"`

	// ServiceMonitors is the list of service monitors being watched for
	// this meter definition
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	ServiceMonitors []*metav1.ObjectMeta `json:"serviceMonitors"`

	// Pods is the list of current pod mointors being watched for
	// this meter definition
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Pods []*metav1.ObjectMeta `json:"pods"`
}


// MeterDefinition is internal Meter Definitions defined by Operators from Red Hat Marketplace.
// This is an internal resource not meant to be modified directly.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=meterdefinitions,scope=Namespaced
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="(Internal) Meter Definitions"
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`ServiceMonitor,v1,"redhat-marketplace-operator"`
type MeterDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MeterDefinitionSpec   `json:"spec,omitempty"`
	Status MeterDefinitionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MeterDefinitionList contains a list of MeterDefinition
type MeterDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MeterDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MeterDefinition{}, &MeterDefinitionList{})
}
