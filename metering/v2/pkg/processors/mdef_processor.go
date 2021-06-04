package processors

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/dictionary"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/meterdefinition"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
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
	log logr.Logger,
) *MeterDefinitionRemovalWatcher {
	return &MeterDefinitionRemovalWatcher{
		dictionary:           dictionary,
		meterDefinitionStore: meterDefinitionStore,
		messageChan:          make(chan cache.Delta),
		log:                  log,
	}
}

func (w *MeterDefinitionRemovalWatcher) Process(ctx context.Context, d cache.Delta) error {
	if d.Type != cache.Deleted {
		return nil
	}

	obj, _ := d.Object.(*v1beta1.MeterDefinition)
	key, err := cache.MetaNamespaceKeyFunc(obj)

	if err != nil {
		w.log.Error(err, "error creating key")
		return err
	}

	objects, err := w.meterDefinitionStore.ByIndex("meterDefinition", key)

	if err != nil {
		w.log.Error(err, "error finding data")
		return err
	}

	for _, obj := range objects {
		err := w.dictionary.Delete(obj)

		if err != nil {
			w.log.Error(err, "error deleting data")
		}
	}

	return nil
}
