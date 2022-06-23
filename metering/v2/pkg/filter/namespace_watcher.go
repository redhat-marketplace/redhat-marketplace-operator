// Copyright 2022 IBM Corp.
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

package filter

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NamespaceWatcher struct {
	namespaces map[client.ObjectKey]map[string][]reflect.Type
	watches    []chan interface{}
	log        logr.Logger

	mu           sync.RWMutex
	watchesMutex sync.Mutex
}

func ProvideNamespaceWatcher(
	log logr.Logger,
) *NamespaceWatcher {
	return &NamespaceWatcher{
		namespaces: map[client.ObjectKey]map[string][]reflect.Type{},
		watches:    make([]chan interface{}, 0),
		log:        log.WithName("namespace-watcher"),
	}
}

func (n *NamespaceWatcher) keyFunc(ns string, t reflect.Type) string {
	return fmt.Sprintf("%s-%s", ns, t.String())
}

func (n *NamespaceWatcher) alert() {
	n.watchesMutex.Lock()
	defer n.watchesMutex.Unlock()

	for _, c := range n.watches {
		c <- true
	}
}

func (n *NamespaceWatcher) RegisterWatch(in chan interface{}) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.watches = append(n.watches, in)
	return nil
}

func (n *NamespaceWatcher) addNamespace(in client.ObjectKey, nsTypes map[string][]reflect.Type) (alert bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	before := n.listNamespaces()

	stored, ok := n.namespaces[in]
	if !ok {
		n.namespaces[in] = nsTypes
		after := n.listNamespaces()

		if !reflect.DeepEqual(before, after) {
			alert = true
		}

		return
	}

	if !reflect.DeepEqual(stored, nsTypes) {
		n.namespaces[in] = mergeMap(stored, nsTypes)
		after := n.listNamespaces()

		if !reflect.DeepEqual(before, after) {
			alert = true
		}
		return
	}

	return
}

func (n *NamespaceWatcher) AddNamespace(in client.ObjectKey, nsTypes map[string][]reflect.Type) {
	alert := n.addNamespace(in, nsTypes)
	n.log.Info("adding namespaces", "namespaces", fmt.Sprintf("%+v", n.listNamespaces()), "alert", alert)
	if alert {
		n.alert()
	}
}

func (n *NamespaceWatcher) removeNamespace(in client.ObjectKey) (alert bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	before := n.listNamespaces()
	delete(n.namespaces, in)
	after := n.listNamespaces()

	if !alert && reflect.DeepEqual(before, after) {
		alert = true
	}

	return
}

func (n *NamespaceWatcher) RemoveNamespace(in client.ObjectKey) {
	alert := n.removeNamespace(in)
	if alert {
		n.alert()
	}
}

func (n *NamespaceWatcher) listNamespaces() map[string][]reflect.Type {
	nses := map[string][]reflect.Type{}

	for _, nsToType := range n.namespaces {
		nses = mergeMap(nses, nsToType)
	}

	if filterTypes, hasAll := nses[corev1.NamespaceAll]; hasAll {
		filteredNSes := map[string][]reflect.Type{}
		for ns, filters := range nses {
			if ns == corev1.NamespaceAll {
				filteredNSes[ns] = filters
				continue
			}
			result := filterTypeSlice(filters, filterTypes)
			if len(result) != 0 {
				filteredNSes[ns] = result
			}
		}

		return filteredNSes
	}

	return nses
}

func (n *NamespaceWatcher) Get() map[string][]reflect.Type {
	n.mu.RLock()
	defer n.mu.RUnlock()

	resp := n.listNamespaces()
	n.log.Info("list namespaces", "namespaces", fmt.Sprintf("%v", resp))
	return resp
}

func mergeMap(a, b map[string][]reflect.Type) map[string][]reflect.Type {
	resultMap := map[string][]reflect.Type{}

	for name := range a {
		resultMap[name] = a[name]
	}

	for name := range b {
		v, ok := resultMap[name]

		if ok {
			resultMap[name] = mergeTypeSlice(v, b[name])
			continue
		}

		resultMap[name] = b[name]
	}

	return resultMap
}

func mergeTypeSlice(a, b []reflect.Type) []reflect.Type {
	typeMap := map[reflect.Type]interface{}{}
	for _, typ := range a {
		typeMap[typ] = nil
	}
	for _, typ := range b {
		typeMap[typ] = nil
	}

	merged := []reflect.Type{}
	for key := range typeMap {
		merged = append(merged, key)
	}
	return merged
}

func filterTypeSlice(a, filters []reflect.Type) []reflect.Type {
	typeMap := map[reflect.Type]interface{}{}
	for _, typ := range a {
		found := false
		for _, filter := range filters {
			if typ == filter {
				found = true
				break
			}
		}

		if found {
			continue
		}
		typeMap[typ] = nil
	}

	filtered := []reflect.Type{}
	for key := range typeMap {
		filtered = append(filtered, key)
	}
	return filtered

}
