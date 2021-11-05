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
	"strconv"

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
	// DEPRECATED
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	PrometheusService *common.ServiceReference `json:"prometheusService,omitempty"`

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
	// +optional
	UploadStatus UploadDetailConditions `json:"uploadStatus,omitempty"`

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

	// DataServiceStatus is the status of the report stored in data service
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	DataServiceStatus *UploadDetails `json:"dataServiceStatus,omitempty"`

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

func (stat *MeterReportStatus) IsStored() bool {
	if cond := stat.Conditions.GetCondition(ReportConditionTypeStorageStatus); cond != nil {
		return cond.IsTrue()
	}
	return false
}

func (stat *MeterReportStatus) IsUploaded() bool {
	if cond := stat.Conditions.GetCondition(ReportConditionTypeUploadStatus); cond != nil {
		return cond.IsTrue()
	}
	return false
}

type UploadStatus string

const (
	UploadStatusSuccess UploadStatus = "success"
	UploadStatusFailure UploadStatus = "failure"
)

func (a *UploadStatus) UnmarshalJSON(b []byte) error {
	str, err := strconv.Unquote(string(b))

	if err != nil {
		return err
	}

	*a = UploadStatus(str)
	return nil
}

func (a UploadStatus) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(string(a))), nil
}

func (a UploadStatus) String() string {
	return string(a)
}

// UploadDetails provides details about uploads for the meterreport
type UploadDetails struct {
	// Target is the upload target
	Target string `json:"target"`
	// ID is the upload id
	ID string `json:"id,omitempty"`
	// Status is the current status
	Status UploadStatus `json:"status"`
	// Error is present if an error occurred on upload
	Error string `json:"error,omitempty"`
}

func (u UploadDetails) Success() bool {
	return u.Status == UploadStatusSuccess
}

func (u UploadDetails) Err() error {
	if u.Error == "" {
		return nil
	}
	return errors.New(u.Error)
}

type UploadDetailConditions []*UploadDetails

func (u *UploadDetailConditions) Append(conds UploadDetailConditions) {
	if u == nil {
		u = &UploadDetailConditions{}
	}

	for j := range conds {
		cond := conds[j]
		u.Set(*cond)
	}
}

func (u *UploadDetailConditions) Set(cond UploadDetails) {
	if u == nil {
		u = &UploadDetailConditions{}
	}

	for i := range *u {
		if (*u)[i].Target == cond.Target {
			(*u)[i] = &cond
			return
		}
	}

	*u = append(*u, &cond)
}

func (u UploadDetailConditions) Get(target string) *UploadDetails {
	for _, status := range u {
		if status != nil && status.Target == target {
			return status
		}
	}

	return nil
}

func (u UploadDetailConditions) OneSucessOf(targets []string) bool {
	for _, target := range targets {
		status := u.Get(target)

		if status != nil && status.Target == target {
			if status.Status == UploadStatusSuccess {
				return true
			}
		}
	}

	return false
}

func (u UploadDetailConditions) AllSuccesses() bool {
	if len(u) == 0 {
		return false
	}

	for _, status := range u {
		if status != nil && !status.Success() {
			return false
		}
	}

	return true
}

func (u UploadDetailConditions) Errors() (err error) {
	for _, status := range u {
		if status != nil && !status.Success() {
			err = errors.Append(err, errors.Errorf("target:%s error:%s", status.Target, status.Error))
			return
		}
	}

	return
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
	ReportConditionTypeJobRunning          status.ConditionType   = "JobRunning"
	ReportConditionReasonJobSubmitted      status.ConditionReason = "Submitted"
	ReportConditionReasonJobNotStarted     status.ConditionReason = "NotStarted"
	ReportConditionReasonJobWaiting        status.ConditionReason = "Waiting"
	ReportConditionReasonJobFinished       status.ConditionReason = "Finished"
	ReportConditionReasonJobErrored        status.ConditionReason = "Errored"
	ReportConditionReasonJobIsDisconnected status.ConditionReason = "Disconn"

	ReportConditionTypeStorageStatus status.ConditionType = "Stored"
	ReportConditionTypeUploadStatus  status.ConditionType = "Uploaded"

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

	ReportConditionStorageStatusFinished = status.Condition{
		Type:   ReportConditionTypeStorageStatus,
		Status: corev1.ConditionTrue,
		Reason: ReportConditionReasonUploadStatusFinished,
	}
	ReportConditionStorageStatusUnknown = status.Condition{
		Type:   ReportConditionTypeStorageStatus,
		Status: corev1.ConditionUnknown,
		Reason: ReportConditionReasonUploadStatusNotStarted,
	}
	ReportConditionStorageStatusErrored = status.Condition{
		Type:   ReportConditionTypeStorageStatus,
		Status: corev1.ConditionFalse,
		Reason: ReportConditionReasonUploadStatusErrored,
	}
	ReportConditionJobIsDisconnected = status.Condition{
		Type:    ReportConditionTypeUploadStatus,
		Status:  corev1.ConditionFalse,
		Reason:  ReportConditionReasonJobIsDisconnected,
		Message: "Report is running in a disconnected environment",
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
// +kubebuilder:printcolumn:name="STORED",type=string,JSONPath=`.status.conditions[?(@.type == "Stored")].status`
// +kubebuilder:printcolumn:name="STORED_REASON",type=string,JSONPath=`.status.conditions[?(@.type == "Stored")].reason`
// +kubebuilder:printcolumn:name="UPLOADED",type=string,JSONPath=`.status.conditions[?(@.type == "Uploaded")].status`
// +kubebuilder:printcolumn:name="UPLOADED_REASON",type=string,JSONPath=`.status.conditions[?(@.type == "Uploaded")].reason`
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
