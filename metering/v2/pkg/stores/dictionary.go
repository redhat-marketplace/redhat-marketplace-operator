// Copyright 2021 IBM Corp.
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
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ cache.Store = &MeterDefinitionDictionary{}
var _ metav1.Object = &MeterDefinitionExtended{}

type MeterDefinitionExtended struct {
	*v1beta1.MeterDefinition
	Filter *filter.MeterDefinitionLookupFilter
}

var _ metav1.Object = &MeterDefinitionExtended{}

// MeterDefinitionDictionary tracks the meterdefinitions
// current on the system and provides methods of checking
// if a resources is apart of the meterdefinition.
type MeterDefinitionDictionary struct {
	cache   cache.Store
	keyFunc cache.KeyFunc
	delta   *cache.DeltaFIFO

	log     logr.Logger
	factory *filter.MeterDefinitionLookupFilterFactory

	sync.RWMutex
}

func NewMeterDefinitionDictionary(
	ctx context.Context,
	namespaces pkgtypes.Namespaces,
	log logr.Logger,
	factory *filter.MeterDefinitionLookupFilterFactory,
) *MeterDefinitionDictionary {
	keyFunc := cache.MetaNamespaceKeyFunc
	store := cache.NewStore(keyFunc)

	return &MeterDefinitionDictionary{
		log:     log.WithName("mdef_dictionary"),
		keyFunc: keyFunc,
		cache:   store,
		delta:   cache.NewDeltaFIFOWithOptions(cache.DeltaFIFOOptions{KeyFunction: keyFunc, KnownObjects: store}),
		factory: factory,
	}
}

func (def *MeterDefinitionDictionary) FindObjectMatches(
	obj interface{},
	results *[]filter.Result,
) error {
	filters, err := def.ListFilters()

	if err != nil {
		def.log.Error(err, "error listing filters")
		return err
	}

	for i := range filters {
		localLookup := filters[i]

		lookupOk, err := localLookup.Matches(obj)

		if err != nil {
			if errors.Is(err, rhmclient.AccessDeniedErr) {
				def.log.Info("skipping object for resource filter", "error", err.Error())
				continue
			}

			if !errors.Is(err, rhmclient.AccessDeniedErr) {
				def.log.Error(err, "error finding matches for resource filter")
				return err
			}

		}

		if lookupOk {
			*results = append(*results, filter.Result{
				Ok:     lookupOk,
				Lookup: localLookup,
			})
		}
	}

	return nil
}

func (def *MeterDefinitionDictionary) ListFilters() ([]*filter.MeterDefinitionLookupFilter, error) {
	filters := []*filter.MeterDefinitionLookupFilter{}

	objs := def.List()

	for _, obj := range objs {
		meterdef, ok := obj.(*MeterDefinitionExtended)

		if !ok {
			return nil, errors.New("encountered unexpected type")
		}

		filters = append(filters, meterdef.Filter)
	}

	return filters, nil
}

// Add adds the given object to the accumulator associated with the given object's key
func (def *MeterDefinitionDictionary) Add(obj interface{}) error {
	addObj, err := def.newMeterDefinitionExtended(obj)

	if err != nil {
		def.log.Error(err, "error extending obj")
		return err
	}

	def.log.Info("recording obj", "filters", fmt.Sprintf("%+v", addObj.Filter), "obj", fmt.Sprintf("%+v", obj))

	def.Lock()
	defer def.Unlock()

	if err := def.delta.Add(addObj); err != nil {
		def.log.Error(err, "error adding obj to delta")
		return err
	}

	if err := def.cache.Add(addObj); err != nil {
		def.log.Error(err, "error adding obj to cache")
		return err
	}

	return nil
}

// Update updates the given object in the accumulator associated with the given object's key
func (def *MeterDefinitionDictionary) Update(obj interface{}) error {
	addObj, err := def.newMeterDefinitionExtended(obj)

	if err != nil {
		def.log.Error(err, "error extending obj")
		return err
	}

	// Skip Updates where Generation does not change
	item, exists, err := def.Get(addObj)
	if exists && err == nil {
		prevObj, ok := item.(*MeterDefinitionExtended)
		if ok {
			if addObj.ObjectMeta.Generation == prevObj.ObjectMeta.Generation {
				return nil
			}
		}
	}

	def.log.Info("updating obj", "obj", fmt.Sprintf("%+v", obj))

	def.Lock()
	defer def.Unlock()

	if err := def.delta.Update(addObj); err != nil {
		def.log.Error(err, "error adding obj to delta")
		return err
	}

	if err := def.cache.Update(addObj); err != nil {
		def.log.Error(err, "error adding obj to cache")
		return err
	}

	return nil
}

// Delete deletes the given object from the accumulator associated with the given object's key
func (def *MeterDefinitionDictionary) Delete(obj interface{}) error {
	def.Lock()
	defer def.Unlock()

	addObj, err := def.newMeterDefinitionExtended(obj)

	if err != nil {
		def.log.Error(err, "error extending obj")
		return err
	}

	if err := def.delta.Delete(addObj); err != nil {
		return err
	}

	return def.cache.Delete(addObj)
}

// List returns a list of all the currently non-empty accumulators
func (def *MeterDefinitionDictionary) List() []interface{} {
	return def.cache.List()
}

// ListKeys returns a list of all the keys currently associated with non-empty accumulators
func (def *MeterDefinitionDictionary) ListKeys() []string {
	return def.cache.ListKeys()
}

// Get returns the accumulator associated with the given object's key
func (def *MeterDefinitionDictionary) Get(obj interface{}) (item interface{}, exists bool, err error) {
	def.RLock()
	defer def.RUnlock()

	return def.cache.Get(obj)
}

// GetByKey returns the accumulator associated with the given key
func (def *MeterDefinitionDictionary) GetByKey(key string) (item interface{}, exists bool, err error) {
	def.RLock()
	defer def.RUnlock()

	return def.cache.GetByKey(key)
}

func (def *MeterDefinitionDictionary) AddIfNotPresent(obj interface{}) error {
	return def.delta.Add(obj)
}

func (def *MeterDefinitionDictionary) Close() {
	def.delta.Close()
}

func (def *MeterDefinitionDictionary) HasSynced() bool {
	return def.delta.HasSynced()
}

func (def *MeterDefinitionDictionary) Pop(process cache.PopProcessFunc) (interface{}, error) {
	return def.delta.Pop(process)
}

// Replace will delete the contents of the store, using instead the
// given list. Store takes ownership of the list, you should not reference
// it after calling this function.
func (def *MeterDefinitionDictionary) Replace(in []interface{}, str string) error {
	def.Lock()
	defer def.Unlock()

	objs := []interface{}{}

	for _, obj := range in {
		localObj := obj
		addObj, err := def.newMeterDefinitionExtended(localObj)
		if err != nil {
			return err
		}

		objs = append(objs, addObj)
	}

	if err := def.cache.Replace(objs, str); err != nil {
		return err
	}

	if err := def.delta.Replace(objs, str); err != nil {
		return err
	}

	return nil
}

// Resync is meaningless in the terms appearing here but has
// meaning in some implementations that have non-trivial
// additional behavior (e.g., DeltaFIFO).
func (def *MeterDefinitionDictionary) Resync() error {
	return nil
}

func (def *MeterDefinitionDictionary) newMeterDefinitionExtended(obj interface{}) (*MeterDefinitionExtended, error) {
	meterdef, ok := obj.(*v1beta1.MeterDefinition)

	if !ok {
		return nil, errors.New("expected meter definition")
	}

	lookup, err := def.factory.New(meterdef)

	if err != nil {
		return nil, err
	}

	return &MeterDefinitionExtended{
		MeterDefinition: meterdef,
		Filter:          lookup,
	}, nil
}

func ProvideMeterDefinitionList(
	cacheIsStarted managers.CacheIsStarted,
	client runtimeClient.Client,
) (*v1beta1.MeterDefinitionList, error) {
	obj := v1beta1.MeterDefinitionList{}
	err := client.List(context.TODO(), &obj, &runtimeClient.ListOptions{
		Namespace: "",
	})
	return &obj, err
}
