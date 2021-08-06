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

package razeeengine

import (
	"context"

	"github.com/InVisionApp/go-health/v2"
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/mailbox"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/processorsenders"
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
	razeeStore *RazeeStoreRunnable,
	mailbox *mailbox.Mailbox,
	razeeProcessor *processorsenders.RazeeProcessor,
	razeeChannelProducer *mailbox.RazeeChannelProducer,
) Runnables {
	// this is the start up order
	return Runnables{
		mailbox,
		razeeChannelProducer,
		razeeProcessor,
		razeeStore,
	}
}

var RunnablesSet = wire.NewSet(
	mailbox.ProvideMailbox,
	ProvideRunnables,
	ProvideRazeeStoreRunnable,
	processorsenders.ProvideRazeeProcessor,
	mailbox.ProvideRazeeChannelProducer,
)
