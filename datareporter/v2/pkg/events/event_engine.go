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

type EventEngine struct {
	log logr.Logger
	*ProcessorSender

	mainContext  *context.Context
	localContext *context.Context
	cancelFunc   context.CancelFunc
}

func NewEventEngine(
	ctx context.Context,
	log logr.Logger,
) *EventEngine {
	ee := &EventEngine{
		log: log.WithValues("process", "EventEngine"),
		ProcessorSender: &ProcessorSender{
			log:           log,
			digestersSize: 1,
		},
	}

	return ee
}

func (e *EventEngine) Start(ctx context.Context) error {
	if e.cancelFunc != nil {
		e.mainContext = nil
		e.localContext = nil
		e.cancelFunc()
	}

	localCtx, cancel := context.WithCancel(ctx)
	e.mainContext = &ctx
	e.localContext = &localCtx
	e.cancelFunc = cancel

	wg := sync.WaitGroup{}
	wg.Add(1)

	e.log.Info("Starting Engine")
	go func() {
		wg.Done()
		e.ProcessorSender.Start(localCtx)
	}()

	wg.Wait()

	return nil
}
