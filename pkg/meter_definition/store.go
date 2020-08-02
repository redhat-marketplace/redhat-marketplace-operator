package meter_definition

import (
	"context"
	"strings"
	"sync"

	"emperror.dev/errors"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1client "github.com/coreos/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	marketplacev1alpha1client "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/generated/clientset/versioned/typed/marketplace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubectl/pkg/scheme"
)

type MeterDefUID types.UID
type ObjectUID types.UID
type ResourceSet map[MeterDefUID]*v1alpha1.WorkloadResource

// MeterDefinitionStore keeps the MeterDefinitions in place
// and tracks the dependents using the rules based on the
// rules. MeterDefinition controller uses this to effectively
// find the child assets of a meter definition rules.
type MeterDefinitionStore struct {
	meterDefinitionFilters map[MeterDefUID]*MeterDefinitionLookupFilter

	objectResourceSet map[ObjectUID]ResourceSet

	mutex sync.RWMutex

	log logr.Logger

	kubeClient       clientset.Interface
	monitoringClient *monitoringv1client.MonitoringV1Client
	marketplaceClient *marketplacev1alpha1client.MarketplaceV1alpha1Client
}

// func (c *MeterDefinitionStore) Register(ctrl controller.Controller, types []runtime.Object) error {
// 	for _, rType := range types {
// 		err := ctrl.Watch(&source.Kind{Type: rType}, &handler.EnqueueRequestsFromMapFunc{
// 			ToRequests: c.Handler(),
// 		})

// 		if err != nil {
// 			return err
// 		}
// 	}

// 	return nil
// }

// func (c *MeterDefinitionStore) Handler() handler.Mapper {
// 	return handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
// 		key := a.Meta.GetUID()
// 		request, ok := c.Get(key)

// 		if ok {
// 			return []reconcile.Request{request}
// 		}

// 		return []reconcile.Request{}
// 	})
// }

func (s *MeterDefinitionStore) AddMeterDefinition(meterdef v1alpha1.MeterDefinition, lookup *MeterDefinitionLookupFilter) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.meterDefinitionFilters[MeterDefUID(meterdef.UID)] = lookup
}

func (s *MeterDefinitionStore) RemoveMeterDefinition(meterdef v1alpha1.MeterDefinition) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.meterDefinitionFilters, MeterDefUID(meterdef.UID))
	for uid := range s.objectResourceSet {
		if _, ok := s.objectResourceSet[uid][MeterDefUID(meterdef.UID)]; ok {
			delete(s.objectResourceSet[uid], MeterDefUID(meterdef.UID))
		}
	}
}

// Implementing k8s.io/client-go/tools/cache.Store interface

// Add inserts adds to the OwnerCache by calling the metrics generator functions and
// adding the generated metrics to the metrics map that underlies the MetricStore.
func (s *MeterDefinitionStore) Add(obj interface{}) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	o, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	if _, ok := s.objectResourceSet[ObjectUID(o.GetUID())]; ok {
		delete(s.objectResourceSet, ObjectUID(o.GetUID()))
	}

	for meterDefUID, lookup := range s.meterDefinitionFilters {
		if lookup != nil {
			workload, err := lookup.FindMatchingWorkloads(obj)

			if err != nil {
				return err
			}

			if workload != nil {
				resource, err := v1alpha1.NewWorkloadResource(*workload, obj)
				if err != nil {
					return err
				}

				s.objectResourceSet[ObjectUID(o.GetUID())][meterDefUID] = resource
			}
		}
	}

	return nil
}

// Update updates the existing entry in the OwnerCache.
func (s *MeterDefinitionStore) Update(obj interface{}) error {
	// TODO: For now, just call Add, in the future one could check if the resource version changed?
	return s.Add(obj)
}

// Delete deletes an existing entry in the OwnerCache.
func (s *MeterDefinitionStore) Delete(obj interface{}) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	o, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	if _, ok := s.objectResourceSet[ObjectUID(o.GetUID())]; ok {
		delete(s.objectResourceSet, ObjectUID(o.GetUID()))
	}

	return nil
}

// List implements the List method of the store interface.
func (s *MeterDefinitionStore) List() []interface{} {
	return nil
}

// ListKeys implements the ListKeys method of the store interface.
func (s *MeterDefinitionStore) ListKeys() []string {
	return nil
}

// Get implements the Get method of the store interface.
func (s *MeterDefinitionStore) Get(obj interface{}) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

// GetByKey implements the GetByKey method of the store interface.
func (s *MeterDefinitionStore) GetByKey(key string) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

// Replace will delete the contents of the store, using instead the
// given list.
func (s *MeterDefinitionStore) Replace(list []interface{}, _ string) error {
	s.mutex.Lock()
	s.objectResourceSet = map[ObjectUID]ResourceSet{}
	s.mutex.Unlock()

	for _, o := range list {
		err := s.Add(o)
		if err != nil {
			return err
		}
	}

	return nil
}

// Resync implements the Resync method of the store interface.
func (s *MeterDefinitionStore) Resync() error {
	return nil
}

type MeterDefinitionLookupFilter struct {
	workloads map[string]v1alpha1.Workload
	filters   map[string]FilterRuntimeObject
}

func (f *MeterDefinitionLookupFilter) FindMatchingWorkloads(obj interface{}) (*v1alpha1.Workload, error) {
	for key, filter := range f.filters {
		ans, err := filter.Filter(obj)

		if err != nil {
			return nil, err
		}

		if ans {
			workload, ok := f.workloads[key]

			if !ok {
				return nil, errors.NewWithDetails("matched on a workload but workload name not found", "name", name)
			}

			return &workload, nil
		}
	}

	return nil, nil
}

type FilterRuntimeObject interface {
	Filter(interface{}) (bool, error)
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

type WorkloadTypeFilter struct {
	gvk common.GroupVersionKind
}

func (f *WorkloadTypeFilter) Filter(obj interface{}) (bool, error) {
	typeMeta, err := meta.TypeAccessor(obj)

	if err != nil {
		return false, err
	}

	return f.gvk.APIVersion == typeMeta.GetAPIVersion() && f.gvk.Kind == typeMeta.GetKind(), nil
}

type WorkloadComposedFilters struct {
	filters []FilterRuntimeObject
}

func (f *WorkloadComposedFilters) Filter(obj interface{}) (bool, error) {
	for _, filter := range f.filters {
		ans, err := filter.Filter(obj)

		if err != nil {
			return false, err
		}

		if !ans {
			return ans, err
		}
	}

	return true, nil
}

type WorkloadFilterForOwner struct {
	workload  v1alpha1.Workload
	findOwner FindOwnerHelper
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

	if owner.APIVersion == f.workload.Owner.APIVersion && owner.Kind == f.workload.Owner.Kind {
		return true, nil
	}

	namespace := meta.GetNamespace()
	var err error
	var serviceDefOwner *metav1.OwnerReference

	for i := 0; i < 5; i++ {
		var lookupObj runtime.Object

		gvk := schema.FromAPIVersionAndKind(owner.APIVersion, owner.Kind)

		lookupObj, err = scheme.Scheme.new(gvk)
		owner, err = f.findOwner.FindOwner(owner.Name, namespace, lookupObj)

		if err != nil {
			return false, err
		}

		if owner == nil {
			return false, nil
		}

		if owner.APIVersion == f.workload.Owner.APIVersion && owner.Kind == f.workload.Owner.Kind {
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

	return f.labelSelector.Matches(labels.Labels(meta.GetLabels())), nil
}

type WorkloadAnnotationFilter struct {
	annotationSelector labels.Selector
}

func (f *WorkloadAnnotationFilter) Filter(obj metav1.Object) (bool, error) {
	meta, ok := obj.(metav1.Object)

	if !ok {
		return false, errors.New("type was not a metav1.Object")
	}

	return f.annotationSelector.Matches(labels.Labels(meta.GetAnnotations())), nil
}

func (s *MeterDefinitionStore) FindNamespaces(
	instance *v1alpha.MeterDefinition,
) (namespaces []string, err error) {
	cc := s.cc
	functionError := errors.NewWithDetails("error with findNamespaces", "meterdef", instance.Name+"/"+instance.Namespace)
	reqLogger := s.log.WithFields("func", "findNamespaces", "meterdef", instance.Name+"/"+instance.Namespace)

	switch instance.Spec.WorkloadVertex {
	case v1alpha1.WorkloadVertexOperatorGroup:
		reqLogger.Info("operatorGroup vertex")
		csv := &olmv1.ClusterServiceVersion{}

		if instance.Spec.InstalledBy == nil {
			err = errors.Wrap(functionError, "installed by is not found")
			reqLogger.Error(err, "installed by is not found")
			return
		}

		result, _ := cc.Do(context.TODO(),
			GetAction(instance.Spec.InstalledBy.ToTypes(), csv),
		)

		if !result.Is(Continue) {
			err = errors.Wrap(functionError, "csv not found")
			reqLogger.Error(err, "installed by is not found")

			return
		}

		olmNamespacesStr, ok := csv.GetAnnotations()["olm.targetNamespaces"]

		if !ok {
			err = errors.Wrap(functionError, "olmNamspaces on CSV not found")
			// set condition and requeue for later
			reqLogger.Error(err)
			return
		}

		if olmNamespacesStr == "" {
			reqLogger.Info("operatorGroup is for all namespaces")
			namespaces = []string{corev1.NamespaceAll}
			return
		}

		namespaces = strings.Split(olmNamespacesStr, ",")
		return
	case v1alpha1.WorkloadVertexNamespace:
		reqLogger.Info("namespace vertex with filter")

		if instance.Spec.VertexLabelSelectors == nil || len(instance.Spec.VertexLabelSelectors) == 0 {
			reqLogger.Info("namespace vertex is for all namespaces")
			break
		}

		namespaceList := &corev1.NamespaceList{}

		result, _ := cc.Do(context.TODO(),
			ListAction(namespaceList, instance.Spec.VertexLabelSelector),
		)

		if !result.Is(Continue) {
			err = errors.Wrap(functionError, "csv not found")
			reqLogger.Info("csv not found", "csv", instance.Spec.InstalledBy)

			return
		}

		for _, ns := range namespaceList {
			namespaces = append(namespaces, ns.GetName())
		}
	}

	return
}

func (s *MeterDefinitionStore) CreateFilters(
	instance *v1alpha1.MeterDefinition,
	namespaces []string,
) (map[string]FilterRuntimeObject, error) {

	// Bottom Up
	// Start with pods, filter, go to owner. If owner not provided, stop.
	filters := make(map[string]FilterRuntimeObject)

	for _, workload := range instance.Spec.Workloads {
		runtimeFilters := []FilterRuntimeObject{&WorkloadNamespaceFilter{namespaces: namespace}}

		var err error
		typeFilter := &WorkloadTypeFilter{}
		switch workload.WorkloadType {
		case v1alpha1.WorkloadTypePod:
			typeFilter.gvk, err = common.NewGroupVersionKind(&corev1.Pod{})
		case v1alpha1.WorkloadTypePVC:
			typeFilter.gvk, err = common.NewGroupVersionKind(&corev1.PersistentVolumeClaim{})
		case v1alpha1.WorkloadTypeServiceMonitor:
			typeFilter.gvk, err = common.NewGroupVersionKind(&monitoringv1.ServiceMonitor{})
		default:
			err = errors.NewWithDetails("unknown type filter", "type", workload.WorkloadType)
		}

		if err != nil {
			return nil, err
		}

		runtimeFilters = append(runtimeFilters, typeFilter)

		if workload.LabelSelector != nil {
			selector := labels.NewSelector()

			if workload.LabelSelector != nil {
				selector.Add(labels.Set(workload.LabelSelector.MatchLabels))
			}

			for _, exp := range workload.LabelSelector.MatchExpressions {
				selector.Add(labels.NewRequirement(exp.Key, exp.Operator, exp.Values))
			}

			runtimeFilters = append(runtimeFilters, &WorkloadLabelFilter{
				labelSelector: selector,
			})
		}

		if workload.AnnotationSelector != nil {
			selector := labels.NewSelector()

			if workload.AnnotationSelector != nil {
				selector.Add(labels.Set(workload.AnnotationSelector.MatchLabels))
			}

			for _, exp := range workload.AnnotationSelector.MatchExpressions {
				selector.Add(labels.NewRequirement(exp.Key, exp.Operator, exp.Values))
			}

			runtimeFilters = append(runtimeFilters, &WorkloadAnnotationFilter{
				annotationSelector: *workload.AnnotationSelector,
			})
		}

		if workload.Owner != nil {
			runtimeFilters = append(runtimeFilters, &WorkloadOwnerFilter{
				workload:  workload,
				findOwner: rhmclient.FindOwner{cc: s.cc},
			})
		}

		filters[workload.Name] = &WorkloadComposedFilter{filters: runtimeFilters}
	}
	return filters
}

func (s *MeterDefinitionStore) Start() {
	for _, ns := range s.namespaces {
		for expectedType, lister := range s.createWatchers(ns) {
			reflect := cache.NewReflector(lister, expectedType, s, 0)
			go reflector.Run(s.ctx.Done())
		}
	}
}

func (s *MeterDefinitionStore) createWatchers(ns string) map[runtime.Object]cache.ListerWatcher {
	return map[runtime.Object]cache.ListerWatcher{
		&corev1.PersistentVolumeClaim{}: CreatePVCListWatch(s.kubeClient, ns),
		&corev1.Pod{}:                   CreatePodListWatch(s.kubeClient, ns),
		&monitoringv1.ServiceMonitor{}:  CreateServiceMonitorListWatch(s.monitoringClient, ns),
		&v1alpha1.MeterDefinition{}:     CreateMeterDefinitionWatch(s.marketplaceClient, ns)
		//CreateServiceListWatch(s.kubeClient, ns),
	}
}

func CreatePVCListWatch(kubeClient clientset.Interface, ns string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return kubeClient.CoreV1().PersistentVolumeClaims(ns).List(context.TODO(), opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return kubeClient.CoreV1().PersistentVolumeClaims(ns).Watch(context.TODO(), opts)
		},
	}
}

func CreatePodListWatch(kubeClient clientset.Interface, ns string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return kubeClient.CoreV1().Pods(ns).List(context.TODO(), opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return kubeClient.CoreV1().Pods(ns).Watch(context.TODO(), opts)
		},
	}
}

func CreateServiceMonitorListWatch(c *monitoringv1client.MonitoringV1Client, ns string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return c.ServiceMonitors(ns).List(context.TODO(), opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return c.ServiceMonitors(ns).Watch(context.TODO(), opts)
		},
	}
}

func CreateServiceListWatch(kubeClient clientset.Interface, ns string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return kubeClient.CoreV1().Services(ns).List(context.TODO(), opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return kubeClient.CoreV1().Services(ns).Watch(context.TODO(), opts)
		},
	}
}

func CreateMeterDefinitionWatch(c *marketplacev1alpha1client.MarketplaceV1alpha1Client) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return c.MeterDefinitions(ns).List(context.TODO(), opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return c.MeterDefinitions(ns).Watch(context.TODO(), opts)
		},
	}
}
