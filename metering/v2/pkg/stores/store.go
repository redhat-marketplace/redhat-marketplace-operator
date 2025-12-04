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

package stores

import (
	"context"
	"fmt"
	"sync"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/filter"
	pkgtypes "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

type MeterDefinitionStores = map[string]*MeterDefinitionStore

// MeterDefinitionStore keeps the MeterDefinitions in place
// and tracks the dependents using the rules based on the
// spec. MeterDefinition controller uses this to effectively
// find the child assets of a meter definition rules.
type MeterDefinitionStore struct {
	dictionary *MeterDefinitionDictionary
	indexStore cache.Indexer
	delta      *cache.DeltaFIFO
	keyFunc    cache.KeyFunc

	ctx    context.Context
	log    logr.Logger
	scheme *runtime.Scheme

	sync.RWMutex
}

var _ cache.Queue = &MeterDefinitionStore{}

const IndexMeterDefinition = "meterDefinition"
const IndexNamespace = "namespace"

func EnhancedObjectKeyFunc(scheme *runtime.Scheme) func(obj interface{}) (string, error) {
	return func(obj interface{}) (string, error) {
		mdefObj, err := newStoreObject(obj)

		if err != nil {
			return "", err
		}

		return pkgtypes.GVKNamespaceKeyFunc(scheme)(mdefObj.Object)
	}
}

func MeterDefinitionIndexFunc(obj interface{}) ([]string, error) {
	v, ok := obj.(*pkgtypes.MeterDefinitionEnhancedObject)
	if !ok {
		return nil, errors.New("failed to index obj")
	}

	keys := make([]string, 0, len(v.MeterDefinitions))
	for i := range v.MeterDefinitions {
		meterDef := v.MeterDefinitions[i]
		key, err := cache.MetaNamespaceKeyFunc(meterDef)
		if !ok {
			return nil, errors.Wrap(err, "failed to get obj key")
		}
		keys = append(keys, key)
	}

	return keys, nil
}

// Implementing k8s.io/client-go/tools/cache.Store interface

// Add inserts adds to the OwnerCache by calling the metrics generator functions and
// adding the generated metrics to the metrics map that underlies the MetricStore.
func (s *MeterDefinitionStore) Add(obj interface{}) error {
	addObj, err := s.addObj(obj)
	if err != nil {
		return err
	}

	s.Lock()
	defer s.Unlock()
	if addObj != nil {
		if len(addObj.MeterDefinitions) != 0 {
			if err := s.delta.Add(addObj); err != nil {
				s.log.Error(err, "failed to update/add to delta store")
				return err
			}
		}
		if err := s.indexStore.Add(addObj); err != nil {
			s.log.Error(err, "failed to update/add to index store")
			return err
		}
	}
	return nil
}

// Create an MeterDefinitionEnhancedObject with matching MeterDefinitions to be added to the store
func (s *MeterDefinitionStore) addObj(obj interface{}) (*pkgtypes.MeterDefinitionEnhancedObject, error) {
	key, err := s.keyFunc(obj)

	if err != nil {
		s.log.Error(err, "cannot create a key")
		return nil, err
	}

	mdefObj, err := newStoreObject(obj)
	if err != nil {
		return nil, err
	}

	logger := s.log.WithValues("func", "addObj", "namespace/name", key)
	logger.Info("adding object", "type", fmt.Sprintf("%T", obj))

	// look over all meterDefinitions, matching workloads are saved
	results := []filter.Result{}

	err = s.dictionary.FindObjectMatches(obj, &results)
	if err != nil {
		logger.Error(err,
			"failed to find object matches",
			errors.GetDetails(err)...)
		return mdefObj, err
	}

	if len(results) == 0 {
		logger.Info("no results returned")
		return mdefObj, nil
	}

	for i := range results {
		result := results[i]

		if !result.Ok {
			logger.Info("no match", "obj", obj)
			continue
		}

		mdef := result.Lookup.MeterDefinition
		logger.Info("result", "name", mdef.GetName())
		mdefObj.MeterDefinitions = append(mdefObj.MeterDefinitions, mdef)
	}

	if len(mdefObj.MeterDefinitions) == 0 {
		logger.Info("no matched meterdefs returned")
		return mdefObj, nil
	}

	logger.Info("return meterdefs results", "len", len(mdefObj.MeterDefinitions))

	return mdefObj, nil
}

// Update updates the existing entry in the OwnerCache.
func (s *MeterDefinitionStore) Update(obj interface{}) error {
	addObj, err := s.addObj(obj)
	if err != nil {
		return err
	}

	s.Lock()
	defer s.Unlock()

	if addObj != nil {
		item, exists, err := s.indexStore.Get(addObj)
		if exists && err == nil {
			// Skip Updates where Generation does not change
			prevObj, ok := item.(*pkgtypes.MeterDefinitionEnhancedObject)
			if ok {
				if addObj.Object.GetGeneration() == prevObj.Object.GetGeneration() {
					return nil
				}
			}

			// Emit a delta delete for processing if the object no longer matches a meterdefinition
			if len(addObj.MeterDefinitions) != 0 {
				if err := s.delta.Add(addObj); err != nil {
					s.log.Error(err, "failed to update/add to delta store")
					return err
				}
			} else {
				if err := s.delta.Delete(addObj); err != nil {
					s.log.Error(err, "failed to update/delete to delta store")
					return err
				}
			}

			if err := s.indexStore.Add(addObj); err != nil {
				s.log.Error(err, "failed to update/add to index store")
				return err
			}
		}
	}

	return nil
}

// Delete deletes an existing entry in the OwnerCache.
func (s *MeterDefinitionStore) Delete(obj interface{}) error {
	s.Lock()
	defer s.Unlock()

	mdefObj, err := newStoreObject(obj)

	if err != nil {
		s.log.Error(err, "cannot create a key")
		return err
	}

	key, err := s.keyFunc(mdefObj)

	if err != nil {
		s.log.Error(err, "cannot create a key")
		return err
	}

	logger := s.log.V(2).WithValues("func", "delete",
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

// Delete deletes an existing entry in the OwnerCache.
func (s *MeterDefinitionStore) DeleteFromIndex(obj interface{}) error {
	mdefObj, err := newStoreObject(obj)

	if err != nil {
		s.log.Error(err, "cannot create a key")
		return err
	}

	key, err := s.keyFunc(mdefObj)

	if err != nil {
		s.log.Error(err, "cannot create a key")
		return err
	}

	logger := s.log.WithValues("func", "deletefromindex",
		"name", mdefObj.GetName(),
		"namespace", mdefObj.GetNamespace(),
		"key", key)

	fullObj, found, err := s.indexStore.Get(mdefObj)
	if err != nil {
		s.log.Error(err, "cannot get")
		return err
	}

	if found {
		logger.V(2).Info("deleting obj")

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
	return s.delta.Add(obj)
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

	mdefObj, err := newStoreObject(obj)

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
// Resync walks the entire cache to refresh the Object matches to MeterDefinitions
// Updates and Delta deletes if the no MeterDefinitions match
func (s *MeterDefinitionStore) Resync() error {
	s.Lock()
	defer s.Unlock()
	objs := s.indexStore.List()
	for _, obj := range objs {
		if mdeo, ok := obj.(*pkgtypes.MeterDefinitionEnhancedObject); ok {
			addObj, err := s.addObj(mdeo.Object)
			if err != nil {
				return err
			} else if addObj != nil {
				// Emit a delta delete if the object no longer matches a meterdefinition
				if len(addObj.MeterDefinitions) != 0 {
					if err := s.delta.Add(addObj); err != nil {
						s.log.Error(err, "failed to update/add to delta store")
						return err
					}
				} else {
					if err := s.delta.Delete(addObj); err != nil {
						s.log.Error(err, "failed to update/delete to delta store")
						return err
					}
				}

				if err := s.indexStore.Add(addObj); err != nil {
					s.log.Error(err, "failed to resync/add to index store")
					return err
				}
			}
		}
	}
	return nil
}

// For a MeterDefinition event, Delta Add or Delete Objects if they still match
func (s *MeterDefinitionStore) SyncByIndex(indexName, indexedValue string) error {

	s.Lock()
	defer s.Unlock()

	objs, err := s.indexStore.ByIndex(indexName, indexedValue)
	if err != nil {
		return err
	}

	for _, obj := range objs {
		if mdeo, ok := obj.(*pkgtypes.MeterDefinitionEnhancedObject); ok {
			addObj, err := s.addObj(mdeo.Object)
			if err != nil {
				return err
			}
			// Emit a delta delete for processing if the object no longer matches a meterdefinition
			if len(addObj.MeterDefinitions) != 0 {
				if err := s.delta.Add(addObj); err != nil {
					s.log.Error(err, "failed to update/add to delta store")
					return err
				}
			} else {
				if err := s.delta.Delete(addObj); err != nil {
					s.log.Error(err, "failed to update/delete to delta store")
					return err
				}
			}

			if err := s.indexStore.Add(addObj); err != nil {
				s.log.Error(err, "failed to update/add to index store")
				return err
			}
		}
	}
	return nil
}

func newStoreObject(obj interface{}) (*pkgtypes.MeterDefinitionEnhancedObject, error) {
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
	dictionary *MeterDefinitionDictionary,
	scheme *runtime.Scheme,
) *MeterDefinitionStore {
	keyFunc := EnhancedObjectKeyFunc(scheme)

	storeIndexers := cache.Indexers{
		IndexMeterDefinition: MeterDefinitionIndexFunc,
		IndexNamespace:       cache.MetaNamespaceIndexFunc,
	}

	store := cache.NewIndexer(keyFunc, storeIndexers)
	delta := cache.NewDeltaFIFOWithOptions(cache.DeltaFIFOOptions{KeyFunction: keyFunc, KnownObjects: store})

	return &MeterDefinitionStore{
		ctx:        ctx,
		log:        log.WithName("obj_store").V(4),
		scheme:     scheme,
		dictionary: dictionary,
		delta:      delta,
		indexStore: store,
		keyFunc:    keyFunc,
	}
}
