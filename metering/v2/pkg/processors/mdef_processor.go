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

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/dictionary"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/mailbox"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/meterdefinition"
	"k8s.io/client-go/tools/cache"
)

type MeterDefinitionRemovalWatcher struct {
	*Processor
	dictionary           *dictionary.MeterDefinitionDictionary
	meterDefinitionStore *meterdefinition.MeterDefinitionStore
	keyFunc              cache.KeyFunc

	messageChan chan cache.Delta
	log         logr.Logger
}

func ProvideMeterDefinitionRemovalWatcher(
	dictionary *dictionary.MeterDefinitionDictionary,
	meterDefinitionStore *meterdefinition.MeterDefinitionStore,
	mb *mailbox.Mailbox,
	log logr.Logger,
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

	return nil
}

func (w *MeterDefinitionRemovalWatcher) onAdd(ctx context.Context, d cache.Delta) error {
	meterdef, ok := d.Object.(*dictionary.MeterDefinitionExtended)

	if !ok {
		return errors.New("encountered unexpected type")
	}

	w.log.Info("processing meterdef", "name/namespace", meterdef.Name+"/"+meterdef.Namespace)
	return w.meterDefinitionStore.Resync()
}

func (w *MeterDefinitionRemovalWatcher) onDelete(ctx context.Context, d cache.Delta) error {
	meterdef, ok := d.Object.(*dictionary.MeterDefinitionExtended)

	if !ok {
		return errors.New("encountered unexpected type")
	}

	w.log.Info("processing meterdef", "name/namespace", meterdef.Name+"/"+meterdef.Namespace)

	key, err := cache.MetaNamespaceKeyFunc(&meterdef.MeterDefinition)

	if err != nil {
		w.log.Error(err, "error creating key")
		return errors.WithStack(err)
	}

	objects, err := w.meterDefinitionStore.ByIndex(meterdefinition.IndexMeterDefinition, key)

	if err != nil {
		w.log.Error(err, "error finding data")
		return errors.WithStack(err)
	}

	for i := range objects {
		obj := objects[i]
		w.log.Info("deleting obj", "object", obj)
		err := w.meterDefinitionStore.Delete(obj)

		if err != nil {
			w.log.Error(err, "error deleting data")
			return errors.WithStack(err)
		}
	}

	return nil
}

func (w *MeterDefinitionRemovalWatcher) onUpdate(ctx context.Context, d cache.Delta) error {
	meterdef, ok := d.Object.(*dictionary.MeterDefinitionExtended)

	if !ok {
		return errors.New("encountered unexpected type")
	}

	w.log.Info("processing meterdef", "name/namespace", meterdef.Name+"/"+meterdef.Namespace)

	key, err := cache.MetaNamespaceKeyFunc(&meterdef.MeterDefinition)

	if err != nil {
		w.log.Error(err, "error creating key")
		return errors.WithStack(err)
	}

	objects, err := w.meterDefinitionStore.ByIndex(meterdefinition.IndexMeterDefinition, key)

	if err != nil {
		w.log.Error(err, "error finding data")
		return errors.WithStack(err)
	}

	// Flush potential old objects from the index associated with the meterdefinition

	for i := range objects {
		obj := objects[i]
		w.log.Info("deleting obj from index", "object", obj)
		err := w.meterDefinitionStore.DeleteFromIndex(obj)

		if err != nil {
			w.log.Error(err, "error deleting data")
			return errors.WithStack(err)
		}
	}

	// Resync to index objects associated with the meterdefinition

	return w.meterDefinitionStore.Resync()
}
