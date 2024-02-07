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
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("EngineTest", func() {
	var err error

	var cc *v1alpha1.ComponentConfig
	var config *Config

	var ctx context.Context
	var cancel context.CancelFunc

	//var engine *EventEngine
	var ps *ProcessorSender

	//var eventone Event

	BeforeEach(func() {

		cc = v1alpha1.NewComponentConfig()
		config = &Config{
			OutputDirectory:      os.TempDir(),
			DataServiceTokenFile: "/dev/null",
			DataServiceCertFile:  "/dev/null",
			Namespace:            namespace,
			MaxFlushTimeout:      cc.MaxFlushTimeout,
		}

		ctx, cancel = context.WithCancel(context.Background())

		//eventone = Event{"one", json.RawMessage(`{"event":"one"}`)}

		ps = &ProcessorSender{
			log:           logf.Log,
			digestersSize: 1,
			config:        config,
			client:        k8sClient,
		}
		ready := make(chan bool)

		err = ps.Start(ctx, ready)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		By("stopping the processor")
		cancel()
	})

	/* Processor will not start without active DataService, need to consider mock

	It("should send event after reaching the configured limit", func() {
		// Send 1 less than limit
		for i := 0; i < cc.MaxEventEntries-1; i++ {
			utils.PrettyPrint("send one")
			ps.EventChan <- eventone
		}
		Eventually(func() []string {
			utils.PrettyPrint(engine.eventAccumulator.GetKeys())
			return ps.eventAccumulator.GetKeys()
		}, timeout, interval).Should(HaveLen(cc.MaxEventEntries - 1))

		// Send 1 more, should Flush
		ps.EventChan <- eventone
		Eventually(func() []string {
			utils.PrettyPrint(engine.eventAccumulator.GetKeys())
			return ps.eventAccumulator.GetKeys()
		}, timeout, interval).Should(HaveLen(0))
	})
	*/

})
