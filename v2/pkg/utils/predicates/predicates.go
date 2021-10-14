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

package predicates

import (
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func NamespacePredicate(targetNamespace string) predicate.Funcs {
	return predicate.Funcs{
		// Ensures MarketPlaceConfig reconciles only within namespace
		GenericFunc: func(e event.GenericEvent) bool {
			return e.Meta.GetNamespace() == targetNamespace
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.MetaOld.GetNamespace() == targetNamespace && e.MetaNew.GetNamespace() == targetNamespace
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Meta.GetNamespace() == targetNamespace
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Meta.GetNamespace() == targetNamespace
		},
	}
}

type SyncedMapHandler struct {
	exists func(types.NamespacedName) bool
	data   map[types.NamespacedName]types.NamespacedName
	mutex  sync.Mutex
}

func NewSyncedMapHandler(exists func(types.NamespacedName) bool) *SyncedMapHandler {
	return &SyncedMapHandler{exists: exists, data: map[types.NamespacedName]types.NamespacedName{}}
}

var _ handler.Mapper = &SyncedMapHandler{}

func (s *SyncedMapHandler) Start(done <-chan struct{}) error {
	exit := make(chan struct{})
	ticker := time.NewTicker(60 * time.Second)

	go func() {
		defer close(exit)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.Cleanup()
			case <-done:
				return
			}
		}
	}()

	<-exit
	return nil
}

func (s *SyncedMapHandler) AddOrUpdate(from types.NamespacedName, to types.NamespacedName) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.data[from] = to
}

func (s *SyncedMapHandler) Cleanup() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	deleteKeys := []types.NamespacedName{}

	for key, value := range s.data {
		if !s.exists(key) || !s.exists(value) {
			deleteKeys = append(deleteKeys, key)
		}
	}

	for _, key := range deleteKeys {
		delete(s.data, key)
	}
}

func (s *SyncedMapHandler) Map(in handler.MapObject) []reconcile.Request {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if in.Meta == nil {
		return nil
	}

	name := types.NamespacedName{Name: in.Meta.GetName(), Namespace: in.Meta.GetNamespace()}

	if v, ok := s.data[name]; ok {
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      v.Name,
					Namespace: v.Namespace,
				},
			}}
	}

	return nil
}
