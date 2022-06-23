// Copyright 2021 IBM Corp.
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
	"fmt"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/filter"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/mailbox"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/stores"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MeterDefinitionRemovalWatcher struct {
	*Processor
	dictionary           *stores.MeterDefinitionDictionary
	meterDefinitionStore *stores.MeterDefinitionStore
	nsWatcher            *filter.NamespaceWatcher
	messageChan          chan cache.Delta
	log                  logr.Logger
	kubeClient           client.Client
}

func ProvideMeterDefinitionRemovalWatcher(
	dictionary *stores.MeterDefinitionDictionary,
	meterDefinitionStore *stores.MeterDefinitionStore,
	mb *mailbox.Mailbox,
	log logr.Logger,
	kubeClient client.Client,
	nsWatcher *filter.NamespaceWatcher,
) *MeterDefinitionRemovalWatcher {
	sp := &MeterDefinitionRemovalWatcher{
		Processor: &Processor{
			log:           log,
			digestersSize: 1,
			retryCount:    3,
			mailbox:       mb,
			channelName:   mailbox.MeterDefinitionChannel,
		},
		dictionary:           dictionary,
		meterDefinitionStore: meterDefinitionStore,
		messageChan:          make(chan cache.Delta),
		log:                  log.WithName("meterDefinitionRemovalWatcher"),
		kubeClient:           kubeClient,
		nsWatcher:            nsWatcher,
	}
	sp.Processor.DeltaProcessor = sp
	return sp
}

func (w *MeterDefinitionRemovalWatcher) Process(ctx context.Context, d cache.Delta) error {
	if d.Type == cache.Deleted {
		return w.onDelete(ctx, d)
	}

	if d.Type == cache.Added {
		return w.onAdd(ctx, d)
	}

	if d.Type == cache.Updated {
		return w.onUpdate(ctx, d)
	}

	if d.Type == cache.Sync {
		return w.onSync(ctx, d)
	}

	return nil
}

// A MeterDefinition add/update/sync must Resync Objects in case the workloadFilter was updated, in order to match appropriate Objects
// A MeterDefinition delete must Resync Objects that are associated with the MeterDefinition
// The store Resync function decides to emit a delta update or delete if a MeterDefinition still matches the Object

func (w *MeterDefinitionRemovalWatcher) onAdd(_ context.Context, d cache.Delta) error {
	meterdef, ok := d.Object.(*stores.MeterDefinitionExtended)

	if !ok {
		return errors.New("encountered unexpected type")
	}

	w.log.Info("processing meterdef", "name/namespace", meterdef.Name+"/"+meterdef.Namespace, "namespaces", fmt.Sprintf("%+v", meterdef.Filter.GetNamespaces()))
	w.nsWatcher.AddNamespace(client.ObjectKeyFromObject(meterdef.MeterDefinition), meterdef.Filter.GetNamespaces())

	return w.meterDefinitionStore.SyncByIndex(stores.IndexNamespace, meterdef.GetNamespace())
}

func (w *MeterDefinitionRemovalWatcher) onDelete(_ context.Context, d cache.Delta) error {
	meterdef, ok := d.Object.(*stores.MeterDefinitionExtended)

	if !ok {
		return errors.New("encountered unexpected type")
	}

	w.log.Info("deleting meterdef", "name/namespace", meterdef.Name+"/"+meterdef.Namespace)
	w.nsWatcher.RemoveNamespace(client.ObjectKeyFromObject(meterdef.MeterDefinition))

	key, err := cache.MetaNamespaceKeyFunc(meterdef.MeterDefinition)

	if err != nil {
		w.log.Error(err, "error creating key")
		return errors.WithStack(err)
	}

	return w.meterDefinitionStore.SyncByIndex(stores.IndexMeterDefinition, key)
}

func (w *MeterDefinitionRemovalWatcher) onUpdate(_ context.Context, d cache.Delta) error {
	meterdef, ok := d.Object.(*stores.MeterDefinitionExtended)
	if !ok {
		return errors.New("encountered unexpected type")
	}

	w.nsWatcher.AddNamespace(client.ObjectKeyFromObject(meterdef.MeterDefinition), meterdef.Filter.GetNamespaces())

	return w.meterDefinitionStore.SyncByIndex(stores.IndexNamespace, meterdef.GetNamespace())
}

// Clear Status.WorkloadResources on initial sync such that Status is correct when metric-state starts
func (w *MeterDefinitionRemovalWatcher) onSync(_ context.Context, d cache.Delta) error {
	meterdef, ok := d.Object.(*stores.MeterDefinitionExtended)
	if !ok {
		return errors.New("encountered unexpected type")
	}

	w.nsWatcher.AddNamespace(client.ObjectKeyFromObject(meterdef.MeterDefinition), meterdef.Filter.GetNamespaces())

	return w.meterDefinitionStore.SyncByIndex(stores.IndexNamespace, meterdef.GetNamespace())
}
