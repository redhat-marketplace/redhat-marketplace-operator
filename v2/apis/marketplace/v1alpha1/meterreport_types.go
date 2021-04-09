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
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// MeterReportSpec defines the desired state of MeterReport
// +k8s:openapi-gen=true
type MeterReportSpec struct {
	// StartTime of the job
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	StartTime metav1.Time `json:"startTime"`

	// EndTime of the job
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	EndTime metav1.Time `json:"endTime"`

	// PrometheusService is the definition for the service labels.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	PrometheusService *common.ServiceReference `json:"prometheusService"`

	// MeterDefinitions is the list of meterDefinitions included in the report
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	MeterDefinitions []MeterDefinition `json:"meterDefinitions,omitempty"`

	// ExtraArgs is a set of arguments to pass to the job
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="hidden"
	// +optional
	ExtraArgs []string `json:"extraJobArgs,omitempty"`
}

// MeterReportStatus defines the observed state of MeterReport
type MeterReportStatus struct {
	// Conditions represent the latest available observations of an object's stateonfig
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.x-descriptors="urn:alm:descriptor:io.kubernetes.conditions"
	Conditions status.Conditions `json:"conditions,omitempty"`

	// A list of pointers to currently running jobs.
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	AssociatedJob *common.JobReference `json:"jobReference,omitempty"`

	// MetricUploadCount is the number of metrics in the report
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	MetricUploadCount *int `json:"metricUploadCount,omitempty"`

	// UploadID is the ID associated with the upload
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	UploadID *types.UID `json:"uploadUID,omitempty"`

	// QueryErrorList shows if there were any errors from queries
	// for the report.
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	QueryErrorList []string `json:"queryErrorList,omitempty"`
}

const (
	ReportConditionTypeJobRunning      status.ConditionType   = "JobRunning"
	ReportConditionReasonJobSubmitted  status.ConditionReason = "Submitted"
	ReportConditionReasonJobNotStarted status.ConditionReason = "NotStarted"
	ReportConditionReasonJobWaiting    status.ConditionReason = "Waiting"
	ReportConditionReasonJobFinished   status.ConditionReason = "Finished"
	ReportConditionReasonJobErrored    status.ConditionReason = "Errored"
)

var (
	ReportConditionJobNotStarted = status.Condition{
		Type:    ReportConditionTypeJobRunning,
		Status:  corev1.ConditionFalse,
		Reason:  ReportConditionReasonJobNotStarted,
		Message: "Job has not been started",
	}
	ReportConditionJobSubmitted = status.Condition{
		Type:    ReportConditionTypeJobRunning,
		Status:  corev1.ConditionTrue,
		Reason:  ReportConditionReasonJobSubmitted,
		Message: "Job has been submitted",
	}
	ReportConditionJobWaiting = status.Condition{
		Type:    ReportConditionTypeJobRunning,
		Status:  corev1.ConditionFalse,
		Reason:  ReportConditionReasonJobWaiting,
		Message: "Report end time has not progressed.",
	}
	ReportConditionJobFinished = status.Condition{
		Type:    ReportConditionTypeJobRunning,
		Status:  corev1.ConditionFalse,
		Reason:  ReportConditionReasonJobFinished,
		Message: "Job has finished",
	}
	ReportConditionJobErrored = status.Condition{
		Type:    ReportConditionTypeJobRunning,
		Status:  corev1.ConditionFalse,
		Reason:  ReportConditionReasonJobErrored,
		Message: "Job has errored",
	}
)

// +kubebuilder:object:root=true

// MeterReport is the Schema for the meterreports API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=marketplaceconfigs,scope=Namespaced
// +kubebuilder:printcolumn:name="RUNNING",type=string,JSONPath=`.status.conditions[?(@.type == "JobRunning")].status`
// +kubebuilder:printcolumn:name="REASON",type=string,JSONPath=`.status.conditions[?(@.type == "JobRunning")].reason`
// +kubebuilder:printcolumn:name="METRICS",type=string,JSONPath=`.status.metricUploadCount`
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Reports"
// +kubebuilder:resource:path=meterreports,scope=Namespaced
type MeterReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MeterReportSpec   `json:"spec,omitempty"`
	Status MeterReportStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MeterReportList contains a list of MeterReport
type MeterReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MeterReport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MeterReport{}, &MeterReportList{})
}
