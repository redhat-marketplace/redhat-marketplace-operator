package meter_definition

import (
	"fmt"
	"reflect"
	"strings"

	"emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

var filterLogs = logger.NewLogger("filter")

type FilterRuntimeObject interface {
	Filter(interface{}) (bool, error)
}

func printFilterList(fs []FilterRuntimeObject) string {
	strs := make([]string, 0, len(fs))

	for _, f := range fs {
		strs = append(strs, printFilter(f))
	}

	return strings.Join(strs, ",")
}

func printFilter(f FilterRuntimeObject) string {
	if v, ok := f.(fmt.Stringer); ok {
		return v.String()
	}

	return fmt.Sprintf("Filter{Type:%T}", f)
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
	o, ok := obj.(runtime.Object)
	if !ok {
		return false, nil
	}

	for _, gvk := range f.gvks {
		if gvk == reflect.TypeOf(o) {
			filterLogs.Info("matching gvk",
				"matched", "true",
				"gvk", gvk,
				"obj", fmt.Sprintf("%T", obj))
			return true, nil
		}
	}
	return false, nil
}

type WorkloadFilterForOwner struct {
	workload  v1alpha1.Workload
	findOwner *rhmclient.FindOwnerHelper
}

func (f *WorkloadFilterForOwner) String() string {
	return fmt.Sprintf("WorkloadFilterForOwner{workload=%v}", f.workload)
}

func (f *WorkloadFilterForOwner) Filter(obj interface{}) (bool, error) {
	meta, ok := obj.(metav1.Object)

	if !ok {
		return false, errors.New("type was not a metav1.Object")
	}

	owner := metav1.GetControllerOf(meta)

	if owner == nil {
		return false, nil
	}

	if owner.APIVersion == f.workload.OwnerCRD.APIVersion && owner.Kind == f.workload.OwnerCRD.Kind {
		return true, nil
	}

	namespace := meta.GetNamespace()
	var err error

	for i := 0; i < 5; i++ {
		owner, err = f.findOwner.FindOwner(owner.Name, namespace, owner)

		if err != nil {
			return false, err
		}

		if owner == nil {
			return false, nil
		}

		if owner.APIVersion == f.workload.OwnerCRD.APIVersion && owner.Kind == f.workload.OwnerCRD.Kind {
			return true, nil
		}
	}

	return false, nil
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
