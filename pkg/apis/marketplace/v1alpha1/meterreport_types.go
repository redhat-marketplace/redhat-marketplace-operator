package v1alpha1

import (
	"github.com/operator-framework/operator-sdk/pkg/status"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// MeterReportSpec defines the desired state of MeterReport
// +k8s:openapi-gen=true
type MeterReportSpec struct {
	// StartTime of the job
	StartTime metav1.Time `json:"startTime"`

	// EndTime of the job
	EndTime metav1.Time `json:"endTime"`

	// PrometheusService is the definition for the service labels.
	PrometheusService *common.ServiceReference `json:"prometheusService"`

	// MeterDefinitions is the list of meterDefinitions included in the report
	// +optional
	MeterDefinitions []MeterDefinition `json:"meterDefinitions,omitempty"`
}

// MeterReportStatus defines the observed state of MeterReport
type MeterReportStatus struct {
	// Conditions represent the latest available observations of an object's stateonfig
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.x-descriptors="urn:alm:descriptor:io.kubernetes.conditions"
	Conditions *status.Conditions `json:"conditions,omitempty"`

	// A list of pointers to currently running jobs.
	// +optional
	AssociatedJob *common.JobReference `json:"jobReference,omitempty"`

	// MetricUploadCount is the number of metrics in the report
	// +optional
	MetricUploadCount *int `json:"metricUploadCount,omitempty"`

	// UploadID is the ID associated with the upload
	// +optional
	UploadID *types.UID `json:"uploadUID,omitempty"`

	// QueryErrorList shows if there were any errors from queries
	// for the report.
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MeterReport is the Schema for the meterreports API
// +kubebuilder:subresource:status
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Reports"
// +kubebuilder:resource:path=meterreports,scope=Namespaced
type MeterReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MeterReportSpec   `json:"spec,omitempty"`
	Status MeterReportStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MeterReportList contains a list of MeterReport
type MeterReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MeterReport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MeterReport{}, &MeterReportList{})
}
