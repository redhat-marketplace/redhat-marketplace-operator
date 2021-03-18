// Copyright 2021 IBM Corp.
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
	"strings"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

// WorkloadResource represents the resources associated to a workload
// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
// +kubebuilder:object:generate:=true
type WorkloadResource struct {
	// ReferencedWorkloadName is the name of the workload
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	ReferencedWorkloadName string `json:"referencedWorkloadName,omitempty"`

	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	NamespacedNameReference `json:",inline"`
}

type ByAlphabetical []WorkloadResource

func (a ByAlphabetical) Len() int      { return len(a) }
func (a ByAlphabetical) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByAlphabetical) Less(i, j int) bool {
	return strings.Compare(a[i].ReferencedWorkloadName, a[j].ReferencedWorkloadName) > 0 &&
		strings.Compare(a[i].NamespacedNameReference.Namespace, a[j].NamespacedNameReference.Namespace) > 0 &&
		strings.Compare(a[i].NamespacedNameReference.Name, a[j].NamespacedNameReference.Name) > 0
}

func NewWorkloadResource(obj interface{}, scheme *runtime.Scheme) (*WorkloadResource, error) {
	accessor, err := meta.Accessor(obj)

	if err != nil {
		return nil, err
	}
	gvk, err := NewGroupVersionKind(obj, scheme)
	if err != nil {
		return nil, err
	}

	return &WorkloadResource{
		NamespacedNameReference: NamespacedNameReference{
			Name:             accessor.GetName(),
			Namespace:        accessor.GetNamespace(),
			UID:              accessor.GetUID(),
			GroupVersionKind: &gvk,
		},
	}, nil
}

const (
	MeterDefConditionTypeHasResult           status.ConditionType   = "FoundMatches"
	MeterDefConditionReasonNoResultsInStatus status.ConditionReason = "No results in status"
	MeterDefConditionReasonResultsInStatus   status.ConditionReason = "Results in status"

	MeterDefConditionTypeSignatureVerified             status.ConditionType   = "SignatureVerified"
	MeterDefConditionReasonSignatureUnverified         status.ConditionReason = "Signature unverified"
	MeterDefConditionReasonSignatureVerified           status.ConditionReason = "Signature verified"
	MeterDefConditionReasonSignatureVerificationFailed status.ConditionReason = "Signature verification failed"
)

var (
	MeterDefConditionNoResults = status.Condition{
		Type:    MeterDefConditionTypeHasResult,
		Status:  corev1.ConditionFalse,
		Reason:  MeterDefConditionReasonNoResultsInStatus,
		Message: "Meter definition has no results yet.",
	}
	MeterDefConditionHasResults = status.Condition{
		Type:    MeterDefConditionTypeHasResult,
		Status:  corev1.ConditionTrue,
		Reason:  MeterDefConditionReasonResultsInStatus,
		Message: "Meter definition has results.",
	}

	// MeterDefinition was not signed. No signing annotations
	MeterDefConditionSignatureUnverified = status.Condition{
		Type:    MeterDefConditionTypeSignatureVerified,
		Status:  corev1.ConditionFalse,
		Reason:  MeterDefConditionReasonSignatureUnverified,
		Message: "Meter definition unsigned and unverified",
	}
	// MeterDefinition was signed and signature verified
	MeterDefConditionSignatureVerified = status.Condition{
		Type:    MeterDefConditionTypeSignatureVerified,
		Status:  corev1.ConditionTrue,
		Reason:  MeterDefConditionReasonSignatureVerified,
		Message: "Meter definition signature verified.",
	}
	// MeterDefinition was signed and signature verification failed
	MeterDefConditionSignatureVerificationFailed = status.Condition{
		Type:    MeterDefConditionTypeSignatureVerified,
		Status:  corev1.ConditionFalse,
		Reason:  MeterDefConditionReasonSignatureVerificationFailed,
		Message: "Meter definition signature verification failed.",
	}
)
