package engine

import (
	"context"

	"github.com/InVisionApp/go-health/v2"
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/dictionary"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/mailbox"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/processors"
)

type Runnables []Runnable

type Runnable interface {
	Start(context.Context) error
}

type Recoverable interface {
	RegisterHealthCheck(func([]*health.Config))
	Recover()
}

func ProvideRunnables(
	objectsSeenStore *ObjectsSeenStoreRunnable,
	meterDefinitionStore *MeterDefinitionStoreRunnable,
	meterDefinitionDictionary *MeterDefinitionDictionaryStoreRunnable,
	mdefSeenStore *MeterDefinitionSeenStoreRunnable,
	mailbox *mailbox.Mailbox,
	statusProcessor *processors.StatusProcessor,
	serviceAnnotatorProcessor *processors.ServiceAnnotatorProcessor,
	prometheusProcessor *processors.PrometheusProcessor,
	prometheusMdefProcessor *processors.PrometheusMdefProcessor,
	objectChannelProducer *mailbox.ObjectChannelProducer,
	mdefChannelProducer *mailbox.MeterDefinitionChannelProducer,
	dictionary *dictionary.MeterDefinitionDictionary,
) Runnables {
	// this is the start up order
	return Runnables{
		mailbox,
		objectChannelProducer,
		mdefChannelProducer,
		statusProcessor,
		serviceAnnotatorProcessor,
		prometheusProcessor,
		prometheusMdefProcessor,
		meterDefinitionDictionary,
		meterDefinitionStore,
		dictionary,
	}
}

var RunnablesSet = wire.NewSet(
	mailbox.ProvideMailbox,
	ProvideRunnables,
	ProvideObjectsSeenStoreRunnable,
	ProvideMeterDefinitionStoreRunnable,
	ProvideMeterDefinitionDictionaryStoreRunnable,
	ProvideMeterDefinitionSeenStoreRunnable,
	processors.ProvideMeterDefinitionRemovalWatcher,
	processors.ProvideServiceAnnotatorProcessor,
	processors.ProvideStatusProcessor,
	processors.ProvidePrometheusProcessor,
	processors.ProvidePrometheusMdefProcessor,
	mailbox.ProvideObjectChannelProducer,
	mailbox.ProvideMeterDefinitionChannelProducer,
)
