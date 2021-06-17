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

package mailbox

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/dictionary"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/meterdefinition"
	"k8s.io/client-go/tools/cache"
)

type MailboxChannelProducer struct {
	queue   cache.Queue
	mailbox *Mailbox
	log     logr.Logger
	name    ChannelName
}

type ObjectChannelProducer struct {
	MailboxChannelProducer
}

func ProvideObjectChannelProducer(
	meterDefinitionStore *meterdefinition.MeterDefinitionStore,
	mb *Mailbox,
	log logr.Logger,
) *ObjectChannelProducer {
	return &ObjectChannelProducer{
		MailboxChannelProducer: MailboxChannelProducer{
			queue:   meterDefinitionStore,
			mailbox: mb,
			log:     log.WithName("objectChannel").V(4),
			name:    ObjectChannel,
		},
	}
}

type MeterDefinitionChannelProducer struct {
	MailboxChannelProducer
}

func ProvideMeterDefinitionChannelProducer(
	dictionary *dictionary.MeterDefinitionDictionary,
	mb *Mailbox,
	log logr.Logger,
) *MeterDefinitionChannelProducer {
	return &MeterDefinitionChannelProducer{
		MailboxChannelProducer: MailboxChannelProducer{
			queue:   dictionary,
			mailbox: mb,
			log:     log.WithName("mdefChannel").V(4),
			name:    MeterDefinitionChannel,
		},
	}
}

// Start will start processing pops from the dictionary. Will block
func (w *MailboxChannelProducer) Start(ctx context.Context) error {
	go func() {
		select {
		case <-ctx.Done():
			w.queue.Close()
		}
	}()

	go func() {
		for {
			obj, err := w.queue.Pop(cache.PopProcessFunc(w.handlePop))
			if err != nil {
				if err == cache.ErrFIFOClosed {
					w.log.Error(err, "pop is closed")
					return
				}

				w.log.Error(err, "error processing")
				w.queue.AddIfNotPresent(obj)
			}
		}
	}()

	return nil
}

func (w *MailboxChannelProducer) handlePop(i interface{}) error {
	deltas, ok := i.(cache.Deltas)
	if !ok {
		return errors.New("obj is not a delta")
	}

	w.log.Info("handling deltas", "deltas", len(deltas))

	for i := range deltas {
		if err := w.mailbox.Broadcast(w.name, deltas[i]); err != nil {
			return err
		}
	}

	return nil
}
