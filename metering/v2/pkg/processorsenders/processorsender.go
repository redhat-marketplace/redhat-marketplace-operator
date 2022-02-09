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

package processorsenders

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/mailbox"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

const timerDuration time.Duration = 1 * time.Second

type DeltaProcessorSender interface {
	// Process is the function that will process messages.
	Process(context.Context, cache.Delta) error
	Send(context.Context) error
}

// processorImpl creates a listener and processes new object
// resource messages to update the meterdefinition status.
type ProcessorSender struct {
	log logr.Logger

	channelName   mailbox.ChannelName
	retryCount    int
	digestersSize int
	resourceChan  chan cache.Delta

	DeltaProcessorSender

	mailbox *mailbox.Mailbox

	sendReadyChan chan bool
}

// Start functions as a Processor, with an extra thread for Sending
// When either the Processor indicates the accumulator is full, or the timer expires, Send
func (u *ProcessorSender) Start(ctx context.Context) error {
	u.resourceChan = make(chan cache.Delta)
	u.mailbox.RegisterListener(u.channelName, u.resourceChan)

	var wg sync.WaitGroup

	wg.Add(u.digestersSize)

	u.sendReadyChan = make(chan bool)
	timer := time.NewTimer(timerDuration)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-u.sendReadyChan:
				err := u.Send(ctx)
				if err != nil {
					u.log.Error(err, "ProcessorSender Send error")
				}
			case <-timer.C:
				err := u.Send(ctx)
				if err != nil {
					u.log.Error(err, "ProcessorSender Send error")
				}
			}
		}
	}()

	for i := 0; i < u.digestersSize; i++ {
		go func() {
			for data := range u.resourceChan {
				// restart the send timer if new data is arriving
				timer.Reset(timerDuration)
				localData := data
				err := retry.RetryOnConflict(retry.DefaultBackoff,
					func() error {
						return u.Process(ctx, localData)
					})
				if err != nil {
					u.log.Error(err, "error processing message")
				}
			}
			wg.Done()
		}()
	}

	<-ctx.Done()
	u.log.Info("processor is shutting down", "processor", fmt.Sprintf("%T", u.DeltaProcessorSender))
	close(u.resourceChan)
	wg.Wait()
	return nil
}
