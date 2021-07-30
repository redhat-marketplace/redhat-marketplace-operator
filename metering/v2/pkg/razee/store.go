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

package meterdefinition

import (
	"context"
	"fmt"
	"sync"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	monitoringv1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/dictionary"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/filter"
	pkgtypes "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	marketplacev1beta1client "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/typed/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type RazeeStores = map[string]*RazeeStore
type ObjectsSeenStore cache.Store

type RazeeStore struct {
	indexStore  cache.Indexer
	delta       *cache.DeltaFIFO
	objectsSeen ObjectsSeenStore
	keyFunc     cache.KeyFunc

	ctx    context.Context
	log    logr.Logger
	scheme *runtime.Scheme

	sync.RWMutex

	// kubeClient to query kube
	kubeClient clientset.Interface

	findOwner *rhmclient.FindOwnerHelper
}

var _ cache.Queue = &RazeeStore{}

// Implementing k8s.io/client-go/tools/cache.Store interface

// Add inserts adds to the OwnerCache by calling the metrics generator functions and
// adding the generated metrics to the metrics map that underlies the MetricStore.
func (s *MeterDefinitionStore) Add(obj interface{}) error {
	key, err := s.keyFunc(obj)

	if err != nil {
		s.log.Error(err, "cannot create a key")
		return err
	}

	if err := s.objectsSeen.Add(obj); err != nil {
		s.log.Error(err, "cannot add object seen")
	}

	logger := s.log.WithValues("func", "add", "namespace/name", key).V(4)
	logger.Info("adding object", "type", fmt.Sprintf("%T", obj))

	// look over all meterDefinitions, matching workloads are saved
	results := []filter.Result{}

	err = s.dictionary.FindObjectMatches(obj, &results, false)
	if err != nil {
		logger.Error(err,
			"failed to find object matches",
			errors.GetDetails(err)...)
		return err
	}

	if len(results) == 0 {
		logger.Info("no results returned")
		return nil
	}

	meterDefs := []v1beta1.MeterDefinition{}

	for _, result := range results {
		if !result.Ok {
			logger.Info("no match", "obj", obj)
			continue
		}

		mdef := *result.Lookup.MeterDefinition
		logger.Info("result", "name", mdef.GetName())
		meterDefs = append(meterDefs, mdef)
	}

	if len(meterDefs) == 0 {
		logger.Info("no matched meterdefs returned")
		return nil
	}

	logger.Info("return meterdefs results", "len", len(meterDefs))
	mdefObj, err := newMeterDefinitionExtended(obj)

	if err != nil {
		return err
	}

	mdefObj.MeterDefinitions = meterDefs

	s.Lock()
	defer s.Unlock()

	if err := s.delta.Add(mdefObj); err != nil {
		logger.Error(err, "failed to add to delta store")
		return err
	}

	if err := s.indexStore.Add(mdefObj); err != nil {
		logger.Error(err, "failed to add to index store")
		return err
	}

	return nil
}

// Update updates the existing entry in the OwnerCache.
func (s *MeterDefinitionStore) Update(obj interface{}) error {
	key, err := s.keyFunc(obj)

	if err != nil {
		s.log.Error(err, "cannot create a key")
		return err
	}

	if err := s.objectsSeen.Update(obj); err != nil {
		s.log.Error(err, "error updating seen object")
	}

	logger := s.log.WithValues("func", "add", "namespace/name", key).V(4)
	logger.Info("updating obj")

	// look over all meterDefinitions, matching workloads are saved
	results := []filter.Result{}

	err = s.dictionary.FindObjectMatches(obj, &results, false)
	if err != nil {
		logger.Error(err,
			"failed to find object matches",
			errors.GetDetails(err)...)
		return err
	}

	if len(results) == 0 {
		logger.Info("no results returned")
		return nil
	}

	meterDefs := []v1beta1.MeterDefinition{}

	for _, result := range results {
		if !result.Ok {
			logger.Info("no match", "obj", obj)
			continue
		}

		mdef := result.Lookup.MeterDefinition
		logger.Info("result", "name", mdef.GetName())
		meterDefs = append(meterDefs, *mdef)
	}

	if len(meterDefs) == 0 {
		logger.Info("no matched meterdefs returned")
		return nil
	}

	logger.Info("return meterdefs results", "len", len(meterDefs))

	mdefObj, err := newMeterDefinitionExtended(obj)

	if err != nil {
		return err
	}

	mdefObj.MeterDefinitions = meterDefs

	s.Lock()
	defer s.Unlock()

	if err := s.delta.Update(mdefObj); err != nil {
		logger.Error(err, "failed to add to delta store")
		return err
	}

	if err := s.indexStore.Update(mdefObj); err != nil {
		logger.Error(err, "failed to add to index store")
		return err
	}

	return nil
}

// Delete deletes an existing entry in the OwnerCache.
func (s *MeterDefinitionStore) Delete(obj interface{}) error {
	mdefObj, err := newMeterDefinitionExtended(obj)

	if err != nil {
		s.log.Error(err, "cannot create a key")
		return err
	}

	if err := s.objectsSeen.Delete(obj); err != nil {
		s.log.Error(err, "error deleting seen object")
	}

	key, err := s.keyFunc(mdefObj)

	if err != nil {
		s.log.Error(err, "cannot create a key")
		return err
	}

	logger := s.log.WithValues("func", "delete",
		"name", mdefObj.GetName(),
		"namespace", mdefObj.GetNamespace(),
		"key", key)

	fullObj, found, err := s.indexStore.Get(mdefObj)
	if err != nil {
		s.log.Error(err, "cannot get")
		return err
	}

	if found {
		logger.Info("deleting obj")

		s.Lock()
		defer s.Unlock()
		if err := s.delta.Delete(fullObj); err != nil {
			logger.Error(err, "can't delete obj")
			return err
		}

		if err := s.indexStore.Delete(fullObj); err != nil {
			logger.Error(err, "can't delete obj")
			return err
		}
	}

	return nil
}

// List implements the List method of the store interface.
func (s *MeterDefinitionStore) List() []interface{} {
	s.RLock()
	defer s.RUnlock()
	return s.indexStore.List()
}

func (s *MeterDefinitionStore) AddIfNotPresent(obj interface{}) error {
	return s.delta.AddIfNotPresent(obj)
}

func (s *MeterDefinitionStore) Close() {
	s.delta.Close()
}

func (s *MeterDefinitionStore) HasSynced() bool {
	return s.delta.HasSynced()
}

func (s *MeterDefinitionStore) Pop(process cache.PopProcessFunc) (interface{}, error) {
	return s.delta.Pop(process)
}

func (s *MeterDefinitionStore) ByIndex(indexName, indexedValue string) ([]interface{}, error) {
	s.RLock()
	defer s.RUnlock()
	return s.indexStore.ByIndex(indexName, indexedValue)
}

// ListKeys implements the ListKeys method of the store interface.
func (s *MeterDefinitionStore) ListKeys() []string {
	s.RLock()
	defer s.RUnlock()
	return s.indexStore.ListKeys()
}

// Get implements the Get method of the store interface.
func (s *MeterDefinitionStore) Get(obj interface{}) (item interface{}, exists bool, err error) {
	s.RLock()
	defer s.RUnlock()

	mdefObj, err := newMeterDefinitionExtended(obj)

	if err != nil {
		return nil, false, err
	}

	return s.indexStore.Get(mdefObj)
}

// GetByKey implements the GetByKey method of the store interface.
func (s *MeterDefinitionStore) GetByKey(key string) (item interface{}, exists bool, err error) {
	s.RLock()
	defer s.RUnlock()
	return s.indexStore.GetByKey(key)
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
	list := s.objectsSeen.List()
	for i := range list {
		obj := list[i]
		_, exists, err := s.Get(obj)

		if err != nil {
			s.log.Error(err, "failed to get")
			return err
		}

		key, _ := s.keyFunc(obj.(runtime.Object))
		if !exists {
			s.log.Info("adding obj from objectsSeen", "obj", key)
			if err := s.Add(obj); err != nil {
				s.log.Error(err, "failed to add")
				return err
			}
		} else {
			s.log.Info("updating obj from objectsSeen", "obj", key)
			if err := s.Update(obj); err != nil {
				s.log.Error(err, "failed to add")
				return err
			}
		}
	}

	seenKeys := s.objectsSeen.ListKeys()
	if len(seenKeys) == 0 {
		return nil
	}

	s.log.Info("objects seen keys", "keys", seenKeys)
	seenKeysMap := make(map[string]interface{})

	for j := range seenKeys {
		seenKeysMap[seenKeys[j]] = nil
	}

	keys := s.ListKeys()
	for i := range keys {
		key := keys[i]
		enobj, _, err := s.GetByKey(key)

		if err != nil {
			s.log.Error(err, "failed to get")
			return err
		}

		_, exists := seenKeysMap[key]

		if !exists {
			s.log.Info("deleting object from objectsseen", "obj", key)
			err := s.Delete(enobj)
			if err != nil {
				s.log.Error(err, "failed to delete")
				return err
			}
		}
	}

	return nil
}

func newMeterDefinitionExtended(obj interface{}) (*pkgtypes.MeterDefinitionEnhancedObject, error) {
	if v, ok := obj.(*pkgtypes.MeterDefinitionEnhancedObject); ok {
		return v, nil
	}

	if v, ok := obj.(metav1.Object); ok {
		return &pkgtypes.MeterDefinitionEnhancedObject{
			Object: v,
		}, nil
	}

	return nil, errors.New(fmt.Sprintf("can't convert %T to meterdefinition extended", obj))
}

func NewMeterDefinitionStore(
	ctx context.Context,
	log logr.Logger,
	kubeClient clientset.Interface,
	findOwner *rhmclient.FindOwnerHelper,
	monitoringClient *monitoringv1client.MonitoringV1Client,
	marketplaceclientV1beta1 *marketplacev1beta1client.MarketplaceV1beta1Client,
	dictionary *dictionary.MeterDefinitionDictionary,
	scheme *runtime.Scheme,
) *MeterDefinitionStore {

	store := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.MetaNamespaceIndexFunc)
	delta := cache.NewDeltaFIFO(cache.MetaNamespaceKeyFunc, store)
	return &MeterDefinitionStore{
		ctx:                      ctx,
		log:                      log.WithName("obj_store").V(4),
		scheme:                   scheme,
		kubeClient:               kubeClient,
		monitoringClient:         monitoringClient,
		marketplaceClientV1beta1: marketplaceclientV1beta1,
		dictionary:               dictionary,
		findOwner:                findOwner,
		delta:                    delta,
		indexStore:               store,
		objectsSeen:              NewObjectsSeenStore(scheme),
		keyFunc:                  keyFunc,
	}
}

func NewObjectsSeenStore(
	scheme *runtime.Scheme,
) ObjectsSeenStore {
	return cache.NewStore(pkgtypes.GVKNamespaceKeyFunc(scheme))
}
