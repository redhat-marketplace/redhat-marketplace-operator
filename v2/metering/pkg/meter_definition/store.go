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
	"time"

	"emperror.dev/errors"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/client"
	marketplacev1alpha1client "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/typed/marketplace/v1alpha1"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	"github.com/sasha-s/go-deadlock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MeterDefinitionStores = map[string]*MeterDefinitionStore

// MeterDefinitionStore keeps the MeterDefinitions in place
// and tracks the dependents using the rules based on the
// rules. MeterDefinition controller uses this to effectively
// find the child assets of a meter definition rules.
type MeterDefinitionStore struct {
	meterDefinitionFilters map[MeterDefUID]*MeterDefinitionLookupFilter
	objectResourceSet      map[ObjectResourceKey]*ObjectResourceValue
	objectsSeen            map[ObjectUID]interface{}

	mutex deadlock.Mutex

	ctx    context.Context
	log    logr.Logger
	scheme *runtime.Scheme

	// client command runner to execute some code
	cc ClientCommandRunner

	// namespaces to listen to
	namespaces []string

	// kubeClient to query kube
	kubeClient        clientset.Interface
	findOwner         *rhmclient.FindOwnerHelper
	monitoringClient  *monitoringv1client.MonitoringV1Client
	marketplaceClient *marketplacev1alpha1client.MarketplaceV1alpha1Client

	// listeners are used for downstream
	listenerMutex deadlock.Mutex
	listeners     []chan *ObjectResourceMessage

	// resyncObjChan will resync the store
	resyncObjChan chan interface{}
}

type MeterDefinitionStoreBuilder struct {
	ctx    context.Context
	log    logr.Logger
	scheme *runtime.Scheme

	// client command runner to execute some code
	cc ClientCommandRunner

	// namespaces to listen to
	namespaces []string

	// kubeClient to query kube
	kubeClient        clientset.Interface
	findOwner         *rhmclient.FindOwnerHelper
	monitoringClient  *monitoringv1client.MonitoringV1Client
	marketplaceClient *marketplacev1alpha1client.MarketplaceV1alpha1Client
}

func NewMeterDefinitionStoreBuilder(
	ctx context.Context,
	log logr.Logger,
	cc ClientCommandRunner,
	kubeClient clientset.Interface,
	findOwner *rhmclient.FindOwnerHelper,
	monitoringClient *monitoringv1client.MonitoringV1Client,
	marketplaceclient *marketplacev1alpha1client.MarketplaceV1alpha1Client,
	scheme *runtime.Scheme,
) *MeterDefinitionStoreBuilder {
	return &MeterDefinitionStoreBuilder{
		ctx:               ctx,
		log:               log,
		cc:                cc,
		kubeClient:        kubeClient,
		monitoringClient:  monitoringClient,
		marketplaceClient: marketplaceclient,
		findOwner:         findOwner,
		scheme:            scheme,
	}
}

func (s *MeterDefinitionStoreBuilder) NewInstance() *MeterDefinitionStore {
	return &MeterDefinitionStore{
		ctx:                    s.ctx,
		log:                    s.log,
		cc:                     s.cc,
		scheme:                 s.scheme,
		kubeClient:             s.kubeClient,
		monitoringClient:       s.monitoringClient,
		marketplaceClient:      s.marketplaceClient,
		findOwner:              s.findOwner,
		namespaces:             s.namespaces,
		mutex:                  deadlock.Mutex{},
		listenerMutex:          deadlock.Mutex{},
		resyncObjChan:          make(chan interface{}),
		objectsSeen:            make(map[ObjectUID]interface{}),
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
	toDelete := []ObjectResourceKey{}

	for key, val := range s.objectResourceSet {
		if key.MeterDefUID == MeterDefUID(meterdef.GetUID()) {
			toDelete = append(toDelete, key)
			s.broadcast(&ObjectResourceMessage{
				Action: DeleteMessageAction,
				Object: val.Object,
			})
		}
	}

	for _, key := range toDelete {
		delete(s.objectResourceSet, key)
	}

	s.broadcast(&ObjectResourceMessage{
		Action: DeleteMessageAction,
		Object: meterdef,
	})
}

func (s *MeterDefinitionStore) broadcast(msg *ObjectResourceMessage) {
	for _, ch := range s.listeners {
		select {
		case ch <- msg:
			s.log.V(3).Info("sent message", "msg", msg)
		}
	}
}

func (s *MeterDefinitionStore) GetMeterDefinitionRefs(uid types.UID) []*ObjectResourceValue {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	vals := []*ObjectResourceValue{}
	for key, val := range s.objectResourceSet {
		if key.ObjectUID == ObjectUID(uid) && val.Matched {
			vals = append(vals, val)
		}
	}
	return vals
}

func (s *MeterDefinitionStore) GetMeterDefObjects(meterDefUID types.UID) []*ObjectResourceValue {
	s.mutex.Lock()
	defer s.mutex.Unlock()

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
	runtimeObj, ok := obj.(runtime.Object)

	if !ok {
		err := errors.New("obj not a runtime.Object")
		s.log.Error(err, "obj not a runtime object")
		return err
	}

	key, _ := client.ObjectKeyFromObject(runtimeObj)
	logger := s.log.WithValues("func", "add", "name/namespace", key)
	logger.V(2).Info("adding obj")

	if meterdef, ok := obj.(*v1alpha1.MeterDefinition); ok {
		return s.handleMeterDefinition(meterdef)
	}

	// save obj to objectsSeen
	err := s.addSeenObject(obj)
	if err != nil {
		logger.Error(err, "failed to add to seen object list")
		return err
	}

	// look over all meterDefinitions, matching workloads are saved
	results := []result{}

	err = s.findObjectMatches(obj, &results)
	if err != nil {
		logger.Error(err, "failed get find object matches")
		return err
	}

	if len(results) == 0 {
		logger.V(2).Info("no results returned")
		return nil
	}

	matchedResults := []result{}
	for _, result := range results {
		if !result.ok {
			logger.V(4).Info("no match", "obj", obj, "meterDefUID", string(result.meterDefUID))
			continue
		}

		matchedResults = append(matchedResults, result)
	}

	if len(matchedResults) == 0 {
		logger.V(2).Info("no matched results returned")
		return nil
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	logger.Info("return matched results", len(matchedResults))

	for _, result := range matchedResults {
		resource, err := v1alpha1.NewWorkloadResource(*result.workload, obj, s.scheme)
		if err != nil {
			logger.Error(err, "failed to init a new workload resource")
			return err
		}

		value, err := NewObjectResourceValue(result.lookup, resource, obj, result.ok)
		if err != nil {
			logger.Error(err, "failed to init a new workload resource value")
			return err
		}

		s.objectResourceSet[result.key] = value

		msg := &ObjectResourceMessage{
			Action:              AddMessageAction,
			Object:              obj,
			ObjectResourceValue: value,
		}

		logger.Info("broadcasting message", "msg", msg,
			"type", fmt.Sprintf("%T", obj),
			"mdef", value.MeterDef,
			"workloadName", value.WorkloadResource.Name)
		s.broadcast(msg)
	}

	return nil
}

func (s *MeterDefinitionStore) handleMeterDefinition(meterdef *v1alpha1.MeterDefinition) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// remove meterdefs that don't fit the type
	s.log.Info("adding meterdef", "name", meterdef.Name, "namespace", meterdef.Namespace)
	lookup, err := NewMeterDefinitionLookupFilter(s.cc, meterdef, s.findOwner)

	if err != nil {
		s.log.Error(err, "error building lookup")
		return err
	}

	s.log.Info("found lookup", "lookup", lookup)
	s.meterDefinitionFilters[MeterDefUID(meterdef.UID)] = lookup

	msg := &ObjectResourceMessage{
		Action: AddMessageAction,
		Object: interface{}(meterdef),
	}

	s.log.Info("broadcasting meterdef message", "msg", msg)
	s.broadcast(msg)

	return nil
}

func (s *MeterDefinitionStore) addSeenObject(obj interface{}) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	o, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	uid := ObjectUID(o.GetUID())
	s.objectsSeen[uid] = obj
	return nil
}

func (s *MeterDefinitionStore) findObjectMatches(obj interface{}, results *[]result) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	o, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	for meterDefUID, lookup := range s.meterDefinitionFilters {
		key := NewObjectResourceKey(o, meterDefUID)
		workload, ok, err := lookup.FindMatchingWorkloads(obj)

		if err != nil {
			s.log.Error(err, "error matching")
			return err
		}

		*results = append(*results, result{
			meterDefUID: meterDefUID,
			workload:    workload,
			ok:          ok,
			lookup:      lookup,
			key:         key,
		})
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

	delete(s.objectsSeen, ObjectUID(o.GetUID()))

	if meterdef, ok := obj.(*v1alpha1.MeterDefinition); ok {
		s.removeMeterDefinition(meterdef)
		return nil
	}

	for key, _ := range s.objectResourceSet {
		if key.ObjectUID == ObjectUID(o.GetUID()) {
			delete(s.objectResourceSet, key)
		}
	}

	s.broadcast(&ObjectResourceMessage{
		Action: DeleteMessageAction,
		Object: obj,
	})

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
	for _, o := range list {
		if err := s.Delete(o); err != nil {
			return err
		}

		if err := s.Add(o); err != nil {
			return err
		}
	}

	return nil
}

// Resync implements the Resync method of the store interface.
func (s *MeterDefinitionStore) Resync() error {
	s.mutex.Lock()
	objs := []interface{}{}
	for _, obj := range s.objectsSeen {
		objs = append(objs, obj)
	}
	s.mutex.Unlock()

	for _, obj := range objs {
		s.Add(obj)
	}

	return nil
}

func (s *MeterDefinitionStore) Start() {
	go func() {
		select {
		case obj := <-s.resyncObjChan:
			err := s.Add(obj)

			if err != nil {
				s.log.Error(err, "resyncing state error")
			}
		case <-s.ctx.Done():
			close(s.resyncObjChan)
			return
		}
	}()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for {
			select {
			case <-ticker.C:
				func() {
					s.log.Info("current state", "meterDefs", s.meterDefinitionFilters)
				}()
			case <-s.ctx.Done():
				return
			}
		}
	}()
}

func (s *MeterDefinitionStoreBuilder) CreateStores() MeterDefinitionStores {
	stores := make(MeterDefinitionStores)

	for _, storeConfig := range storeConfigs {
		store := s.NewInstance()

		for _, createLister := range storeConfig.createListers {
			for _, ns := range s.namespaces {
				lister := createLister(s, ns)
				reflector := cache.NewReflector(lister.lister, lister.expectedType, store, 5*60*time.Second)
				go reflector.Run(s.ctx.Done())
			}
		}

		go store.Start()
		stores[storeConfig.name] = store
	}

	return stores
}

func (s *MeterDefinitionStoreBuilder) SetNamespaces(ns []string) {
	s.namespaces = ns
}

type storeConfig struct {
	name          string
	createListers []createLister
}

type reflectorConfig struct {
	expectedType runtime.Object
	lister       cache.ListerWatcher
}

type createLister = func(*MeterDefinitionStoreBuilder, string) reflectorConfig

const (
	ServiceStore          string = "serviceStore"
	PodStore                     = "podStore"
	PersistentVolumeStore        = "pvcStore"
)

var (
	storeConfigs []storeConfig = []storeConfig{pvcStore, podStore, serviceStore}
	pvcStore     storeConfig   = storeConfig{
		name: PersistentVolumeStore,
		createListers: []createLister{
			pvcLister, meterDefLister,
		},
	}
	podStore = storeConfig{
		name: PodStore,
		createListers: []createLister{
			podLister, meterDefLister,
		},
	}
	serviceStore = storeConfig{
		name: ServiceStore,
		createListers: []createLister{
			serviceLister, serviceMonitorLister, meterDefLister,
		},
	}
)

func pvcLister(s *MeterDefinitionStoreBuilder, ns string) reflectorConfig {
	return reflectorConfig{
		expectedType: &corev1.PersistentVolumeClaim{},
		lister:       CreatePVCListWatch(s.kubeClient, ns),
	}
}

func podLister(s *MeterDefinitionStoreBuilder, ns string) reflectorConfig {
	return reflectorConfig{
		expectedType: &corev1.Pod{},
		lister:       CreatePodListWatch(s.kubeClient, ns),
	}
}

func serviceLister(s *MeterDefinitionStoreBuilder, ns string) reflectorConfig {
	return reflectorConfig{
		expectedType: &corev1.Service{},
		lister:       CreateServiceListWatch(s.kubeClient, ns),
	}
}

func serviceMonitorLister(s *MeterDefinitionStoreBuilder, ns string) reflectorConfig {
	return reflectorConfig{
		expectedType: &monitoringv1.ServiceMonitor{},
		lister:       CreateServiceMonitorListWatch(s.monitoringClient, ns),
	}
}

func meterDefLister(s *MeterDefinitionStoreBuilder, ns string) reflectorConfig {
	return reflectorConfig{
		expectedType: &v1alpha1.MeterDefinition{},
		lister:       CreateMeterDefinitionWatch(s.marketplaceClient, ns),
	}
}
