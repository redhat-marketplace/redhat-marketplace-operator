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
	"context"
	"fmt"
	"reflect"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MeterDefWorkload = types.NamespacedName

type MeterDefinitionLookupFilterFactory struct {
	Client    client.Client
	Log       logr.Logger
	FindOwner *rhmclient.FindOwnerHelper
	IsStarted managers.CacheIsStarted
}

type MeterDefinitionLookupFilter struct {
	MeterDefUID     string
	MeterDefName    MeterDefWorkload
	MeterDefinition *v1beta1.MeterDefinition
	ResourceVersion string

	client    client.Client
	log       logr.Logger
	findOwner *rhmclient.FindOwnerHelper

	workloads []v1beta1.ResourceFilter
	filters   []FilterRuntimeObjects
}

func (f *MeterDefinitionLookupFilterFactory) New(
	meterdef *v1beta1.MeterDefinition,
) (*MeterDefinitionLookupFilter, error) {
	s := &MeterDefinitionLookupFilter{
		MeterDefUID:     string(meterdef.UID),
		MeterDefinition: meterdef,
		MeterDefName:    types.NamespacedName{Name: meterdef.Name, Namespace: meterdef.Namespace},
		ResourceVersion: meterdef.ResourceVersion,
		client:          f.Client,
		findOwner:       f.FindOwner,
		log:             f.Log.WithName("meterDefLookupFilter").WithValues("meterdefName", meterdef.Name, "meterdefNamespace", meterdef.Namespace).V(4),
	}

	if len(meterdef.Spec.ResourceFilters) == 0 {
		return nil, errors.New("no resource filters provided")
	}

	filters, err := s.createFilters(meterdef)
	if err != nil {
		s.log.Error(err, "error creating filters")
		return nil, err
	}

	s.filters = filters
	s.workloads = meterdef.Spec.ResourceFilters

	return s, nil
}

func (s *MeterDefinitionLookupFilter) GetNamespaces() map[string][]reflect.Type {
	namespaces := map[string][]reflect.Type{}

	for _, f := range s.filters {
		types := f.Types()

		for _, ns := range f.Namespaces() {
			namespaces[ns] = append(namespaces[ns], types...)
		}
	}

	return namespaces
}

func (s *MeterDefinitionLookupFilter) String() string {
	return fmt.Sprintf("MeterDef{workloads=%v, filters=%v}", len(s.workloads), len(s.filters))
}

func (s *MeterDefinitionLookupFilter) Matches(obj interface{}) (bool, error) {
	o, ok := obj.(metav1.Object)

	if !ok {
		err := errors.New("type is not a metav1 Object")
		s.log.Error(err, "failed to find workload due to error")
		return false, err
	}

	filterLogger := s.log.WithValues("obj", o.GetName()+"/"+o.GetNamespace(), "type", fmt.Sprintf("%T", obj), "filterLen", len(s.filters))

	for key, workloadFilters := range s.filters {
		ans, i, err := workloadFilters.Test(obj)
		// Don't produce an error if the object gets deleted while testing
		if err != nil && k8serrors.IsNotFound(err) {
			return false, nil
		}
		if err != nil && !errors.Is(err, rhmclient.AccessDeniedErr) {
			filterLogger.Error(err, "filter failed", "key", key, "filters", workloadFilters, "i", i)
			return false, err
		}

		if errors.Is(err, rhmclient.AccessDeniedErr) {
			return false, err
		}

		if ans {
			return ans, nil
		}
	}

	return false, nil
}

func (s *MeterDefinitionLookupFilter) installedByNamespace(
	instance *v1beta1.MeterDefinition,
	csv client.Object,
) ([]string, error) {
	reqLogger := s.log.WithValues("func", "installedByNamespace", "meterdef", instance.Name+"/"+instance.Namespace).V(4)

	olmGroup, ok := csv.GetAnnotations()["olm.operatorGroup"]
	if !ok {
		return []string{instance.GetNamespace()}, nil
	}

	olmNamespace, ok := csv.GetAnnotations()["olm.operatorNamespace"]
	if !ok {
		return []string{instance.GetNamespace()}, nil
	}

	if ok && olmGroup == "global-operators" && olmNamespace == "openshift-operators" {
		if olmNamespace == csv.GetNamespace() {
			reqLogger.Info("operatorGroup is for all namespaces")
			return []string{corev1.NamespaceAll}, nil
		} else {
			reqLogger.Info("operatorGroup is for all namespaces, but csv is a copy")
			return []string{}, nil
		}
	}

	reqLogger.Info("installedBy not found, falling back to namespace")
	return []string{instance.GetNamespace()}, nil
}

func (s *MeterDefinitionLookupFilter) findNamespacesForResource(
	instance *v1beta1.MeterDefinition,
	resourceFilter v1beta1.ResourceFilter,
) ([]string, error) {
	functionError := errors.NewWithDetails("error with findNamespaces", "meterdef", instance.Name+"/"+instance.Namespace)
	reqLogger := s.log.WithValues("func", "findNamespaces", "meterdef", instance.Name+"/"+instance.Namespace)

	reqLogger.Info("getting namespaces for resource")

	namespaces := []string{instance.GetNamespace()}

	if instance.GetNamespace() == "openshift-operators" {
		namespaces = []string{corev1.NamespaceAll}
	}

	if resourceFilter.Namespace == nil {
		return namespaces, nil
	}

	if resourceFilter.Namespace.UseOperatorGroup {

		// Use PartialObjectMetadata such that we are only using metadata client
		// Do not ListWatch full CSV in all namespaces, potentially memory intensive
		csvMeta := &metav1.PartialObjectMetadata{}

		kinds, _, err := s.client.Scheme().ObjectKinds(&olmv1alpha1.ClusterServiceVersion{})
		if err != nil {
			return namespaces, err
		}

		csvMeta.SetGroupVersionKind(kinds[0])

		if instance.Spec.InstalledBy != nil {
			reqLogger.Info("using installedBy")
			err := s.client.Get(context.TODO(), instance.Spec.InstalledBy.ToTypes(), csvMeta)

			if err != nil && k8serrors.IsNotFound(err) {
				reqLogger.Info("installedBy not found, falling back to namespace")
				return namespaces, nil
			}

			if err != nil {
				err = errors.Wrap(functionError, "csv not found due to error")
				reqLogger.Error(err, "installed by is not found")
				return namespaces, err
			}

			return s.installedByNamespace(instance, csvMeta)
		}

		reqLogger.Info("looking for operator group")
		operatorGroups := &olmv1.OperatorGroupList{}
		if err := s.client.List(context.TODO(), operatorGroups, client.InNamespace(instance.Namespace)); err != nil && k8serrors.IsNotFound(err) {
			reqLogger.Info("installedBy not found, falling back to namespace")
			return namespaces, nil
		}

		if len(operatorGroups.Items) == 0 || len(operatorGroups.Items) > 1 {
			return namespaces, nil
		}

		og := operatorGroups.Items[0]
		reqLogger.Info("found operator group", "name", og.Name, "namespaces", fmt.Sprintf("%+v", og.Status.Namespaces))
		return og.Status.Namespaces, nil
	}

	if resourceFilter.Namespace.LabelSelector != nil {
		reqLogger.Info("namespace vertex with label filter")
		namespaceList := &corev1.NamespaceList{}

		selector, err := metav1.LabelSelectorAsSelector(resourceFilter.Namespace.LabelSelector)

		if err != nil {
			return namespaces, err
		}

		err = s.client.List(context.TODO(), namespaceList, client.MatchingLabelsSelector{Selector: selector})

		if err != nil {
			err = errors.Wrap(functionError, "namespace list error")
			return namespaces, err
		}

		for i := range namespaceList.Items {
			localNs := namespaceList.Items[i].GetName()
			namespaces = append(namespaces, localNs)
		}

		return namespaces, nil
	}

	return namespaces, nil
}

func (s *MeterDefinitionLookupFilter) createFilters(
	instance *v1beta1.MeterDefinition,
) ([]FilterRuntimeObjects, error) {
	// Bottom Up
	// Start with pods, filter, go to owner. If owner not provided, stop.
	filters := []FilterRuntimeObjects{}

	for _, filter := range instance.Spec.ResourceFilters {
		namespaces, err := s.findNamespacesForResource(instance, filter)
		if err != nil {
			s.log.Error(err, "namespaces err")
			return nil, err
		}

		s.log.Info("filter info", "namespaces", fmt.Sprintf("%+v", namespaces))

		runtimeFilters := []FilterRuntimeObject{&WorkloadNamespaceFilter{namespaces: namespaces}}

		typeFilter := &WorkloadTypeFilter{}
		switch filter.WorkloadType {
		case common.WorkloadTypePod:
			gvk := reflect.TypeOf(&corev1.Pod{})
			typeFilter.gvks = []reflect.Type{gvk}
		case common.WorkloadTypePVC:
			gvk := reflect.TypeOf(&corev1.PersistentVolumeClaim{})
			typeFilter.gvks = []reflect.Type{gvk}
		case common.WorkloadTypeService:
			gvk1 := reflect.TypeOf(&corev1.Service{})
			typeFilter.gvks = []reflect.Type{gvk1}
		default:
			s.log.Error(err, "unknown type filter", "type", filter.WorkloadType)
			err = errors.NewWithDetails("unknown type filter", "type", filter.WorkloadType)
			return nil, err
		}

		runtimeFilters = append(runtimeFilters, typeFilter)

		if filter.Label != nil && filter.Label.LabelSelector != nil {
			selector, err := metav1.LabelSelectorAsSelector(filter.Label.LabelSelector)

			if err != nil {
				return nil, err
			}

			runtimeFilters = append(runtimeFilters, &WorkloadLabelFilter{
				labelSelector: selector,
			})
		}

		if filter.Annotation != nil && filter.Annotation.AnnotationSelector != nil {
			selector, err := metav1.LabelSelectorAsSelector(filter.Annotation.AnnotationSelector)

			if err != nil {
				return nil, err
			}

			runtimeFilters = append(runtimeFilters, &WorkloadAnnotationFilter{
				annotationSelector: selector,
			})
		}

		if filter.OwnerCRD != nil {
			runtimeFilters = append(runtimeFilters, NewWorkloadFilterForOwner(*filter.OwnerCRD, s.findOwner))
		}

		filters = append(filters, runtimeFilters)
	}
	return filters, nil
}

type Result struct {
	Ok     bool
	Lookup *MeterDefinitionLookupFilter
}
