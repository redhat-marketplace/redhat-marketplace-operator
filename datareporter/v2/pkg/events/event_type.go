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
	"net/url"
	"reflect"
	"sync"

	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// config needed to provideDataServiceConfig
type Config struct {
	OutputDirectory      string
	DataServiceTokenFile string
	DataServiceCertFile  string
	DataServiceURL       *url.URL
	Namespace            string
	UserConfigs          UserConfigs
	AccMemoryLimit       resource.Quantity
	MaxFlushTimeout      metav1.Duration
	MaxEventEntries      int
	LicenseAccept        bool
	CipherSuites         []uint16
	MinVersion           uint16
}

type UserConfigs struct {
	userConfigs []v1alpha1.UserConfig
	mu          sync.RWMutex
}

func (a *UserConfigs) HasUser(userName string) bool {
	a.mu.RLock()
	for _, userConfig := range a.userConfigs {
		if userConfig.UserName == userName {
			return true
		}
	}
	a.mu.RUnlock()
	return false
}

func (a *UserConfigs) SetUserConfigs(userConfigs []v1alpha1.UserConfig) {
	a.mu.Lock()

	if !reflect.DeepEqual(userConfigs, a.userConfigs) {
		a.userConfigs = userConfigs
	}

	a.mu.Unlock()
}

func (a *UserConfigs) GetMetadata(userName string) Metadata {
	metadata := make(Metadata)
	a.mu.RLock()
	for _, userConfig := range a.userConfigs {
		if userConfig.UserName == userName {
			metadata = userConfig.Metadata
		}
	}
	a.mu.RUnlock()
	return metadata
}

type Event struct {
	User string // x-remote-user the Event was posted by
	json.RawMessage
	ManifestType string // dataReporter or override
}

type UserManifestType struct {
	User         string
	ManifestType string
}

type Events []Event

type EventJsons []json.RawMessage

type Metadata map[string]string

type Manifest struct {
	Type    string `json:"type,omitempty"`
	Version string `json:"version,omitempty"`
}

type ReportData struct {
	Metadata   Metadata          `json:"metadata,omitempty"`
	EventJsons []json.RawMessage `json:"data"`
}
