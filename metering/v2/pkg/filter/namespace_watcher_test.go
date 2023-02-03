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
	"reflect"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("namespace_watcher", func() {
	var sut *NamespaceWatcher
	var item1, item2, item3, item4, item5, item6 client.ObjectKey
	var alertChan chan interface{}
	var ctx context.Context
	var cancel context.CancelFunc
	var alertCount int

	var types1, types2, types3, types4, types5, types6 map[string][]reflect.Type

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		sut = ProvideNamespaceWatcher(logr.Discard())
		item1 = client.ObjectKey{Namespace: "foo", Name: "pod1"}
		item2 = client.ObjectKey{Namespace: "foo2", Name: "pod1"}
		item3 = client.ObjectKey{Namespace: "foo", Name: "pod2"}
		item4 = client.ObjectKey{Namespace: "foo3", Name: "pod"}
		item5 = client.ObjectKey{Namespace: "foo4", Name: "pod"}
		item6 = client.ObjectKey{Namespace: "openshift-operators", Name: "pod"}
		types1 = map[string][]reflect.Type{
			"foo": {reflect.TypeOf(&corev1.Pod{})},
		}
		types2 = map[string][]reflect.Type{
			"foo2": {reflect.TypeOf(&corev1.Service{})},
		}
		types3 = map[string][]reflect.Type{
			"foo": {reflect.TypeOf(&corev1.Service{})},
		}
		types4 = map[string][]reflect.Type{
			"foo3": {reflect.TypeOf(&corev1.Pod{})},
		}
		types5 = map[string][]reflect.Type{
			"foo4": {reflect.TypeOf(&corev1.Pod{})},
		}
		types6 = map[string][]reflect.Type{
			"": {reflect.TypeOf(&corev1.Pod{})},
		}
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
		sut.AddNamespace(item1, types1)
		sut.AddNamespace(item2, types2)
		sut.AddNamespace(item3, types3)
	}, NodeTimeout(time.Duration.Seconds(60)))

	AfterEach(func() {
		cancel()
	})

	It("should keep track of namespaces based on objects passed to it", func() {
		Expect(sut.Get()).To(
			And(HaveKeyWithValue("foo", ConsistOf(reflect.TypeOf(&corev1.Pod{}), reflect.TypeOf(&corev1.Service{}))),
				HaveKeyWithValue("foo2", []reflect.Type{reflect.TypeOf(&corev1.Service{})}))) // still have item1 so "foo" should still be there
		Eventually(func() int {
			return alertCount
		}, 5).Should(Equal(3))
	})

	It("should remove namespaces based on objects passed to it", func() {
		Expect(sut.Get()).To(And(HaveKey("foo"), HaveKey("foo2")))
		sut.RemoveNamespace(item3)
		Expect(sut.Get()).To(
			And(HaveKeyWithValue("foo", []reflect.Type{reflect.TypeOf(&corev1.Pod{})}),
				HaveKeyWithValue("foo2", []reflect.Type{reflect.TypeOf(&corev1.Service{})}))) // still have item1 so "foo" should still be there
		sut.RemoveNamespace(item1)
		Expect(sut.Get()).To(HaveKey("foo2"))
		Eventually(func() int {
			return alertCount
		}, 5).Should(Equal(5))
	})

	It("should handle multiple alerts", func() {
		newAlertChan := make(chan interface{}, 10)
		defer close(newAlertChan)
		sut.RegisterWatch(newAlertChan)
		sut.AddNamespace(item1, types1)
		sut.AddNamespace(item2, types2)
		sut.AddNamespace(item3, types3)
		Eventually(newAlertChan).ShouldNot(Receive())
		sut.AddNamespace(item4, types4)
		Eventually(newAlertChan).Should(Receive(Equal(true)))
		sut.AddNamespace(item5, types5)

		Expect(sut.Get()).To(And(HaveKey("foo3"), HaveKey("foo4")))

		Eventually(newAlertChan).Should(Receive(Equal(true)))
		sut.RemoveNamespace(item5)
		Eventually(newAlertChan).Should(Receive(Equal(true)))
		sut.RemoveNamespace(item1)
		Eventually(newAlertChan).ShouldNot(Receive())

		Expect(sut.Get()).ToNot(And(HaveKey("foo3"), HaveKey("foo4")))
	})

	It("should handle all-namespaces", func() {
		By("should handle all namespaces pod")
		sut.AddNamespace(item1, types1)
		sut.AddNamespace(item2, types2)
		sut.AddNamespace(item3, types3)
		sut.addNamespace(item6, types6)
		Expect(sut.Get()).To(
			And(HaveLen(3),
				HaveKeyWithValue("", []reflect.Type{reflect.TypeOf(&corev1.Pod{})}),
				HaveKeyWithValue("foo", []reflect.Type{reflect.TypeOf(&corev1.Service{})}),
				HaveKeyWithValue("foo2", []reflect.Type{reflect.TypeOf(&corev1.Service{})})))

		By("should handle new namespaces for pods with no new change")
		Expect(sut.addNamespace(item5, types5)).To(BeFalse())
		Expect(sut.Get()).To(
			And(HaveLen(3),
				HaveKeyWithValue("", []reflect.Type{reflect.TypeOf(&corev1.Pod{})}),
				HaveKeyWithValue("foo", []reflect.Type{reflect.TypeOf(&corev1.Service{})}),
				HaveKeyWithValue("foo2", []reflect.Type{reflect.TypeOf(&corev1.Service{})})))

		By("removing the all namespaces we get them all")
		Expect(sut.removeNamespace(item6)).To(BeTrue())
		Expect(sut.namespaces).To(HaveLen(4))
		Expect(sut.Get()).To(
			And(HaveLen(3),
				HaveKeyWithValue("foo", ConsistOf(reflect.TypeOf(&corev1.Pod{}), reflect.TypeOf(&corev1.Service{}))),
				HaveKeyWithValue("foo4", []reflect.Type{reflect.TypeOf(&corev1.Pod{})}),
				HaveKeyWithValue("foo2", []reflect.Type{reflect.TypeOf(&corev1.Service{})})))
	})

	It("should merge type slices", func() {
		var a, b []reflect.Type
		var (
			rt1, rt2, rt3 reflect.Type = reflect.TypeOf(&corev1.Service{}), reflect.TypeOf(&corev1.Volume{}), reflect.TypeOf(&corev1.Pod{})
		)
		a = []reflect.Type{rt1, rt2}
		b = []reflect.Type{rt3}
		Expect(mergeTypeSlice(a, b)).To(And(ConsistOf(rt1, rt2, rt3), HaveLen(3)))
		a = []reflect.Type{rt1, rt2}
		b = []reflect.Type{rt2}
		Expect(mergeTypeSlice(a, b)).To(And(ConsistOf(rt1, rt2), HaveLen(2)))
		a = []reflect.Type{rt3}
		b = []reflect.Type{rt1, rt2}
		Expect(mergeTypeSlice(a, b)).To(And(ConsistOf(rt1, rt2, rt3), HaveLen(3)))
		a = []reflect.Type{rt1}
		b = []reflect.Type{rt1}
		Expect(mergeTypeSlice(a, b)).To(And(ConsistOf(rt1), HaveLen(1)))
	})

	It("should merge map string type slices", func() {
		var a, b map[string][]reflect.Type
		var (
			rt1, rt2, rt3 reflect.Type = reflect.TypeOf(&corev1.Service{}), reflect.TypeOf(&corev1.Volume{}), reflect.TypeOf(&corev1.Pod{})
		)
		a = map[string][]reflect.Type{
			"a": {rt1, rt2},
			"b": {rt1, rt3},
		}
		b = map[string][]reflect.Type{
			"a": {rt1, rt3},
			"c": {rt1},
		}
		Expect(mergeMap(a, b)).To(
			And(HaveKeyWithValue("a", ConsistOf(rt1, rt2, rt3)),
				HaveKeyWithValue("b", ConsistOf(rt1, rt3)),
				HaveKeyWithValue("c", ConsistOf(rt1)),
			))

		a = map[string][]reflect.Type{
			"a": {rt1, rt2},
			"b": {rt1, rt3},
		}
		b = map[string][]reflect.Type{
			"a": {rt1, rt3},
			"b": {rt1},
		}
		Expect(mergeMap(a, b)).To(
			And(HaveKeyWithValue("a", ConsistOf(rt1, rt2, rt3)),
				HaveKeyWithValue("b", ConsistOf(rt1, rt3)),
			))
	})
})
