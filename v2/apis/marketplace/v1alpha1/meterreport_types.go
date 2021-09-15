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
	"fmt"

	"emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// MeterReportSpec defines the desired state of MeterReport
// +k8s:openapi-gen=true
type MeterReportSpec struct {
	// ReportUUID is the generated ID for the report.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	ReportUUID string `json:"reportUUID,omitempty"`

	// StartTime of the job
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	StartTime metav1.Time `json:"startTime"`

	// EndTime of the job
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	EndTime metav1.Time `json:"endTime"`

	// LabelSelectors are used to filter to the correct workload.
	// DEPRECATED
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	LabelSelector metav1.LabelSelector `json:"labelSelector,omitempty"`

	// PrometheusService is the definition for the service labels.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	PrometheusService *common.ServiceReference `json:"prometheusService"`

	// MeterDefinitions is the list of meterDefinitions included in the report
	// DEPRECATED
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	MeterDefinitions []MeterDefinition `json:"meterDefinitions,omitempty"`

	// MeterDefinitionReferences are used as the first meter definition source. Prometheus data is used to supplement.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +listType:=map
	// +listMapKey:=name
	// +listMapKey:=namespace
	// +optional
	MeterDefinitionReferences []v1beta1.MeterDefinitionReference `json:"meterDefinitionReferences,omitempty"`

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
	// DEPRECATED
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	AssociatedJob *common.JobReference `json:"jobReference,omitempty"`

	// UploadStatus displays the last status for upload targets.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +listType:=map
	// +listMapKey:=target
	// +optional
	UploadStatus []*UploadDetails `json:"uploadStatus,omitempty"`

	// WorkloadCount is the number of workloads reported on
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	WorkloadCount *int `json:"workloadCount,omitempty"`

	// MetricUploadCount is the number of metrics in the report
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	MetricUploadCount *int `json:"metricUploadCount,omitempty"`

	// UploadID is the ID associated with the upload
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	UploadID *types.UID `json:"uploadUID,omitempty"`

	// Errors shows if there were any errors from queries
	// for the report.
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Errors []ErrorDetails `json:"errors,omitempty"`

	// Warnings from the job
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Warnings []ErrorDetails `json:"warnings,omitempty"`
}

// UploadDetails provides details about uploads for the meterreport
type UploadDetails struct {
	// Target is the upload target
	Target string `json:"target"`
	// Status is the current status
	Status string `json:"status"`
	// Error is present if an error occured on upload
	Error string `json:"error,omitempty"`
}

// ErrorDetails provides details about errors that happen in the job
type ErrorDetails struct {
	// Reason the error occurred
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Reason string `json:"reason"`
	// Details of the error
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Details map[string]string `json:"details,omitempty"`
}

func (e ErrorDetails) FromError(err error) ErrorDetails {
	detailsMap := map[string]string{}
	details := errors.GetDetails(err)

	for i := 0; i < len(details); i = i + 2 {
		key := fmt.Sprintf("%v", details[i])
		value := fmt.Sprintf("%+v", details[i+1])
		detailsMap[key] = value
	}

	e.Details = detailsMap
	e.Reason = errors.Cause(err).Error()

	return e
}

const (
	ReportConditionTypeJobRunning      status.ConditionType   = "JobRunning"
	ReportConditionReasonJobSubmitted  status.ConditionReason = "Submitted"
	ReportConditionReasonJobNotStarted status.ConditionReason = "NotStarted"
	ReportConditionReasonJobWaiting    status.ConditionReason = "Waiting"
	ReportConditionReasonJobFinished   status.ConditionReason = "Finished"
	ReportConditionReasonJobErrored    status.ConditionReason = "Errored"

	ReportConditionTypeUploadStatus             status.ConditionType   = "Uploaded"
	ReportConditionReasonUploadStatusFinished   status.ConditionReason = "Finished"
	ReportConditionReasonUploadStatusNotStarted status.ConditionReason = "NotStarted"
	ReportConditionReasonUploadStatusErrored    status.ConditionReason = "Errored"
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

	ReportConditionUploadStatusFinished = status.Condition{
		Type:   ReportConditionTypeUploadStatus,
		Status: corev1.ConditionTrue,
		Reason: ReportConditionReasonUploadStatusFinished,
	}
	ReportConditionUploadStatusUnknown = status.Condition{
		Type:   ReportConditionTypeUploadStatus,
		Status: corev1.ConditionUnknown,
		Reason: ReportConditionReasonUploadStatusNotStarted,
	}
	ReportConditionUploadStatusErrored = status.Condition{
		Type:   ReportConditionTypeUploadStatus,
		Status: corev1.ConditionFalse,
		Reason: ReportConditionReasonUploadStatusErrored,
	}
)

// +kubebuilder:object:root=true

// MeterReport is the Schema for the meterreports API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=marketplaceconfigs,scope=Namespaced
// +kubebuilder:printcolumn:name="METRICS",type=string,JSONPath=`.status.metricUploadCount`
// +kubebuilder:printcolumn:name="UPLOADED",type=string,JSONPath=`.status.conditions[?(@.type == "Uploaded")].status`
// +kubebuilder:printcolumn:name="REASON",type=string,JSONPath=`.status.conditions[?(@.type == "Uploaded")].reason`
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
