package v1alpha1

import (
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// MeterReportSpec defines the desired state of MeterReport
type MeterReportSpec struct {
	// StartTime of the job
	StartTime metav1.Time `json:"startTime"`

	// EndTime of the jbo
	EndTime metav1.Time `json:"endTime"`

	// PrometheusService is the definition for the service labels.
	PrometheusService *ServiceReference `json:"prometheusService"`

	// MeterDefinitions includes the meter defs to be included in this job.
	MeterDefinitionLabels *metav1.LabelSelector `json:"meterDefinitionLabels"`

	// MeterDefinitions is the list of meterDefinitions included in the report
	// +optional
	MeterDefinitions []*MeterDefinition `json:"meterDefinitions,omitempty"`
}

// MeterReportStatus defines the observed state of MeterReport
type MeterReportStatus struct {
	// Conditions represent the latest available observations of an object's stateonfig
	Conditions *batchv1.JobCondition `json:"conditions,omitempty"`

	// A list of pointers to currently running jobs.
	// +optional
	AssociatedJob *JobReference `json:"jobReference,omitempty"`
}

type ServiceReference struct {
	// Namespace of the job
	// Required
	Namespace string `json:"namespace"`

	// Name of the job
	// Required
	Name string `json:"name"`

	// Port name is the name of the part to select
	// Required
	TargetPort intstr.IntOrString `json:"targetPort"`
}

// JobStatus represents the current job for the report and it's status.
type JobReference struct {

	// Namespace of the job
	// Required
	Namespace string `json:"namespace"`

	// Name of the job
	// Required
	Name string `json:"name"`

	// Represents time when the job was acknowledged by the job controller.
	// It is not guaranteed to be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty" protobuf:"bytes,2,opt,name=startTime"`

	// Represents time when the job was completed. It is not guaranteed to
	// be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty" protobuf:"bytes,3,opt,name=completionTime"`

	// The number of actively running pods.
	// +optional
	Active int32 `json:"active,omitempty" protobuf:"varint,4,opt,name=active"`

	// The number of pods which reached phase Succeeded.
	// +optional
	Succeeded int32 `json:"succeeded,omitempty" protobuf:"varint,5,opt,name=succeeded"`

	// The number of pods which reached phase Failed.
	// +optional
	Failed int32 `json:"failed,omitempty" protobuf:"varint,6,opt,name=failed"`
}

func (j *JobReference) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: j.Namespace,
		Name:      j.Name,
	}
}

func (j *JobReference) SetFromJob(job *batchv1.Job) {
	j.Name = job.Name
	j.Namespace = job.Namespace
	j.StartTime = job.Status.StartTime
	j.CompletionTime = job.Status.CompletionTime
	j.Succeeded = job.Status.Succeeded
	j.Active = job.Status.Succeeded
	j.Failed = job.Status.Failed
}

func (j *JobReference) IsSuccessful() bool {
	return j.Succeeded > 0
}

func (j *JobReference) IsFailed() bool {
	return j.Failed > 0 && j.Succeeded == 0 && !j.IsActive()
}

func (j *JobReference) IsActive() bool {
	return j.Active > 0
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
