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
	"time"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/mailbox"
	pkgtypes "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/sasha-s/go-deadlock"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StatusProcessor will update the meter definition
// status with the objects that matched it.
type StatusProcessor struct {
	*Processor
	log        logr.Logger
	kubeClient client.Client
	mutex      deadlock.Mutex
	scheme     *runtime.Scheme

	resourcesNeedUpdate map[client.ObjectKey]bool
	resources           map[client.ObjectKey][]common.WorkloadResource
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
		log:                 log.WithValues("process", "statusProcessor"),
		kubeClient:          kubeClient,
		scheme:              scheme,
		resourcesNeedUpdate: make(map[client.ObjectKey]bool),
		resources:           make(map[client.ObjectKey][]common.WorkloadResource),
	}

	sp.Processor.DeltaProcessor = sp
	return sp
}

func (u *StatusProcessor) Start(ctx context.Context) error {
	tick := time.NewTicker(time.Minute)
	go func() {
		for {
			select {
			case <-tick.C:
				u.flush(ctx)
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

	for key, resources := range u.resources {
		if v, ok := u.resourcesNeedUpdate[key]; ok && !v {
			continue
		}

		u.log.Info("flushing status",
			"key", key,
			"resources", len(resources),
		)

		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			mdef := &marketplacev1beta1.MeterDefinition{}

			if err := u.kubeClient.Get(ctx, key, mdef); err != nil {
				return err
			}

			mdef.Status.WorkloadResources = resources

			return u.kubeClient.Status().Update(ctx, mdef)
		})

		if err != nil {
			u.log.Error(err, "failed to update", "key", key)
		}

		u.resourcesNeedUpdate[key] = false
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

	u.log.Info("updating status",
		"obj", enhancedObj.GetName()+"/"+enhancedObj.GetNamespace(),
		"mdefs", len(enhancedObj.MeterDefinitions),
	)

	u.mutex.Lock()
	defer u.mutex.Unlock()

	for i := range enhancedObj.MeterDefinitions {
		key := client.ObjectKeyFromObject(&enhancedObj.MeterDefinitions[i])

		var resources []common.WorkloadResource
		if resources, ok = u.resources[key]; !ok {
			u.resources[key] = make([]common.WorkloadResource, 0, 0)
			resources = u.resources[key]
		}

		set := map[types.UID]common.WorkloadResource{}
		for _, obj := range resources {
			set[obj.UID] = obj
		}

		workload, err := common.NewWorkloadResource(enhancedObj.Object, u.scheme)

		if err != nil {
			return errors.WithStack(err)
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

		newResources := make([]common.WorkloadResource, 0, len(set))
		for i := range set {
			newResources = append(newResources, set[i])
		}
		sort.Sort(common.ByAlphabetical(newResources))
		u.resources[key] = newResources
		u.resourcesNeedUpdate[key] = true
	}

	return nil
}
