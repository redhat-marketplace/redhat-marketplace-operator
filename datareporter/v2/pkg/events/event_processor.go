// Copyright 2023 IBM Corp.
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

package events

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
)

// Must Start, Process and Send
type EventProcessorSender interface {
	Start(ctx context.Context) error
	Process(context.Context, Event) error
	Send(context.Context, Key) error
}

// Process events on the Event Channel and send when conditions are met
type ProcessorSender struct {
	log logr.Logger

	//retryCount    int
	digestersSize int
	EventChan     chan Event

	EventProcessorSender

	sendReadyChan chan Key

	eventAccumulator *EventAccumulator
}

func (p *ProcessorSender) Start(ctx context.Context) error {
	// consider what the deadline for proces or send should be

	p.EventChan = make(chan Event)
	p.sendReadyChan = make(chan Key)

	p.eventAccumulator = &EventAccumulator{}
	p.eventAccumulator.eventMap = make(map[Key]EventJsons)

	var processWaitGroup sync.WaitGroup
	var sendWaitGroup sync.WaitGroup

	processWaitGroup.Add(p.digestersSize)
	sendWaitGroup.Add(p.digestersSize)

	for i := 0; i < p.digestersSize; i++ {
		go func() {
			for event := range p.EventChan {
				localEvent := event
				if err := p.Process(ctx, localEvent); err != nil {
					p.log.Error(err, "error processes event")
				}
			}
			processWaitGroup.Done()
		}()
	}

	for i := 0; i < p.digestersSize; i++ {
		go func() {
			for key := range p.sendReadyChan {
				localKey := key
				if err := p.Send(ctx, localKey); err != nil {
					p.log.Error(err, "error sending event data")
				}
			}
			sendWaitGroup.Done()
		}()
	}

	<-ctx.Done()
	p.log.Info("processor is shutting down")
	close(p.EventChan)
	close(p.sendReadyChan)
	processWaitGroup.Wait()
	sendWaitGroup.Wait()

	return nil
}

func (p *ProcessorSender) Process(ctx context.Context, event Event) error {

	len := p.eventAccumulator.Add(event)

	// If we are at event max, signal to send
	if len > 50 {
		p.sendReadyChan <- event.Key
	}

	// If the map is at maximum size, signal to send

	return nil
}

func (p *ProcessorSender) Send(ctx context.Context, key Key) error {

	// flush entries for this key
	_ = p.eventAccumulator.Flush(key)

	// build the report

	// POST

	p.log.Info("Send")

	return nil
}
