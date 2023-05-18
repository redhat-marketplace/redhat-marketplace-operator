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
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/dataservice"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
)

// Must Start, Process and Send
type EventProcessorSender interface {
	Start(ctx context.Context) error
	Process(context.Context, Event) error
	Send(context.Context, string) error
}

// Process events on the Event Channel and send when conditions are met
type ProcessorSender struct {
	log logr.Logger

	//retryCount    int
	digestersSize int
	EventChan     chan Event

	EventProcessorSender

	sendReadyChan chan string

	eventAccumulator *EventAccumulator

	eventReporter *EventReporter

	config *Config
}

func (p *ProcessorSender) Start(ctx context.Context) error {
	ticker := time.NewTicker(p.config.MaxFlushTimeout.Duration)
	defer ticker.Stop()

	p.EventChan = make(chan Event)
	p.sendReadyChan = make(chan string)

	p.eventAccumulator = &EventAccumulator{}
	p.eventAccumulator.eventMap = make(map[string]EventJsons)

	eventReporter, err := NewEventReporter(p.log, p.config)
	if err != nil {
		return err
	}
	p.eventReporter = eventReporter

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

	go func() {

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.log.V(4).Info("Timer expired. SendAll.")
				if err := p.SendAll(ctx); err != nil {
					p.log.Error(err, "error sending event data")
				}
			}
		}
	}()

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
	if len >= p.config.MaxEventEntries {
		p.sendReadyChan <- event.User
	}

	// If the map is at maximum size, signal to send

	return nil
}

func (p *ProcessorSender) Send(ctx context.Context, user string) error {

	// flush entries for this key
	eventJsons := p.eventAccumulator.Flush(user)

	// Build and Send the report to dataService
	// There is a case where if an Userkey is removed, the metadata will no longer be available when the Report it sent
	metadata := p.config.UserConfigs.GetMetadata(user)
	if err := p.eventReporter.Report(metadata, eventJsons); err != nil {
		return err
	}

	p.log.Info("Sent Report")

	return nil
}

func (p *ProcessorSender) SendAll(ctx context.Context) error {

	// flush entire map
	eventMap := p.eventAccumulator.FlushAll()

	for key := range eventMap {
		eventJsons := eventMap[key]

		// Build and Send the report to dataService
		// There is a case where if an Userkey is removed, the metadata will no longer be available when the Report it sent
		metadata := p.config.UserConfigs.GetMetadata(key)
		if err := p.eventReporter.Report(metadata, eventJsons); err != nil {
			return err
		}

		p.log.Info("Sent Report")
	}

	return nil
}

func (p *ProcessorSender) provideDataServiceConfig() (*dataservice.DataServiceConfig, error) {
	cert, err := os.ReadFile(p.config.DataServiceCertFile)
	if err != nil {
		return nil, err
	}

	var serviceAccountToken = ""
	if p.config.DataServiceTokenFile != "" {
		content, err := os.ReadFile(p.config.DataServiceTokenFile)
		if err != nil {
			return nil, err
		}
		serviceAccountToken = string(content)
	}

	var dataServiceDNS = fmt.Sprintf("%s.%s.svc:8004", utils.DATA_SERVICE_NAME, p.config.Namespace)

	return &dataservice.DataServiceConfig{
		Address:          dataServiceDNS,
		DataServiceToken: serviceAccountToken,
		DataServiceCert:  cert,
		OutputPath:       p.config.OutputDirectory,
	}, nil
}
