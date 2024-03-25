// Copyright 2023 IBM Corp.
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

package events

import (
	"sync"
)

// EventAccumulator collects events associated with a User and ManifestType
// [User][ManifestType]EventJsons
type EventAccumulator struct {
	eventMap map[UserManifestType]EventJsons
	mu       sync.Mutex
}

// Add an Event for a [User][ManifestType]
func (e *EventAccumulator) Add(event Event) int {
	e.mu.Lock()
	userManifestType := UserManifestType{User: event.User, ManifestType: event.ManifestType}
	eventJsons := e.eventMap[userManifestType]
	aEventJsons := append(eventJsons, event.RawMessage)
	e.eventMap[userManifestType] = aEventJsons
	length := len(aEventJsons)
	e.mu.Unlock()
	return length
}

// Flush all Events for a [User][ManifestType]
func (e *EventAccumulator) Flush(userManifestType UserManifestType) EventJsons {
	e.mu.Lock()
	flushedEvents := e.eventMap[userManifestType]
	delete(e.eventMap, userManifestType)
	e.mu.Unlock()
	return flushedEvents
}

// Flush EventMap and reset to clear memory accumulation
func (e *EventAccumulator) FlushAll() map[UserManifestType]EventJsons {
	e.mu.Lock()
	flushedEventMap := e.eventMap
	e.eventMap = make(map[UserManifestType]EventJsons)
	e.mu.Unlock()
	return flushedEventMap
}

// Are the events empty for a [User][ManifestType]
func (e *EventAccumulator) IsEmpty(userManifestType UserManifestType) bool {
	e.mu.Lock()
	length := len(e.eventMap[userManifestType])
	e.mu.Unlock()
	return length == 0
}

// Get the userManifestType keys
func (e *EventAccumulator) GetKeys() []UserManifestType {
	e.mu.Lock()
	i := 0
	keys := make([]UserManifestType, len(e.eventMap))
	for k := range e.eventMap {
		keys[i] = k
		i++
	}
	e.mu.Unlock()
	return keys
}
