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
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/cache"
)

type Mailbox struct {
	// listeners are used for downstream
	mutex     sync.RWMutex
	listeners map[ChannelName][]chan cache.Delta

	log logr.Logger
}

type ChannelName string

const (
	ObjectChannel          ChannelName = "ObjectChannel"
	MeterDefinitionChannel ChannelName = "MeterDefinitionChannel"
)

var (
	channels = []ChannelName{ObjectChannel, MeterDefinitionChannel}
)

func ProvideMailbox(log logr.Logger) *Mailbox {
	return &Mailbox{
		log:       log,
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
				s.cleanup()
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

func (s *Mailbox) cleanup() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, name := range channels {
		if listeners, ok := s.listeners[name]; ok && len(listeners) > 0 {
			newListeners := []chan cache.Delta{}

			for i := range listeners {
				_, ok := <-listeners[i]

				if ok {
					newListeners = append(newListeners, listeners[i])
				}
			}

			if len(listeners) != len(newListeners) {
				s.listeners[name] = newListeners
			}
		}
	}
}

func (s *Mailbox) Broadcast(channelName ChannelName, obj cache.Delta) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if listeners, ok := s.listeners[channelName]; ok {
		for _, listener := range listeners {
			listener <- obj
		}
		return nil
	}

	return errors.NewWithDetails("channel name doesn't exist", "channel", channelName)
}
