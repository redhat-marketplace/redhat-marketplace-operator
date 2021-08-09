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

package razee

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const IndexRazee = "razee"

type RazeeStores = map[string]*RazeeStore

type RazeeStore struct {
	delta   *cache.DeltaFIFO
	keyFunc cache.KeyFunc

	ctx    context.Context
	log    logr.Logger
	scheme *runtime.Scheme

	sync.RWMutex

	// kubeClient to query kube
	kubeClient clientset.Interface

	findOwner *rhmclient.FindOwnerHelper
}

// Implementing k8s.io/client-go/tools/cache.Store interface

func (s *RazeeStore) Add(obj interface{}) error {
	s.log.Info("dac debug Add an item to deltafifo")
	if err := s.delta.Add(obj); err != nil {
		s.log.Error(err, "failed to add to delta store")
		return err
	}
	return nil
}

func (s *RazeeStore) Update(obj interface{}) error {
	s.log.Info("dac debug Update an item to deltafifo")
	if err := s.delta.Update(obj); err != nil {
		s.log.Error(err, "failed to add to delta store")
		return err
	}
	return nil
}

func (s *RazeeStore) Delete(obj interface{}) error {
	s.log.Info("dac debug Delete an item to deltafifo")
	if err := s.delta.Delete(obj); err != nil {
		s.log.Error(err, "can't delete obj")
		return err
	}
	return nil
}

func (s *RazeeStore) List() []interface{} {
	s.log.Info("dac debug List an item to deltafifo")
	return s.delta.List()
}

func (s *RazeeStore) AddIfNotPresent(obj interface{}) error {
	s.log.Info("dac debug AddIfNotPresent an item to deltafifo")
	return s.delta.AddIfNotPresent(obj)
}

func (s *RazeeStore) Close() {
	s.log.Info("dac debug Close an item to deltafifo")
	s.delta.Close()
}

func (s *RazeeStore) HasSynced() bool {
	s.log.Info("dac debug HasSynced an item to deltafifo")
	return s.delta.HasSynced()
}

func (s *RazeeStore) Pop(process cache.PopProcessFunc) (interface{}, error) {
	s.log.Info("dac debug Pop an item to deltafifo")
	return s.delta.Pop(process)
}

func (s *RazeeStore) Get(obj interface{}) (item interface{}, exists bool, err error) {
	s.log.Info("dac debug Get an item to deltafifo")
	return s.delta.Get(obj)
}

func (s *RazeeStore) GetByKey(key string) (item interface{}, exists bool, err error) {
	s.log.Info("dac debug GetByKey an item to deltafifo")
	return s.delta.GetByKey(key)
}

func (s *RazeeStore) ListKeys() []string {
	s.log.Info("dac debug ListKeys an item to deltafifo")
	return s.delta.ListKeys()
}

func (s *RazeeStore) Replace(list []interface{}, resourceVersion string) error {
	s.log.Info("dac debug Replace an item to deltafifo")
	return s.delta.Replace(list, resourceVersion)
}

func (s *RazeeStore) Resync() error {
	s.log.Info("dac debug Resync deltafifo")
	return s.delta.Resync()
}

var _ cache.Store = &RazeeStore{}

func NewRazeeStore(
	ctx context.Context,
	log logr.Logger,
	kubeClient clientset.Interface,
	findOwner *rhmclient.FindOwnerHelper,
	scheme *runtime.Scheme,
) *RazeeStore {
	keyFunc := cache.MetaNamespaceKeyFunc

	/*
		storeIndexers := cache.Indexers{
			IndexRazee: cache.MetaNamespaceIndexFunc,
		}

	*/
	//store := cache.NewIndexer(keyFunc, storeIndexers)
	delta := cache.NewDeltaFIFO(keyFunc, nil)
	return &RazeeStore{
		ctx:        ctx,
		log:        log.WithName("razee_store").V(4),
		scheme:     scheme,
		kubeClient: kubeClient,
		findOwner:  findOwner,
		delta:      delta,
		keyFunc:    keyFunc,
	}
}
