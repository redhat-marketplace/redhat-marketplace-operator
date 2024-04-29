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

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EventEngine struct {
	log logr.Logger
	*ProcessorSender

	mainContext  *context.Context
	localContext *context.Context
	cancelFunc   context.CancelFunc

	ready bool
}

func NewEventEngine(
	ctx context.Context,
	log logr.Logger,
	config *Config,
	client client.Client,
) *EventEngine {
	ee := &EventEngine{
		log: log.WithValues("process", "EventEngine"),
		ProcessorSender: &ProcessorSender{
			log:           log,
			digestersSize: 1,
			config:        config,
			client:        client,
		},
	}

	return ee
}

// EventEngine Processor channels are ready to receive event after the ready channel is closed
// Error is returned if there is a configuration problem
func (e *EventEngine) Start(ctx context.Context) error {

	// Check client was set
	if e.client == nil {
		return errors.New("eventEngine k8sclient is nil")
	}

	if e.cancelFunc != nil {
		e.mainContext = nil
		e.localContext = nil
		e.cancelFunc()
	}

	localCtx, cancel := context.WithCancel(ctx)
	e.mainContext = &ctx
	e.localContext = &localCtx
	e.cancelFunc = cancel

	ready := make(chan bool)
	go func() {
		<-ready
		e.ready = true
	}()

	return e.ProcessorSender.Start(localCtx, ready)
}

func (e *EventEngine) IsReady() bool {
	return e.ready
}

func (e *EventEngine) SetKubeClient(client client.Client) {
	e.client = client
}
