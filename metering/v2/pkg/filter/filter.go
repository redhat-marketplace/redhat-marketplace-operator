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

package filter

import (
	"fmt"
	"reflect"
	"strings"

	"emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var filterLogs = logf.Log.WithName("filter")

type FilterRuntimeObject interface {
	Filter(interface{}) (bool, error)
}

type FilterRuntimeObjects []FilterRuntimeObject

func (s FilterRuntimeObjects) Test(obj interface{}) (pass bool, filterIndex int, err error) {
	if len(s) == 0 {
		return
	}

	for i, filter := range s {
		pass, err = filter.Filter(obj)

		if err != nil {
			filterIndex = i
			return
		}
		if !pass {
			filterIndex = i
			return
		}
	}

	return
}

func (s FilterRuntimeObjects) String() string {
	strs := make([]string, 0, len(s))
	printFilter := func(f FilterRuntimeObject) string {
		if v, ok := f.(fmt.Stringer); ok {
			return v.String()
		}

		return fmt.Sprintf("Filter{Type:%T}", f)
	}

	for _, f := range s {
		strs = append(strs, printFilter(f))
	}

	return strings.Join(strs, ",")
}

type WorkloadNamespaceFilter struct {
	namespaces []string
}

func (f *WorkloadNamespaceFilter) Filter(obj interface{}) (bool, error) {
	meta, ok := obj.(metav1.Object)

	if !ok {
		return false, errors.New("type was not a metav1.Object")
	}

	for _, ns := range f.namespaces {
		if ns == "" {
			return true, nil
		}

		if ns == meta.GetNamespace() {
			return true, nil
		}
	}

	return false, nil
}

func (f *WorkloadNamespaceFilter) String() string {
	return fmt.Sprintf("WorkloadNamespaceFilter{namespaces: %s}", strings.Join(f.namespaces, ","))
}

type WorkloadTypeFilter struct {
	gvks []reflect.Type
}

func (f *WorkloadTypeFilter) String() string {
	return fmt.Sprintf("WorkloadTypeFilter{gvk: %v}", f.gvks)
}

func (f *WorkloadTypeFilter) Filter(obj interface{}) (bool, error) {
	objType := reflect.TypeOf(obj)

	for _, gvk := range f.gvks {
		if gvk == objType {
			filterLogs.V(4).Info("matching gvk",
				"matched", "true",
				"gvk", gvk,
				"obj", fmt.Sprintf("%T", obj))
			return true, nil
		}

		filterLogs.V(4).Info("not matching",
			"matched", "false",
			"gvk", gvk,
			"obj", fmt.Sprintf("%T", obj))
	}

	return false, nil
}

type WorkloadFilterForOwner struct {
	ownerFilter v1beta1.OwnerCRDFilter
	findOwner   *rhmclient.FindOwnerHelper
	maxDepth    int
}

func NewWorkloadFilterForOwner(ownerFilter v1beta1.OwnerCRDFilter, findOwner *rhmclient.FindOwnerHelper) *WorkloadFilterForOwner {
	return &WorkloadFilterForOwner{
		ownerFilter: ownerFilter,
		findOwner:   findOwner,
		maxDepth:    5,
	}
}

func (f *WorkloadFilterForOwner) getOwners(
	name, namespace string,
	owner *metav1.OwnerReference,
	inRefs []metav1.OwnerReference,
	i, maxDepth int,
) (bool, error) {
	var refs []metav1.OwnerReference
	var err error

	if owner != nil {
		refs, err = f.findOwner.FindOwner(name, namespace, owner)
		if err != nil {
			return false, err
		}
	} else {
		refs = inRefs
	}

	for _, ref := range refs {
		if ref.APIVersion == f.ownerFilter.APIVersion && ref.Kind == f.ownerFilter.Kind {
			return true, nil
		}
	}

	if !(i < maxDepth) {
		return false, nil
	}

	for _, ref := range refs {
		found, err := f.getOwners(ref.Name, namespace, &ref, []metav1.OwnerReference{}, i+1, maxDepth)

		if err != nil {
			return false, err
		}

		if found {
			return true, err
		}
	}

	return false, nil
}

func (f *WorkloadFilterForOwner) String() string {
	return fmt.Sprintf("WorkloadFilterForOwner{workload=%v}", f.ownerFilter)
}

func (f *WorkloadFilterForOwner) Filter(obj interface{}) (bool, error) {
	meta, ok := obj.(metav1.Object)

	if !ok {
		return false, errors.New("type was not a metav1.Object")
	}

	return f.getOwners(
		meta.GetName(),
		meta.GetNamespace(), nil,
		meta.GetOwnerReferences(),
		0,
		f.maxDepth,
	)
}

type WorkloadLabelFilter struct {
	labelSelector labels.Selector
}

func (f *WorkloadLabelFilter) Filter(obj interface{}) (bool, error) {
	meta, ok := obj.(metav1.Object)

	if !ok {
		return false, errors.New("type was not a metav1.Object")
	}

	return f.labelSelector.Matches(labels.Set(meta.GetLabels())), nil
}

type WorkloadAnnotationFilter struct {
	annotationSelector labels.Selector
}

func (f *WorkloadAnnotationFilter) Filter(obj interface{}) (bool, error) {
	meta, ok := obj.(metav1.Object)

	if !ok {
		return false, errors.New("type was not a metav1.Object")
	}

	return f.annotationSelector.Matches(labels.Set(meta.GetAnnotations())), nil
}

type FalseFilter struct{}

func (f *FalseFilter) Filter(obj interface{}) (bool, error) {
	return false, nil
}
