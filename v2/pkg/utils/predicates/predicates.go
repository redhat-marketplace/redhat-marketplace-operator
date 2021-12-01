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
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func NamespacePredicate(targetNamespace string) predicate.Funcs {
	return predicate.Funcs{
		// Ensures MarketPlaceConfig reconciles only within namespace
		GenericFunc: func(e event.GenericEvent) bool {
			return e.Object.GetNamespace() == targetNamespace
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectOld.GetNamespace() == targetNamespace && e.ObjectNew.GetNamespace() == targetNamespace
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetNamespace() == targetNamespace
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Object.GetNamespace() == targetNamespace
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

func (s *SyncedMapHandler) Start(ctx context.Context) error {
	exit := make(chan struct{})
	ticker := time.NewTicker(60 * time.Second)

	go func() {
		defer close(exit)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.Cleanup()
			case <-ctx.Done():
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

	for key := range s.data {
		if !s.exists(key) {
			deleteKeys = append(deleteKeys, key)
		}
	}

	for _, key := range deleteKeys {
		delete(s.data, key)
	}
}

func (s *SyncedMapHandler) Map(a client.Object) []reconcile.Request {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if a == nil {
		return []reconcile.Request{}
	}

	name := types.NamespacedName{Name: a.GetName(), Namespace: a.GetNamespace()}

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
