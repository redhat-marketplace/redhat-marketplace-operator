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
		log:                  log,
	}
	sp.Processor.DeltaProcessor = sp
	return sp
}

func (w *MeterDefinitionRemovalWatcher) Process(ctx context.Context, d cache.Delta) error {
	if d.Type != cache.Deleted {
		return nil
	}

	meterdef, ok := d.Object.(*dictionary.MeterDefinitionExtended)

	if !ok {
		return errors.New("encountered unexpected type")
	}

	key, err := cache.MetaNamespaceKeyFunc(&meterdef.MeterDefinition)

	if err != nil {
		w.log.Error(err, "error creating key")
		return errors.WithStack(err)
	}

	objects, err := w.meterDefinitionStore.ByIndex("meterDefinition", key)

	if err != nil {
		w.log.Error(err, "error finding data")
		return errors.WithStack(err)
	}

	for _, obj := range objects {
		err := w.dictionary.Delete(obj)

		if err != nil {
			w.log.Error(err, "error deleting data")
		}
	}

	return nil
}
