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

package common

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// JobStatus represents the current job for the report and it's status.
// +kubebuilder:object:generate:=true
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
	Active *int32 `json:"active,omitempty" protobuf:"varint,4,opt,name=active"`

	// The number of pods which reached phase Succeeded.
	// +optional
	Succeeded *int32 `json:"succeeded,omitempty" protobuf:"varint,5,opt,name=succeeded"`

	// The number of pods which reached phase Failed.
	// +optional
	Failed *int32 `json:"failed,omitempty" protobuf:"varint,6,opt,name=failed"`

	// Specifies the number of retries before marking this job failed.
	// Defaults to 6
	// +optional
	BackoffLimit *int32 `json:"backoffLimit,omitempty" protobuf:"varint,7,opt,name=backoffLimit"`

	// JobSuccess is the boolean value set if the job succeeded
	// +optional
	JobSuccess bool `json:"jobSuccess,omitempty"`

	// JobFailed is the boolean value set if the job failed
	// +optional
	JobFailed bool `json:"jobFailed,omitempty"`
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
	j.Succeeded = nil
	j.Active = nil
	j.Failed = nil
	j.BackoffLimit = nil
	j.JobFailed = false
	j.JobSuccess = false

	for _, c := range job.Status.Conditions {
		switch c.Status {
		case corev1.ConditionTrue:
			if c.Type == batchv1.JobFailed {
				j.JobFailed = true
			}
			if c.Type == batchv1.JobComplete {
				j.JobSuccess = true
			}
		default:
			continue
		}
	}
}

func (j *JobReference) IsSuccessful() bool {
	return j.JobSuccess
}

func (j *JobReference) IsFailed() bool {
	return j.JobFailed
}

func (j *JobReference) IsDone() bool {
	return j.JobFailed || j.JobSuccess
}
