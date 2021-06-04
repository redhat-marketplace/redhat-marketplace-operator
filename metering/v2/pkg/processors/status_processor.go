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

package processors

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/mailbox"
	pkgtypes "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StatusProcessor will update the meter definition
// status with the objects that matched it.
type StatusProcessor struct {
	*Processor
	log        logr.Logger
	kubeClient client.Client
	mutex      sync.Mutex
	locks      map[types.NamespacedName]sync.Mutex
	scheme     *runtime.Scheme
}

// NewStatusProcessor is the provider that creates
// the processor.
func ProvideStatusProcessor(
	log logr.Logger,
	kubeClient client.Client,
	mb *mailbox.Mailbox,
	scheme *runtime.Scheme,
) *StatusProcessor {
	sp := &StatusProcessor{
		Processor: &Processor{
			log:           log,
			digestersSize: 1,
			retryCount:    3,
			mailbox:       mb,
			channelName:   mailbox.ObjectChannel,
		},
		log:        log.WithValues("process", "statusProcessor"),
		kubeClient: kubeClient,
		locks:      make(map[types.NamespacedName]sync.Mutex),
		scheme:     scheme,
	}

	sp.Processor.DeltaProcessor = sp
	return sp
}

// Process will receive a new ObjectResourceMessage and find and update the metere
// definition associated with the object. To prevent gaps, it bulk retrieves the
// resources and checks it against the status.
func (u *StatusProcessor) Process(ctx context.Context, inObj cache.Delta) error {
	if inObj.Object == nil {
		return nil
	}

	enhancedObj, ok := inObj.Object.(pkgtypes.MeterDefinitionEnhancedObject)

	if !ok {
		return errors.New("obj is unexpected type")
	}

	for i := range enhancedObj.MeterDefinitions {
		mdef := enhancedObj.MeterDefinitions[i]
		key, _ := client.ObjectKeyFromObject(mdef)

		var (
			lock   sync.Mutex
			exists bool
		)

		u.mutex.Lock()
		if lock, exists = u.locks[key]; !exists {
			u.locks[key] = sync.Mutex{}
			lock, _ = u.locks[key]
		}
		u.mutex.Unlock()

		err := func() error {
			lock.Lock()
			defer lock.Unlock()

			resources := []common.WorkloadResource{}
			set := map[types.UID]common.WorkloadResource{}

			for _, obj := range mdef.Status.WorkloadResources {
				set[obj.UID] = obj
			}

			workload, err := common.NewWorkloadResource(inObj.Object, u.scheme)

			if err != nil {
				return err
			}

			switch inObj.Type {
			case cache.Deleted:
				delete(set, workload.UID)
			case cache.Added:
				fallthrough
			case cache.Sync:
				fallthrough
			case cache.Updated:
				fallthrough
			case cache.Replaced:
				set[workload.UID] = *workload
			default:
				return nil
			}

			for _, obj := range set {
				resources = append(resources, obj)
			}

			sort.Sort(common.ByAlphabetical(resources))
			mdef.Status.WorkloadResources = resources

			return u.kubeClient.Status().Update(ctx, mdef)
		}()

		if err != nil {
			return err
		}
	}

	return nil
}
