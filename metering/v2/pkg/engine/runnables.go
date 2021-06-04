package engine

import (
	"context"

	"github.com/InVisionApp/go-health/v2"
	"github.com/google/wire"
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
	pvc *PVCListerRunnable,
	pod *PodListerRunnable,
	service *ServiceListerRunnable,
	mdef *MeterDefinitionListerRunnable,
	mailbox *mailbox.Mailbox,
	statusProcessor *processors.StatusProcessor,
	serviceAnnotatorProcessor *processors.ServiceAnnotatorProcessor,
	prometheusProcessor *processors.PrometheusProcessor,
	objectChannelProducer *mailbox.ObjectChannelProducer,
	mdefChannelProducer *mailbox.MeterDefinitionChannelProducer,
) Runnables {
	return Runnables{
		(*ListerRunnable)(mdef),
		(*ListerRunnable)(pvc),
		(*ListerRunnable)(pod),
		(*ListerRunnable)(service),
		mailbox,
		statusProcessor,
		serviceAnnotatorProcessor,
		objectChannelProducer,
		mdefChannelProducer,
		prometheusProcessor,
	}
}

var RunnablesSet = wire.NewSet(
	mailbox.ProvideMailbox,
	ProvideMeterDefinitionListerRunnable,
	ProvidePVCLister,
	ProvideRunnables,
	ProvidePodListerRunnable,
	ProvideServiceListerRunnable,
	ProvideServiceMonitorListerRunnable,
	processors.ProvideMeterDefinitionRemovalWatcher,
	processors.ProvideServiceAnnotatorProcessor,
	processors.ProvideStatusProcessor,
	processors.ProvidePrometheusProcessor,
	mailbox.ProvideObjectChannelProducer,
	mailbox.ProvideMeterDefinitionChannelProducer,
)
