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

package meter_definition

import (
	"context"
	"fmt"
	"sync"
	"time"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1client "github.com/coreos/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/client"
	marketplacev1alpha1client "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/generated/clientset/versioned/typed/marketplace/v1alpha1"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type ObjectUID types.UID
type MeterDefUID types.UID
type ResourceSet map[MeterDefUID]*v1alpha1.WorkloadResource
type ObjectResourceMessageAction string

const (
	AddMessageAction    ObjectResourceMessageAction = "Add"
	DeleteMessageAction                             = "Delete"
)

type ObjectResourceMessage struct {
	Action               ObjectResourceMessageAction `json:"action"`
	Object               interface{}                 `json:"object"`
	*ObjectResourceValue `json:"resourceValue,omitempty"`
}

type ObjectResourceKey struct {
	ObjectUID
	MeterDefUID
}

func NewObjectResourceKey(object metav1.Object, meterdefUID MeterDefUID) ObjectResourceKey {
	return ObjectResourceKey{
		ObjectUID:   ObjectUID(object.GetUID()),
		MeterDefUID: meterdefUID,
	}
}

type ObjectResourceValue struct {
	MeterDef     types.NamespacedName
	MeterDefHash string
	Generation   int64
	Matched      bool
	*v1alpha1.WorkloadResource
}

func NewObjectResourceValue(
	lookup *MeterDefinitionLookupFilter,
	resource *v1alpha1.WorkloadResource,
	obj metav1.Object,
	matched bool,
) *ObjectResourceValue {
	return &ObjectResourceValue{
		MeterDef:         lookup.MeterDefName,
		MeterDefHash:     lookup.Hash(),
		WorkloadResource: resource,
		Generation:       obj.GetGeneration(),
		Matched:          matched,
	}
}

// MeterDefinitionStore keeps the MeterDefinitions in place
// and tracks the dependents using the rules based on the
// rules. MeterDefinition controller uses this to effectively
// find the child assets of a meter definition rules.
type MeterDefinitionStore struct {
	meterDefinitionFilters map[MeterDefUID]*MeterDefinitionLookupFilter
	objectResourceSet      map[ObjectResourceKey]*ObjectResourceValue

	mutex sync.RWMutex

	ctx    context.Context
	log    logr.Logger
	scheme *runtime.Scheme

	cc ClientCommandRunner

	namespaces []string

	kubeClient        clientset.Interface
	findOwner         *rhmclient.FindOwnerHelper
	monitoringClient  *monitoringv1client.MonitoringV1Client
	marketplaceClient *marketplacev1alpha1client.MarketplaceV1alpha1Client

	listenerMutex sync.Mutex
	listeners     []chan *ObjectResourceMessage
}

func NewMeterDefinitionStore(
	ctx context.Context,
	log logr.Logger,
	cc ClientCommandRunner,
	kubeClient clientset.Interface,
	findOwner *rhmclient.FindOwnerHelper,
	monitoringClient *monitoringv1client.MonitoringV1Client,
	marketplaceclient *marketplacev1alpha1client.MarketplaceV1alpha1Client,
	scheme *runtime.Scheme,
) *MeterDefinitionStore {
	return &MeterDefinitionStore{
		ctx:                    ctx,
		log:                    log,
		cc:                     cc,
		kubeClient:             kubeClient,
		monitoringClient:       monitoringClient,
		marketplaceClient:      marketplaceclient,
		findOwner:              findOwner,
		scheme:                 scheme,
		listeners:              []chan *ObjectResourceMessage{},
		meterDefinitionFilters: make(map[MeterDefUID]*MeterDefinitionLookupFilter),
		objectResourceSet:      make(map[ObjectResourceKey]*ObjectResourceValue),
	}
}

func (s *MeterDefinitionStore) RegisterListener(name string, ch chan *ObjectResourceMessage) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.log.Info("registering listener", "name", name)
	s.listeners = append(s.listeners, ch)
}

func (s *MeterDefinitionStore) addMeterDefinition(meterdef *v1alpha1.MeterDefinition, lookup *MeterDefinitionLookupFilter) {
	s.meterDefinitionFilters[MeterDefUID(meterdef.UID)] = lookup
}

func (s *MeterDefinitionStore) removeMeterDefinition(meterdef *v1alpha1.MeterDefinition) {
	delete(s.meterDefinitionFilters, MeterDefUID(meterdef.UID))
	for key := range s.objectResourceSet {
		if key.MeterDefUID == MeterDefUID(meterdef.GetUID()) {
			delete(s.objectResourceSet, key)
		}
	}
}

func (s *MeterDefinitionStore) broadcast(msg *ObjectResourceMessage) {
	for _, ch := range s.listeners {
		select {
		case ch <- msg:
			s.log.V(3).Info("sent message",  "msg", msg)
		default:
			s.log.V(3).Info("no message sent", "msg", msg)
		}
	}
}

func (s *MeterDefinitionStore) GetMeterDefinitionRefs(uid types.UID) []*ObjectResourceValue {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	vals := []*ObjectResourceValue{}
	for key, val := range s.objectResourceSet {
		if key.ObjectUID == ObjectUID(uid) && val.Matched {
			vals = append(vals, val)
		}
	}
	return vals
}

func (s *MeterDefinitionStore) GetMeterDefObjects(meterDefUID types.UID) []*ObjectResourceValue {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	vals := []*ObjectResourceValue{}
	for key, val := range s.objectResourceSet {
		if key.MeterDefUID == MeterDefUID(meterDefUID) && val.Matched {
			vals = append(vals, val)
		}
	}

	return vals
}

type result struct {
	meterDefUID MeterDefUID
	workload    *v1alpha1.Workload
	ok          bool
	lookup      *MeterDefinitionLookupFilter
	key         ObjectResourceKey
}

// Implementing k8s.io/client-go/tools/cache.Store interface

// Add inserts adds to the OwnerCache by calling the metrics generator functions and
// adding the generated metrics to the metrics map that underlies the MetricStore.
func (s *MeterDefinitionStore) Add(obj interface{}) error {

	if meterdef, ok := obj.(*v1alpha1.MeterDefinition); ok {
		err := func() error {
			s.mutex.Lock()
			defer s.mutex.Unlock()
			lookup, err := NewMeterDefinitionLookupFilter(s.cc, meterdef, s.findOwner)

			if err != nil {
				s.log.Error(err, "error building lookup")
				return err
			}

			s.log.Info("found lookup", "lookup", lookup)
			s.meterDefinitionFilters[MeterDefUID(meterdef.UID)] = lookup
			return nil
		}()

		if err != nil {
			return err
		}

		return nil
	}

	o, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	// look over all meterDefinitions, matching workloads are saved
	results := []result{}

	err = func() error {
		s.mutex.RLock()
		defer s.mutex.RUnlock()
		for meterDefUID, lookup := range s.meterDefinitionFilters {
			key := NewObjectResourceKey(o, meterDefUID)
			workload, ok, err := lookup.FindMatchingWorkloads(obj)

			if err != nil {
				s.log.Error(err, "")
				return err
			}

			results = append(results, result{
				meterDefUID: meterDefUID,
				workload:    workload,
				ok:          ok,
				lookup:      lookup,
				key:         key,
			})
		}
		return nil
	}()

	if err != nil {
		return err
	}

	for _, result := range results {
		if !result.ok {
			continue
		}

		err := func() error {
			s.mutex.Lock()
			defer s.mutex.Unlock()

			log.Info("workload found", "obj", obj, "meterDefUID", string(result.meterDefUID))
			resource, err := v1alpha1.NewWorkloadResource(*result.workload, obj, s.scheme)
			if err != nil {
				s.log.Error(err, "")
				return err
			}

			value := NewObjectResourceValue(result.lookup, resource, o, result.ok)
			s.objectResourceSet[result.key] = value

			msg := &ObjectResourceMessage{
				Action:              AddMessageAction,
				Object:              obj,
				ObjectResourceValue: value,
			}

			log.Info("broadcasting message", "msg", msg, "type", fmt.Sprintf("%T", obj), "mdef", value.MeterDef, "workloadName", value.WorkloadResource.Name)
			s.broadcast(msg)

			return nil
		}()

		if err != nil {
			return err
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

	if meterdef, ok := obj.(*v1alpha1.MeterDefinition); ok {
		s.removeMeterDefinition(meterdef)
		return nil
	}

	o, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	for key, val := range s.objectResourceSet {
		if key.ObjectUID == ObjectUID(o.GetUID()) {
			s.broadcast(&ObjectResourceMessage{
				Action:              DeleteMessageAction,
				Object:              o,
				ObjectResourceValue: val,
			})

			delete(s.objectResourceSet, key)
		}
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
	for _, item := range list {
		o, _ := meta.Accessor(item)

		for key, val := range s.objectResourceSet {
			if key.ObjectUID == ObjectUID(o.GetUID()) {
				s.broadcast(&ObjectResourceMessage{
					Action:              DeleteMessageAction,
					Object:              item,
					ObjectResourceValue: val,
				})

				delete(s.objectResourceSet, key)
			}
		}
	}
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

func (s *MeterDefinitionStore) Start() {
	for _, ns := range s.namespaces {
		lister := CreateMeterDefinitionWatch(s.marketplaceClient, ns)
		reflector := cache.NewReflector(lister, &v1alpha1.MeterDefinition{}, s, 0)
		go reflector.Run(s.ctx.Done())
	}

	time.Sleep(5 * time.Second)

	for _, ns := range s.namespaces {
		for expectedType, lister := range s.createWatchers(ns) {
			reflector := cache.NewReflector(lister, expectedType, s, 0)
			go reflector.Run(s.ctx.Done())
		}
	}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for {
			select {
			case <-ticker.C:
				func() {
					s.mutex.RLock()
					defer s.mutex.RUnlock()

					s.log.Info("current state",
						"meterDefs", s.meterDefinitionFilters)
				}()
			case <-s.ctx.Done():
				return
			}
		}
	}()
}

func (s *MeterDefinitionStore) SetNamespaces(ns []string) {
	s.namespaces = ns
}

func (s *MeterDefinitionStore) createWatchers(ns string) map[runtime.Object]cache.ListerWatcher {
	return map[runtime.Object]cache.ListerWatcher{
		&corev1.PersistentVolumeClaim{}: CreatePVCListWatch(s.kubeClient, ns),
		&corev1.Pod{}:                   CreatePodListWatch(s.kubeClient, ns),
		&corev1.Service{}:               CreateServiceListWatch(s.kubeClient, ns),
		&monitoringv1.ServiceMonitor{}:  CreateServiceMonitorListWatch(s.monitoringClient, ns),
	}
}
