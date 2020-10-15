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
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/go-logr/logr"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	"k8s.io/apimachinery/pkg/types"
)

// StatusProcessor will update the meter definition
// status with the objects that matched it.
type StatusProcessor struct {
	log   logr.Logger
	cc    ClientCommandRunner
	mutex sync.Mutex
	locks map[types.NamespacedName]sync.Mutex
}

// NewStatusProcessor is the provider that creates
// the processor.
func NewStatusProcessor(
	log logr.Logger,
	cc ClientCommandRunner,
) *StatusProcessor {
	return &StatusProcessor{
		log:   log,
		cc:    cc,
		locks: make(map[types.NamespacedName]sync.Mutex),
	}
}

// Start will register it's listener and execute the function.
func (u *StatusProcessor) New(store *MeterDefinitionStore) Processor {
	return NewProcessor("statusProcessor", u.log, u.cc, store, u)
}

// Process will receive a new ObjectResourceMessage and find and update the metere
// definition associated with the object. To prevent gaps, it bulk retrieves the
// resources and checks it against the status.
func (u *StatusProcessor) Process(ctx context.Context, inObj *ObjectResourceMessage) error {
	log := u.log.WithValues("process", "statusProcessor")
	mdef := &marketplacev1alpha1.MeterDefinition{}

	if inObj == nil {
		return nil
	}

	if inObj.ObjectResourceValue == nil {
		return nil
	}

	u.mutex.Lock()
	lock, exists := u.locks[inObj.MeterDef]

	if !exists {
		u.locks[inObj.MeterDef] = sync.Mutex{}
		lock, _ = u.locks[inObj.MeterDef]
	}
	u.mutex.Unlock()

	lock.Lock()
	defer lock.Unlock()

	log.Info("incoming obj", "type", fmt.Sprintf("%T", inObj.Object), "mdef", inObj.MeterDef)

	result, _ := u.cc.Do(ctx,
		HandleResult(
			GetAction(inObj.MeterDef, mdef),
			OnContinue(Call(func() (ClientAction, error) {
				log.Info("found objs", "mdef", inObj.MeterDef)

				resources := []marketplacev1alpha1.WorkloadResource{}

				set := map[types.UID]marketplacev1alpha1.WorkloadResource{}

				for _, obj := range mdef.Status.WorkloadResources {
					set[obj.UID] = obj
				}

				switch inObj.Action {
				case DeleteMessageAction:
					delete(set, inObj.ObjectResourceValue.UID)
				case AddMessageAction:
					set[inObj.ObjectResourceValue.UID] = *inObj.ObjectResourceValue.WorkloadResource
				}

				for _, obj := range set {
					resources = append(resources, obj)
				}

				sort.Sort(marketplacev1alpha1.ByAlphabetical(resources))
				mdef.Status.WorkloadResources = resources

				log.Info("updating meter def", "mdef", inObj.MeterDef, "uid", mdef.UID, "len", len(mdef.Status.WorkloadResources))
				return UpdateAction(mdef, UpdateStatusOnly(true)), nil
			})),
		),
	)

	if result.Is(NotFound) {
		return nil
	}

	if result.Is(Error) {
		log.Error(result, "failed run")
		return result
	}

	return nil
}
