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

package engine

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/filter"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("NamespacedCacheLister", func() {
	var sut *NamespacedCachedListers
	var listWatcher1, listWatcher2 *RunAndStopMock
	var item1, item2, item3, item4, item5 client.ObjectKey
	var types1, types2, types3, types4, types5 map[string][]reflect.Type
	var nsWatcher *filter.NamespaceWatcher
	var ctx context.Context
	var cancel context.CancelFunc

	BeforeEach(func() {
		listWatcher1 = &RunAndStopMock{}
		listWatcher2 = &RunAndStopMock{}
		nsWatcher = filter.ProvideNamespaceWatcher(logr.Discard())

		item1 = client.ObjectKey{Namespace: "foo", Name: "pod1"}
		types1 = map[string][]reflect.Type{"foo": {reflect.TypeOf(&corev1.Pod{})}}
		item2 = client.ObjectKey{Namespace: "foo2", Name: "pod1"}
		types2 = map[string][]reflect.Type{"foo2": {reflect.TypeOf(&corev1.Pod{})}}
		item3 = client.ObjectKey{Namespace: "foo", Name: "pod2"}
		types3 = map[string][]reflect.Type{"foo": {reflect.TypeOf(&corev1.Pod{})}}
		item4 = client.ObjectKey{Namespace: "foo3", Name: "service"}
		types4 = map[string][]reflect.Type{"foo3": {reflect.TypeOf(&corev1.Service{})}}
		item5 = client.ObjectKey{Namespace: "foo4", Name: "service"}
		types5 = map[string][]reflect.Type{"foo4": {reflect.TypeOf(&corev1.Service{})}}

		ctx, cancel = context.WithCancel(context.Background())

		sut = ProvideNamespacedCacheListers(nsWatcher, logr.Discard(),
			ListWatchers{
				reflect.TypeOf(&corev1.Pod{}):     listWatcher1.Get,
				reflect.TypeOf(&corev1.Service{}): listWatcher2.Get,
			})

		By("Starting namedcachelister")
		Expect(sut.Start(ctx)).To(Succeed())
		By("Started namedcachelister")
	}, 10)

	AfterEach(func() {
		cancel()
	})

	It("should start list watchers for each namespace (4)", func() {
		argType := "*context.cancelCtx"

		listWatcher1.On("Get", "foo").Return(listWatcher1)
		listWatcher1.On("Get", "foo2").Return(listWatcher1)
		listWatcher2.On("Get", "foo3").Return(listWatcher2)
		listWatcher2.On("Get", "foo4").Return(listWatcher2)
		listWatcher1.On("Start", mock.AnythingOfType(argType)).Return(nil).Times(2)
		listWatcher2.On("Start", mock.AnythingOfType(argType)).Return(nil).Times(2)

		nsWatcher.AddNamespace(item1, types1)
		nsWatcher.AddNamespace(item2, types2)
		nsWatcher.AddNamespace(item3, types3)
		nsWatcher.AddNamespace(item4, types4)
		nsWatcher.AddNamespace(item5, types5)

		Eventually(func() int {
			return len(listWatcher1.Calls) + len(listWatcher2.Calls)
		}, 5).Should(Equal(8))

		mock.AssertExpectationsForObjects(GinkgoT(), listWatcher1, listWatcher2)
	}, 30)

	It("should add and remove watchers by namespace (2)", func() {
		argType := "*context.cancelCtx"

		listWatcher1.On("Get", "foo").Return(listWatcher1)
		listWatcher1.On("Get", "foo2").Return(listWatcher1)
		listWatcher1.On("Start", mock.AnythingOfType(argType)).Return(nil).Times(2)
		listWatcher1.On("Stop").Times(1)

		nsWatcher.AddNamespace(item1, types1)
		nsWatcher.AddNamespace(item2, types2)
		nsWatcher.AddNamespace(item3, types3)

		Eventually(func() int {
			return len(listWatcher1.Calls)
		}, 5).Should(Equal(4))

		nsWatcher.RemoveNamespace(item2)

		Eventually(func() int {
			return len(listWatcher1.Calls)
		}, 5).Should(Equal(5))

		Expect(listWatcher2.Calls).Should(HaveLen(0))
		mock.AssertExpectationsForObjects(GinkgoT(), listWatcher1, listWatcher2)
	}, 30)

})

type RunAndStopMock struct {
	mock.Mock
}

func (s *RunAndStopMock) Start(ctx context.Context) error {
	args := s.Called(ctx)
	return args.Error(0)
}

func (s *RunAndStopMock) Stop() {
	s.Called()
}

func (g *RunAndStopMock) Get(ns string) RunAndStop {
	args := g.Called(ns)
	return args.Get(0).(RunAndStop)
}
