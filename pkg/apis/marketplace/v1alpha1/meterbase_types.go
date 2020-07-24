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
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	status "github.com/operator-framework/operator-sdk/pkg/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// StorageSpec contains configuration for pvc claims.
type StorageSpec struct {
	// Storage class for the prometheus stateful set. Default is "" i.e. default.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Class *string `json:"class,omitempty"`

	// Storage size for the prometheus deployment. Default is 40Gi.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=quantity
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Size resource.Quantity `json:"size,omitempty"`
}

// PrometheusSpec contains configuration regarding prometheus
// deployment used for metering.
type PrometheusSpec struct {
	// Resource requirements for the deployment. Default is not defined.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	corev1.ResourceRequirements `json:"resources,omitempty"`

	// Selector for the pods in the Prometheus deployment
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	NodeSelector map[string]string `json:"selector,omitempty"`

	// Storage for the deployment.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Storage StorageSpec `json:"storage"`
}

// MeterBaseSpec defines the desired state of MeterBase
// +k8s:openapi-gen=true
type MeterBaseSpec struct {
	// Enabled is the flag that controls if the controller does work. Setting
	// enabled to "true" will install metering components. False will suspend controller
	// operations for metering components.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Enabled bool `json:"enabled"`

	// Prometheus deployment configuration.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Prometheus *PrometheusSpec `json:"prometheus,omitempty"`
}

// MeterBaseStatus defines the observed state of MeterBase.
// +k8s:openapi-gen=true
type MeterBaseStatus struct {
	// MeterBaseConditions represent the latest available observations of an object's stateonfig
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Conditions *status.Conditions `json:"conditions,omitempty"`
	// PrometheusStatus is the most recent observed status of the Prometheus cluster. Read-only. Not
	// included when requesting from the apiserver, only from the Prometheus
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	PrometheusStatus *monitoringv1.PrometheusStatus `json:"prometheusStatus,omitempty"`

	// Total number of non-terminated pods targeted by this Prometheus deployment
	// (their labels match the selector).
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// Total number of non-terminated pods targeted by this Prometheus deployment
	// that have the desired version spec.
	// +optional
	UpdatedReplicas *int32 `json:"updatedReplicas,omitempty"`
	// Total number of available pods (ready for at least minReadySeconds)
	// targeted by this Prometheus deployment.
	// +optional
	AvailableReplicas *int32 `json:"availableReplicas,omitempty"`
	// Total number of unavailable pods targeted by this Prometheus deployment.
	// +optional
	UnavailableReplicas *int32 `json:"unavailableReplicas,omitempty"`
}

// MeterBase is the resource that sets up Metering for Red Hat Marketplace.
// This is an internal resource not meant to be modified directly.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=meterbases,scope=Namespaced
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="(Internal) Meter Config"
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`ServiceMonitor,v1,"redhat-marketplace-operator"`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Prometheus,v1,"redhat-marketplace-operator"`
type MeterBase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MeterBaseSpec   `json:"spec,omitempty"`
	Status MeterBaseStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MeterBaseList contains a list of MeterBase
type MeterBaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MeterBase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MeterBase{}, &MeterBaseList{})
}

const (
	// Reasons for install
	ReasonMeterBaseStartInstall             status.ConditionReason = "StartMeterBaseInstall"
	ReasonMeterBasePrometheusInstall        status.ConditionReason = "StartMeterBasePrometheusInstall"
	ReasonMeterBasePrometheusServiceInstall status.ConditionReason = "StartMeterBasePrometheusServiceInstall"
	ReasonMeterBaseFinishInstall            status.ConditionReason = "FinishedMeterBaseInstall"
)
