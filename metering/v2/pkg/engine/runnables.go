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
	meterDefinitionStore *MeterDefinitionStoreRunnable,
	meterDefinitionDictionary *MeterDefinitionDictionaryStoreRunnable,
	mdefSeenStore *MeterDefinitionSeenStoreRunnable,
	mailbox *mailbox.Mailbox,
	statusProcessor *processors.StatusProcessor,
	serviceAnnotatorProcessor *processors.ServiceAnnotatorProcessor,
	prometheusProcessor *processors.PrometheusProcessor,
	prometheusMdefProcessor *processors.PrometheusMdefProcessor,
	removalWatcher *processors.MeterDefinitionRemovalWatcher,
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
		removalWatcher,
		meterDefinitionDictionary,
		mdefSeenStore,
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
