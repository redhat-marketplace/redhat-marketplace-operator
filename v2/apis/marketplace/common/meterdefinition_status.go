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
)
