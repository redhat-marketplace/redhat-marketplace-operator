/*
Copyright 2020 IBM Co..

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MeterDefinitionStore
// +genclient
type MeterdefinitionStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MeterdefinitionStoreSpec   `json:"spec,omitempty"`
}

// MeterdefinitionStoreList contains a list of MeterdefinitionStores
type MeterdefinitionStoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MeterdefinitionStore `json:"items"`
}

type MeterdefinitionStoreSpec struct {
	
	CSVEntrys []CSVEntry `json:"csvEntrys"`
	
}

type CSVEntry struct {
	// PackageName defines the package name
	PackageName string `json:"packageName"`
	
	// Entry defines an entry to the meterdefinitionstore
	Entry EntryMapping `json:"entry,omitempty"`
}

type EntryMapping map[string]EntryKey

type EntryKey struct {
	VersionRange string `json:"version"`
	MeterDefinitions []MeterDefinitionSpec `json:"meterdefinitions"`
}
