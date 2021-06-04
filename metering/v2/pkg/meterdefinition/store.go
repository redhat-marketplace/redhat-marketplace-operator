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

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	monitoringv1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/dictionary"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/filter"
	pkgtypes "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	marketplacev1beta1client "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/typed/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
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
	dictionary *dictionary.MeterDefinitionDictionary
	indexStore cache.Indexer
	delta      *cache.DeltaFIFO

	ctx    context.Context
	log    logr.Logger
	scheme *runtime.Scheme

	// kubeClient to query kube
	kubeClient clientset.Interface

	findOwner                *rhmclient.FindOwnerHelper
	monitoringClient         *monitoringv1client.MonitoringV1Client
	marketplaceClientV1beta1 *marketplacev1beta1client.MarketplaceV1beta1Client
}

var _ cache.Queue = &MeterDefinitionStore{}

func enhancedObjectKeyFunc(obj interface{}) (string, error) {
	v, ok := obj.(*pkgtypes.MeterDefinitionEnhancedObject)
	if !ok {
		return "", errors.New("failed to create key")
	}

	meta, err := meta.Accessor(v.Object)
	if err != nil {
		return "", fmt.Errorf("object has no meta: %v", err)
	}
	if len(meta.GetNamespace()) > 0 {
		return meta.GetNamespace() + "/" + meta.GetName(), nil
	}
	return meta.GetName(), nil
}

func makeStoreIndexers(keyFunc cache.KeyFunc) cache.Indexers {
	return cache.Indexers{
		"meterDefinition": func(obj interface{}) ([]string, error) {
			v, ok := obj.(*pkgtypes.MeterDefinitionEnhancedObject)
			if !ok {
				return nil, errors.New("failed to index obj")
			}

			keys := make([]string, 0, len(v.MeterDefinitions))
			for i := range v.MeterDefinitions {
				meterDef := v.MeterDefinitions[i]
				key, err := keyFunc(meterDef)
				if !ok {
					return nil, errors.Wrap(err, "failed to get obj key")
				}
				keys = append(keys, key)
			}

			return keys, nil
		},
	}
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
	logger := s.log.WithValues("func", "add", "namespace/name", key)
	logger.Info("adding obj")

	// look over all meterDefinitions, matching workloads are saved
	results := []filter.Result{}

	err := s.dictionary.FindObjectMatches(obj, &results, false)
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

	meterDefs := []*v1beta1.MeterDefinition{}

	for _, result := range results {
		if !result.Ok {
			logger.V(4).Info("no match", "obj", obj)
			continue
		}

		mdef := result.Lookup.MeterDefinition
		logger.Info("result", "name", mdef.GetName())
		meterDefs = append(meterDefs, mdef)
	}

	if len(meterDefs) == 0 {
		logger.Info("no matched meterdefs returned")
		return nil
	}

	logger.Info("return meterdefs results", "len", len(meterDefs))

	mdefObj := &pkgtypes.MeterDefinitionEnhancedObject{
		Object:           runtimeObj,
		MeterDefinitions: meterDefs,
	}
	err = s.indexStore.Add(mdefObj)

	if err != nil {
		logger.Error(err, "failed to add to index store")
		return err
	}

	if err := s.delta.Add(mdefObj); err != nil {
		logger.Error(err, "failed to add to delta store")
		return err
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
	key, err := cache.MetaNamespaceKeyFunc(obj)

	if err != nil {
		return err
	}

	item, exists, err := s.indexStore.GetByKey(key)

	if err != nil {
		return err
	}

	if exists {
		if err := s.indexStore.Delete(item); err != nil {
			return err
		}

		if err := s.delta.Delete(item); err != nil {
			return err
		}
	}

	return nil
}

// List implements the List method of the store interface.
func (s *MeterDefinitionStore) List() []interface{} {
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
	return s.indexStore.ByIndex(indexName, indexedValue)
}

// ListKeys implements the ListKeys method of the store interface.
func (s *MeterDefinitionStore) ListKeys() []string {
	return s.indexStore.ListKeys()
}

// Get implements the Get method of the store interface.
func (s *MeterDefinitionStore) Get(obj interface{}) (item interface{}, exists bool, err error) {
	runtimeObj, ok := obj.(runtime.Object)

	if !ok {
		err := errors.New("obj not a runtime.Object")
		s.log.Error(err, "obj not a runtime object")
		return nil, false, err
	}

	return s.indexStore.Get(&pkgtypes.MeterDefinitionEnhancedObject{
		Object: runtimeObj,
	})
}

// GetByKey implements the GetByKey method of the store interface.
func (s *MeterDefinitionStore) GetByKey(key string) (item interface{}, exists bool, err error) {
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
	return nil
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
	keyFunc := enhancedObjectKeyFunc
	store := cache.NewIndexer(keyFunc, makeStoreIndexers(keyFunc))
	delta := cache.NewDeltaFIFO(keyFunc, nil)

	return &MeterDefinitionStore{
		ctx:                      ctx,
		log:                      log.WithName("obj_store").V(0),
		scheme:                   scheme,
		kubeClient:               kubeClient,
		monitoringClient:         monitoringClient,
		marketplaceClientV1beta1: marketplaceclientV1beta1,
		dictionary:               dictionary,
		findOwner:                findOwner,
		delta:                    delta,
		indexStore:               store,
	}
}
