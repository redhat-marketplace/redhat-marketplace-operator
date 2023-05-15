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

// EventAccumulator collects events associated with a key
type EventAccumulator struct {
	eventMap map[string]EventJsons
	mu       sync.Mutex
}

// Add an Event for a Key
func (e *EventAccumulator) Add(event Event) int {
	e.mu.Lock()
	eventJsons := e.eventMap[event.User]
	aEventJsons := append(eventJsons, event.RawMessage)
	e.eventMap[event.User] = aEventJsons
	length := len(aEventJsons)
	e.mu.Unlock()
	return length
}

// Flush all Events for a Key
func (e *EventAccumulator) Flush(user string) EventJsons {
	e.mu.Lock()
	flushedEvents := e.eventMap[user]
	delete(e.eventMap, user)
	e.mu.Unlock()
	return flushedEvents
}

// Flush EventMap and reset to clear memory accumulation
func (e *EventAccumulator) FlushAll() map[string]EventJsons {
	e.mu.Lock()
	flushedEventMap := e.eventMap
	e.eventMap = make(map[string]EventJsons)
	e.mu.Unlock()
	return flushedEventMap
}

func (e *EventAccumulator) IsEmpty(user string) bool {
	e.mu.Lock()
	length := len(e.eventMap[user])
	e.mu.Unlock()
	return length == 0
}

func (e *EventAccumulator) GetKeys() []string {
	e.mu.Lock()
	i := 0
	keys := make([]string, len(e.eventMap))
	for k := range e.eventMap {
		keys[i] = k
		i++
	}
	e.mu.Unlock()
	return keys
}
