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

package processors

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/mailbox"
	"k8s.io/client-go/tools/cache"
)

// ObjectResourceMessageProcessor defines a function to process
// object resource messages arriving from the store.
type DeltaProcessor interface {
	// Process is the function that will process messages.
	Process(context.Context, cache.Delta) error
}

// processorImpl creates a listener and processes new object
// resource messages to update the meterdefinition status.
type Processor struct {
	log logr.Logger

	channelName   mailbox.ChannelName
	retryCount    int
	digestersSize int
	resourceChan  chan cache.Delta

	DeltaProcessor

	mailbox *mailbox.Mailbox
}

// Start registers the processor to the meter definition
// store and then executes a number of go routines that
// equal the max digestersSize value on the struct. The work will then
// wait until the context is closed, and the wait group is finished to exit.
func (u *Processor) Start(ctx context.Context) error {
	u.resourceChan = make(chan cache.Delta)
	u.mailbox.RegisterListener(u.channelName, u.resourceChan)

	var wg sync.WaitGroup

	wg.Add(u.digestersSize)
	for i := 0; i < u.digestersSize; i++ {
		go func() {
			for data := range u.resourceChan {
				localData := data
				err := u.Process(ctx, localData)
				if err != nil {
					u.log.Error(err, "error processing message")
				}
			}
			wg.Done()
		}()
	}

	go func() {
		<-ctx.Done()
		u.log.Info("processor is shutting down", "processor", fmt.Sprintf("%T", u.DeltaProcessor))
		close(u.resourceChan)
		wg.Wait()
	}()

	return nil
}
