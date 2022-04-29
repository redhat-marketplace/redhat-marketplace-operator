// Copyright 2022 IBM Corp.
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

package filter

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("namespace_watcher", func() {
	var sut *NamespaceWatcher
	var item1, item2, item3, item4, item5 client.ObjectKey
	var alertChan chan interface{}
	var ctx context.Context
	var cancel context.CancelFunc
	var alertCount int

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		sut = ProvideNamespaceWatcher(logr.Discard())
		item1 = client.ObjectKey{Namespace: "foo", Name: "pod1"}
		item2 = client.ObjectKey{Namespace: "foo2", Name: "pod1"}
		item3 = client.ObjectKey{Namespace: "foo", Name: "pod2"}
		item4 = client.ObjectKey{Namespace: "foo3", Name: "pod"}
		item5 = client.ObjectKey{Namespace: "foo4", Name: "pod"}
		alertChan = make(chan interface{})
		alertCount = 0

		go func() {
			defer close(alertChan)
			for {
				select {
				case <-alertChan:
					alertCount = alertCount + 1
				case <-ctx.Done():
					return
				}
			}
		}()

		sut.RegisterWatch(alertChan)
		sut.AddNamespace(item1)
		sut.AddNamespace(item2)
		sut.AddNamespace(item3)
	})

	AfterEach(func() {
		cancel()
	})

	It("should keep track of namespaces based on objects passed to it", func() {
		Expect(sut.Get()).To(ContainElements("foo", "foo2"))
		Eventually(func() int {
			return alertCount
		}, 5).Should(Equal(2))
	})

	It("should remove namespaces based on objects passed to it", func() {
		Expect(sut.Get()).To(ContainElements("foo", "foo2"))
		sut.RemoveNamespace(item3)
		Expect(sut.Get()).To(ContainElements("foo", "foo2")) // still have item1 so "foo" should still be there
		sut.RemoveNamespace(item1)
		Expect(sut.Get()).To(ContainElements("foo2"))
		Eventually(func() int {
			return alertCount
		}, 5).Should(Equal(3))
	})

	It("should handle multiple alerts", func() {
		newAlertChan := make(chan interface{}, 10)
		defer close(newAlertChan)
		sut.RegisterWatch(newAlertChan)
		sut.AddNamespace(item1)
		sut.AddNamespace(item2)
		sut.AddNamespace(item3)
		Eventually(newAlertChan).ShouldNot(Receive())
		sut.AddNamespace(item4)
		Eventually(newAlertChan).Should(Receive(Equal(true)))
		sut.AddNamespace(item5)
		Eventually(newAlertChan).Should(Receive(Equal(true)))
		sut.RemoveNamespace(item5)
		Eventually(newAlertChan).Should(Receive(Equal(true)))
		sut.RemoveNamespace(item1)
		Eventually(newAlertChan).ShouldNot(Receive())
	})
})
