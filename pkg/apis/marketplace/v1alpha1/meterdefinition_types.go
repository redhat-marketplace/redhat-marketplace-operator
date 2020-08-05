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
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/common"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: Add open API validation

// MeterDefinitionSpec defines the desired metering spec
// +k8s:openapi-gen=true
type MeterDefinitionSpec struct {
	// MeterDomain defines the primary CRD domain of the meter
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Group string `json:"group"`

	// MeterVersion defines the primary CRD version of the meter, ths
	// is optional.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// TODO: delete this
	// +optional
	Version string `json:"version,omitempty"`

	// MeterKind defines the primary CRD kind of the meter
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Kind string `json:"kind"`

	// InstalledBy is a reference to the CSV that install the meter
	// definition. This is used to determine an operator group.
	// +optional
	InstalledBy *common.NamespacedNameReference `json:"installedBy,omitempty"`

	// WorkloadVertex is the top most object of a workload. It allows
	// you to identify the upper bounds of your workloads. If you select
	// OperatorGroup it will use the OperatorGroup associated with the
	// Operator to select the namespaces. If you select Namespace, the
	// workloads will be filtered by labels or annotations.
	// +kubebuilder:validation:Enum=Namespace;OperatorGroup
	WorkloadVertex `json:"workloadVertexType,omitempty"`

	// VertexFilters are used when Namespace is selected. Can be omitted
	// if you select OperatorGroup
	VertexLabelSelector *metav1.LabelSelector `json:"workloadVertexLabelSelectors,omitempty"`

	// Workloads identify the workloads to meter.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +kubebuilder:validation:MinItems=1
	Workloads []Workload `json:"workloads,omitempty"`
}

const (
	WorkloadVertexOperatorGroup WorkloadVertex = "OperatorGroup"
	WorkloadVertexNamespace                    = "Namespace"
)
const (
	WorkloadTypePod            WorkloadType = "Pod"
	WorkloadTypeServiceMonitor              = "ServiceMonitor"
	WorkloadTypePVC                         = "PVC"
)

type WorkloadVertex string
type WorkloadType string
type CSVNamespacedName common.NamespacedNameReference

// Workload helps identify what to target for metering.
type Workload struct {
	// Name of the workload, must be unique in a meter definition.
	Name string `json:"name"`

	// WorkloadType identifies the type of workload to look for. This can be
	// pod or service right now.
	// +kubebuilder:validation:Enum=Pod;ServiceMonitor;PVC
	WorkloadType WorkloadType `json:"type"`

	// OwningGVK is the name of the GVK to look for as the owner of all the
	// meterable assets. If omitted, the labels and annotations are used instead.
	// +optional
	Owner *common.GroupVersionKind `json:"ownerCRD,omitempty"`

	// Labels are used to filter to the correct workload.
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// Annotations are used to filter to the correct workload.
	AnnotationSelector *metav1.LabelSelector `json:"annotationSelector,omitempty"`

	// MetricLabels are the labels to collect
	MetricLabels []MeterLabelQuery `json:"metricLabels,omitempty"`
}

type WorkloadResource struct {
	ReferencedWorkloadName string `json:"referencedWorkloadName"`

	common.NamespacedNameReference `json:",inline"`
}

func NewWorkloadResource(workload Workload, obj interface{}) (*WorkloadResource, error) {
	accessor, err := meta.Accessor(obj)

	if err != nil {
		return nil, err
	}
	gvk, err := common.NewGroupVersionKind(obj)
	if err != nil {
		return nil, err
	}

	return &WorkloadResource{
		ReferencedWorkloadName: workload.Name,
		NamespacedNameReference: common.NamespacedNameReference{
			Name: accessor.GetName(),
			Namespace: accessor.GetNamespace(),
			UID: accessor.GetUID(),
			GroupVersionKind: gvk,
		},
	}, nil
}

// WorkloadStatus provides quick status to check if
// workloads are working correctly
type WorkloadStatus struct {
	// Name of the workload, must be unique in a meter definition.
	Name string `json:"name"`

	CurrentMetricValue string `json:"currentValue"`

	LastReadTime metav1.Time `json:"startTime"`
}

// MeterLabelQuery helps define a meter label to build and search for
type MeterLabelQuery struct {
	// Label is the name of the meter
	Label string `json:"label"`

	// Query to use for the label
	Query map[string]string `json:"query,omitempty"`

	// Aggregation to use with the query
	// +kubebuilder:validation:Enum:=sum;min;max;avg;count
	Aggregation string `json:"aggregation,omitempty"`
}

// MeterDefinitionStatus defines the observed state of MeterDefinition
// +k8s:openapi-gen=true
// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
type MeterDefinitionStatus struct {

	// Conditions represent the latest available observations of an object's state
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Conditions status.Conditions `json:"conditions,omitempty"`

	// WorkloadResources is the list of resoruces discovered by
	// this meter definition
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	WorkloadResources []WorkloadResource `json:"workloadResource,omitempty"`
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
// +genclient
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
