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
	"fmt"
	"time"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/sasha-s/go-deadlock"
	"k8s.io/client-go/tools/cache"
)

type Mailbox struct {
	// listeners are used for downstream
	mutex     deadlock.RWMutex
	listeners map[ChannelName][]chan cache.Delta

	log logr.Logger
}

type ChannelName string

const (
	ObjectChannel          ChannelName = "ObjectChannel"
	MeterDefinitionChannel ChannelName = "MeterDefinitionChannel"
	RazeeChannel           ChannelName = "RazeeChannel"
)

var (
	channels = []ChannelName{ObjectChannel, MeterDefinitionChannel, RazeeChannel}
)

func ProvideMailbox(log logr.Logger) *Mailbox {
	return &Mailbox{
		log:       log.WithName("mailbox").V(4),
		listeners: make(map[ChannelName][]chan cache.Delta),
	}
}

func (s *Mailbox) Start(ctx context.Context) error {
	go func() {
		ticker := time.NewTicker(60 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	return nil
}

func (s *Mailbox) RegisterListener(channelName ChannelName, ch chan cache.Delta) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.log.Info("registering listener", "name", channelName)

	var (
		ok        bool
		listeners []chan cache.Delta
	)
	if listeners, ok = s.listeners[channelName]; !ok {
		listeners = []chan cache.Delta{}
	}

	listeners = append(listeners, ch)
	s.listeners[channelName] = listeners
}

func (s *Mailbox) Broadcast(channelName ChannelName, obj cache.Delta) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	logger := s.log.WithValues("ChannelName", channelName).V(4)

	listeners, ok := s.listeners[channelName]
	if !ok {
		return errors.NewWithDetails("channel name doesn't exist", "channel", channelName)
	}

	logger.Info("sending message",
		"obj", fmt.Sprintf("%+v", obj),
		"listeners", len(listeners))

	for _, listener := range listeners {
		listener <- obj
	}

	logger.Info("done sending message",
		"obj", fmt.Sprintf("%+v", obj))

	return nil
}
