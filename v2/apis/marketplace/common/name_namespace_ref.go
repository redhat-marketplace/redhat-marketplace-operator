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
	"emperror.dev/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// JobStatus represents the current job for the report and it's status.
// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
// +kubebuilder:object:generate:=true
type NamespacedNameReference struct {

	// Namespace of the resource
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	UID types.UID `json:"uid,omitempty"`

	// Namespace of the resource
	// Required
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Namespace string `json:"namespace"`

	// Name of the resource
	// Required
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Name string `json:"name"`

	// GroupVersionKind of the resource
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	*GroupVersionKind `json:"groupVersionKind,omitempty"`
}

// +kubebuilder:object:generate:=true
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

func NewGroupVersionKind(t interface{}, scheme *runtime.Scheme) (*GroupVersionKind, error) {
	v, ok := t.(client.Object)
	if !ok {
		return &GroupVersionKind{}, errors.New("not a runtime object")
	}

	gvk, err := apiutil.GVKForObject(v, scheme)

	if !ok {
		return &GroupVersionKind{}, errors.Wrap(err, "can't get gvk")
	}

	apiVersion, kind := gvk.ToAPIVersionAndKind()

	return &GroupVersionKind{
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

func NamespacedNameFromMeta(t client.Object) *NamespacedNameReference {
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
