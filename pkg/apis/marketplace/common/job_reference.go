package common

import (
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

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

	// Specifies the number of retries before marking this job failed.
	// Defaults to 6
	// +optional
	BackoffLimit int32 `json:"backoffLimit,omitempty" protobuf:"varint,7,opt,name=backoffLimit"`
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
	j.Active = job.Status.Active
	j.Failed = job.Status.Failed
	j.BackoffLimit = 6
	if job.Spec.BackoffLimit != nil {
		j.BackoffLimit = *job.Spec.BackoffLimit
	}

}

func (j *JobReference) IsSuccessful() bool {
	return j.Succeeded >= 1
}

func (j *JobReference) IsFailed() bool {
	return j.Failed == j.BackoffLimit+1 && !j.IsSuccessful()
}

func (j *JobReference) IsActive() bool {
	return j.Active > 0
}

func (j *JobReference) IsDone() bool {
	return !j.IsActive() && (j.IsSuccessful() || j.IsFailed())
}
