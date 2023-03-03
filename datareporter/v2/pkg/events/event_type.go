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
	"encoding/json"
	"reflect"
	"sync"
)

// config needed to provideDataServiceConfig
type Config struct {
	OutputDirectory      string
	DataServiceTokenFile string
	DataServiceCertFile  string
	Namespace            string
	ApiKeys              ApiKeys
}

type ApiKey struct {
	Key      Key // Decoded X-API-KEY
	Metadata Metadata
}

type ApiKeys struct {
	apiKeys []ApiKey
	mu      sync.RWMutex
}

func (a *ApiKeys) HasKey(key Key) bool {
	a.mu.RLock()
	for _, apiKey := range a.apiKeys {
		if apiKey.Key == key {
			return true
		}
	}
	a.mu.RUnlock()
	return false
}

func (a *ApiKeys) SetApiKeys(apiKeys []ApiKey) {
	a.mu.Lock()

	if !reflect.DeepEqual(apiKeys, a.apiKeys) {
		a.apiKeys = apiKeys
	}

	a.mu.Unlock()
}

func (a *ApiKeys) GetMetadata(key Key) Metadata {
	metadata := make(Metadata)
	a.mu.RLock()
	for _, apiKey := range a.apiKeys {
		if apiKey.Key == key {
			metadata = apiKey.Metadata
		}
	}
	a.mu.RUnlock()
	return metadata
}

type Key string // the api key the event was sent with

type Event struct {
	Key
	json.RawMessage
}

type Events []Event

type EventJsons []json.RawMessage

type Metadata map[string]string

type Manifest struct {
	Type     string `json:"type,omitempty"`
	Metadata `json:"metadata,omitempty"`
}

type ReportData struct {
	Metadata   Metadata          `json:"metadata,omitempty"`
	EventJsons []json.RawMessage `json:"data"`
}
