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
	"sort"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/mailbox"
	pkgtypes "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StatusFlushDuration time.Duration

// StatusProcessor will update the meter definition
// status with the objects that matched it.
type StatusProcessor struct {
	*Processor
	log        logr.Logger
	kubeClient client.Client
	mutex      sync.Mutex

	flushTime time.Duration
	resources map[client.ObjectKey]*updateableStatus
}

type updateableStatus struct {
	needsUpdate bool
	resources   map[types.UID]*common.WorkloadResource
}

func newUpdateableStatus() *updateableStatus {
	return &updateableStatus{
		needsUpdate: true,
		resources:   make(map[types.UID]*common.WorkloadResource),
	}
}

// NewStatusProcessor is the provider that creates
// the processor.
func ProvideStatusProcessor(
	log logr.Logger,
	kubeClient client.Client,
	mb *mailbox.Mailbox,
	duration StatusFlushDuration,
	_ managers.CacheIsStarted,
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
		resources:  make(map[client.ObjectKey]*updateableStatus),
		flushTime:  time.Duration(duration),
	}

	sp.Processor.DeltaProcessor = sp
	return sp
}

func (u *StatusProcessor) Start(ctx context.Context) error {
	tick := time.NewTicker(u.flushTime)
	go func() {
		for {
			u.flush(ctx)
			select {
			case <-tick.C:
			case <-ctx.Done():
				return
			}
		}
	}()

	return u.Processor.Start(ctx)
}

func (u *StatusProcessor) flush(ctx context.Context) {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	deleted := []types.NamespacedName{}
	mdef := &marketplacev1beta1.MeterDefinition{}

	for key, status := range u.resources {
		if !status.needsUpdate {
			continue
		}

		u.log.Info("flushing status",
			"key", key,
			"resources", len(status.resources),
		)

		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			err := u.kubeClient.Get(ctx, key, mdef)
			// it's not found, let's delete it
			if err != nil && k8serrors.IsNotFound(err) {
				deleted = append(deleted, key)
				return nil
			}

			// random err, return it for backoff and retry
			if err != nil {
				return err
			}

			// is there really a change?
			equal := true

			// if they don't have the same length
			if len(mdef.Status.WorkloadResources) != len(status.resources) {
				equal = false
			} else {
				// if a UID is not the same between the two resources
				for i := range mdef.Status.WorkloadResources {
					res1 := mdef.Status.WorkloadResources[i]
					_, ok := status.resources[res1.UID]
					if !ok {
						equal = false
					}
				}
			}

			// no op if the resources are equal
			if equal {
				return nil
			}

			mdef.Status.WorkloadResources = make([]common.WorkloadResource, 0, len(status.resources))

			for i := range status.resources {
				mdef.Status.WorkloadResources = append(mdef.Status.WorkloadResources, *status.resources[i])
			}

			sort.Sort(common.ByAlphabetical(mdef.Status.WorkloadResources))
			return u.kubeClient.Status().Update(ctx, mdef)
		})

		if err != nil {
			u.log.Error(err, "failed to update", "key", key)
		}

		status.needsUpdate = false
	}

	// remove resources that are not found
	for _, name := range deleted {
		delete(u.resources, name)
	}
}

// Process will receive a new ObjectResourceMessage and find and update the metere
// definition associated with the object. To prevent gaps, it bulk retrieves the
// resources and checks it against the status.
func (u *StatusProcessor) Process(ctx context.Context, inObj cache.Delta) error {
	enhancedObj, ok := inObj.Object.(*pkgtypes.MeterDefinitionEnhancedObject)

	if !ok {
		return errors.WithStack(errors.New("obj is unexpected type"))
	}

	u.log.V(2).Info("updating status",
		"obj", enhancedObj.GetName()+"/"+enhancedObj.GetNamespace(),
		"mdefs", len(enhancedObj.MeterDefinitions),
	)

	u.mutex.Lock()
	defer u.mutex.Unlock()

	var status *updateableStatus

	for i := range enhancedObj.MeterDefinitions {
		key := client.ObjectKeyFromObject(enhancedObj.MeterDefinitions[i])

		if status, ok = u.resources[key]; !ok {
			status = newUpdateableStatus()
			u.resources[key] = status
		}

		for k := range status.resources {
			obj := status.resources[k]
			status.resources[obj.UID] = obj
		}

		workload, err := common.NewWorkloadResource(enhancedObj.Object)

		if err != nil {
			return errors.WithStack(err)
		}

		switch inObj.Type {
		case cache.Deleted:
			delete(status.resources, workload.UID)
		case cache.Replaced, cache.Added, cache.Sync, cache.Updated:
			status.resources[workload.UID] = workload
		default:
			return nil
		}

		status.needsUpdate = true
	}

	return nil
}
