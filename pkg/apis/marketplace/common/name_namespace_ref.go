package common

import (
	"errors"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// JobStatus represents the current job for the report and it's status.
type NamespacedNameReference struct {

	// Namespace of the resource
	// Required
	UID types.UID `json:"uid"`

	// Namespace of the resource
	// Required
	Namespace string `json:"namespace"`

	// Name of the resource
	// Required
	Name string `json:"name"`

	// GroupVersionKind of the resource
	// Required
	GroupVersionKind `json:"groupVersionKind"`
}

type GroupVersionKind struct {
	// APIVersion of the CRD
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="text"
	APIVersion string `json:"apiVersion"`
	// Kind of the CRD
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="text"
	Kind string `json:"kind"`
}

func (g GroupVersionKind) String() string {
	return g.APIVersion + "/" + g.Kind
}

func NewGroupVersionKind(t interface{}) (GroupVersionKind, error) {
	v, ok := t.(runtime.Object)
	if !ok {
		return GroupVersionKind{}, errors.New("not a runtime object")
	}

	apiVersion, kind := v.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()

	return GroupVersionKind{
		APIVersion: apiVersion,
		Kind:       kind,
	}, nil
}

func (n *NamespacedNameReference) ToTypes() types.NamespacedName {
	return types.NamespacedName{
		Namespace: n.Namespace,
		Name:      n.Name,
	}
}

func NamespacedNameFromMeta(t runtime.Object) *NamespacedNameReference {
	o, ok := t.(v1.Object)
	if !ok {
		return nil
	}

	return &NamespacedNameReference{
		UID:       o.GetUID(),
		Name:      o.GetName(),
		Namespace: o.GetNamespace(),
	}
}
