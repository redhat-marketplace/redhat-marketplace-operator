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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("EngineTest", func() {
	var err error

	var cfg *Config

	var ctx context.Context
	var cancel context.CancelFunc

	var engine *EventEngine

	var eventone Event
	//var eventtwo Event

	BeforeEach(func() {

		cfg = &Config{
			OutputDirectory:      os.TempDir(),
			DataServiceTokenFile: "dataServiceTokenFile",
			DataServiceCertFile:  "dataServiceCertFile",
			Namespace:            "namespace",
		}

		ctx, cancel = context.WithCancel(context.Background())

		eventone = Event{"one", []byte("{\"event\":\"one\"}")}
		//eventtwo = Event{"two", []byte("{\"event\":\"two\"}")}

		// Pass ComponentConfig that sets the mock server where we can read the request
		engine = NewEventEngine(ctx, logf.Log, cfg)

		err = engine.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		By("stopping the engine")
		cancel()
	})

	It("should send event after reaching the configured limit", func() {
		for i := 0; i < 60; i++ {
			engine.EventChan <- eventone
		}

		// read the request
	})
})
