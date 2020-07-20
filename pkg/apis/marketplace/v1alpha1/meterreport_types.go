package v1alpha1

import (
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MeterReportSpec defines the desired state of MeterReport
type MeterReportSpec struct {
	// StartTime of the job
	StartTime metav1.Time `json:"startTime"`

	// EndTime of the jbo
	EndTime metav1.Time `json:"endTime"`

	// PrometheusService is the definition for the service labels.
	PrometheusService *corev1.ObjectReference `json:"prometheusService"`

	// MeterDefinitions includes the meter defs to be included in this job.
	MeterDefinitionLabels *metav1.LabelSelector `json:"meterDefinitionLabels"`

	// MeterDefinitions is the list of meterDefinitions included in the report
	MeterDefinitions []*MeterDefinition `json:"meterDefinitions"`
}

// MeterReportStatus defines the observed state of MeterReport
type MeterReportStatus struct {
	// Conditions represent the latest available observations of an object's stateonfig
	Conditions *batch.JobCondition `json:"conditions,omitempty"`

	// A list of pointers to currently running jobs.
	// +optional
	Active []corev1.ObjectReference `json:"active,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MeterReport is the Schema for the meterreports API
// +kubebuilder:subresource:status
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
