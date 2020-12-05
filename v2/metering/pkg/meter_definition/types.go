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

package meter_definition

import (
	"encoding/json"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type ObjectUID types.UID
type MeterDefUID types.UID
type ResourceSet map[MeterDefUID]*v1alpha1.WorkloadResource
type ObjectResourceMessageAction string

const (
	AddMessageAction    ObjectResourceMessageAction = "Add"
	DeleteMessageAction                             = "Delete"
)

type ObjectResourceMessage struct {
	Action               ObjectResourceMessageAction `json:"action"`
	Object               interface{}                 `json:"object"`
	*ObjectResourceValue `json:"resourceValue,omitempty"`
}

func (o *ObjectResourceMessage) String() string {
	jsonOut, _ := json.Marshal(o)
	return string(jsonOut)
}

type ObjectResourceKey struct {
	ObjectUID
	MeterDefUID
}

func (o *ObjectResourceKey) String() string {
	jsonOut, _ := json.Marshal(o)
	return string(jsonOut)
}

func NewObjectResourceKey(object metav1.Object, meterdefUID MeterDefUID) ObjectResourceKey {
	return ObjectResourceKey{
		ObjectUID:   ObjectUID(object.GetUID()),
		MeterDefUID: meterdefUID,
	}
}

type ObjectResourceValue struct {
	MeterDef     types.NamespacedName
	MeterDefHash string
	Generation   int64
	Matched      bool
	Object       interface{}
	*v1alpha1.WorkloadResource
}

func NewObjectResourceValue(
	lookup *MeterDefinitionLookupFilter,
	resource *v1alpha1.WorkloadResource,
	obj interface{},
	matched bool,
) (*ObjectResourceValue, error) {
	o, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	return &ObjectResourceValue{
		MeterDef:         lookup.MeterDefName,
		MeterDefHash:     lookup.Hash(),
		WorkloadResource: resource,
		Generation:       o.GetGeneration(),
		Object:           obj,
		Matched:          matched,
	}, nil
}
