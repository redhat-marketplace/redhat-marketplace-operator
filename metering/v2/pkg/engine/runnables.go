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

package engine

import (
	"context"

	// "github.com/InVisionApp/go-health/v2"
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/filter"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/mailbox"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/processors"
)

type Runnables []Runnable

type Runnable interface {
	// Start starts the runnable and does not block.
	Start(context.Context) error
}

type Stoppable interface {
	Stop()
}

type RunAndStop interface {
	Runnable
	Stoppable
}

/*
	type Recoverable interface {
		RegisterHealthCheck(func([]*health.Config))
		Recover()
	}
*/
func ProvideRunnables(
	meterDefinitionStore *MeterDefinitionStoreRunnable,
	meterDefinitionDictionary *MeterDefinitionDictionaryStoreRunnable,
	mb *mailbox.Mailbox,
	statusProcessor *processors.StatusProcessor,
	serviceAnnotatorProcessor *processors.ServiceAnnotatorProcessor,
	prometheusProcessor *processors.PrometheusProcessor,
	prometheusMdefProcessor *processors.PrometheusMdefProcessor,
	removalWatcher *processors.MeterDefinitionRemovalWatcher,
	objectChannelProducer *mailbox.ObjectChannelProducer,
	mdefChannelProducer *mailbox.MeterDefinitionChannelProducer,
	nsWatcher *filter.NamespaceWatcher,
) Runnables {
	// this is the start up order
	return Runnables{
		mb,
		objectChannelProducer,
		mdefChannelProducer,
		statusProcessor,
		//This was for annotating ServiceMonitors for RHM prom selector, should no longer be necessary
		//serviceAnnotatorProcessor,
		prometheusProcessor,
		prometheusMdefProcessor,
		removalWatcher,
		meterDefinitionDictionary,
		meterDefinitionStore,
	}
}

var RunnablesSet = wire.NewSet(
	mailbox.ProvideMailbox,
	ProvideRunnables,
	ProvideMeterDefinitionStoreRunnable,
	ProvideMeterDefinitionDictionaryStoreRunnable,
	processors.ProvideMeterDefinitionRemovalWatcher,
	//This was for annotating ServiceMonitors for RHM prom selector, should no longer be necessary
	//processors.ProvideServiceAnnotatorProcessor,
	processors.ProvideStatusProcessor,
	processors.ProvidePrometheusProcessor,
	processors.ProvidePrometheusMdefProcessor,
	mailbox.ProvideObjectChannelProducer,
	mailbox.ProvideMeterDefinitionChannelProducer,
	filter.ProvideNamespaceWatcher,
)
