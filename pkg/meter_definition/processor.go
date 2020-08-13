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

package meter_definition

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
)

// Processor provies and interface for starting a job runner of some sort. It takes a
// context and is meant to be blocking. Run in a go routine if you want to be non-blocking
type Processor interface {
	// Start is blocking and starts the process.
	Start(context.Context) error
}

// ObjectResourceMessageProcessor defines a function to process
// object resource messages arriving from the store.
type ObjectResourceMessageProcessor interface {
	// Process is the function that will process messages.
	Process(context.Context, *ObjectResourceMessage) error
}

// processorImpl creates a listener and processes new object
// resource messages to update the meterdefinition status.
type processorImpl struct {
	log logr.Logger
	cc  ClientCommandRunner

	retryCount    int
	digestersSize int
	resourceChan  chan *ObjectResourceMessage

	meterDefStore *MeterDefinitionStore
	processor     ObjectResourceMessageProcessor
}

// NewProcessor creates a processor implementation. It sets some
// basic defaults and registers listeners with the meter definition
// store when Start is called. This returns the default implementation
// of a processor that supports retry and multiple concurrent workers.
func NewProcessor(
	log logr.Logger,
	cc ClientCommandRunner,
	meterDefStore *MeterDefinitionStore,
	processor ObjectResourceMessageProcessor,
) Processor {
	return &processorImpl{
		log:           log,
		cc:            cc,
		meterDefStore: meterDefStore,
		processor:     processor,
		digestersSize: 4,
		retryCount:    3,
	}
}

// Start registers the processor to the meter definition
// store and then executes a number of go routines that
// equal the max digestersSize value on the struct. The work will then
// wait until the context is closed, and the wait group is finished to exit.
func (u *processorImpl) Start(ctx context.Context) error {
	u.resourceChan = make(chan *ObjectResourceMessage)
	u.meterDefStore.RegisterListener(u.resourceChan)

	var wg sync.WaitGroup

	wg.Add(u.digestersSize)

	for i := 0; i < u.digestersSize; i++ {
		go func() {
			for data := range u.resourceChan {
				err := utils.Retry(
					func() error {
						return u.processor.Process(ctx, data)
					}, u.retryCount)
				if err != nil {
					u.log.Error(err, "error processing message")
				}
			}
			wg.Done()
		}()
	}

	<-ctx.Done()
	close(u.resourceChan)

	wg.Wait()
	return nil
}
