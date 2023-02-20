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

import "sync"

// EventAccumulator collects events associated with a key
type EventAccumulator struct {
	eventMap map[Key]EventJsons
	mu       sync.Mutex
}

// Add an Event for a Key
func (e *EventAccumulator) Add(event Event) int {
	e.mu.Lock()
	eventJsons := e.eventMap[event.Key]
	aEventJsons := append(eventJsons, event.EventJson)
	e.eventMap[event.Key] = aEventJsons
	length := len(aEventJsons)
	e.mu.Unlock()
	return length
}

// Flush all Events for a Key
func (e *EventAccumulator) Flush(key Key) EventJsons {
	e.mu.Lock()
	flushedEvents := e.eventMap[key]
	e.eventMap[key] = nil
	e.mu.Unlock()
	return flushedEvents
}

func (e *EventAccumulator) IsEmpty(key Key) bool {
	e.mu.Lock()
	length := len(e.eventMap[key])
	e.mu.Unlock()
	return length == 0
}
