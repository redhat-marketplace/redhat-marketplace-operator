package engine

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/filter"
	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("NamespacedCacheLister", func() {
	var sut *NamespacedCachedListers
	var listWatcher1, listWatcher2 *RunAndStopMock
	var item1, item2, item3, item4, item5 client.ObjectKey
	var nsWatcher *filter.NamespaceWatcher
	var ctx context.Context
	var cancel context.CancelFunc

	BeforeEach(func() {
		listWatcher1 = &RunAndStopMock{}
		listWatcher2 = &RunAndStopMock{}
		nsWatcher = filter.ProvideNamespaceWatcher(logr.Discard())

		item1 = client.ObjectKey{Namespace: "foo", Name: "pod1"}
		item2 = client.ObjectKey{Namespace: "foo2", Name: "pod1"}
		item3 = client.ObjectKey{Namespace: "foo", Name: "pod2"}
		item4 = client.ObjectKey{Namespace: "foo3", Name: "pod"}
		item5 = client.ObjectKey{Namespace: "foo4", Name: "pod"}

		ctx, cancel = context.WithCancel(context.Background())

		sut = ProvideNamespacedCacheListers(nsWatcher, logr.Discard(),
			ListWatchers{
				"1": listWatcher1.Get,
				"2": listWatcher2.Get,
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
		listWatcher1.On("Get", "foo3").Return(listWatcher1)
		listWatcher1.On("Get", "foo4").Return(listWatcher1)
		listWatcher2.On("Get", "foo").Return(listWatcher2)
		listWatcher2.On("Get", "foo2").Return(listWatcher2)
		listWatcher2.On("Get", "foo3").Return(listWatcher2)
		listWatcher2.On("Get", "foo4").Return(listWatcher2)
		listWatcher1.On("Start", mock.AnythingOfType(argType)).Return(nil).Times(4)
		listWatcher2.On("Start", mock.AnythingOfType(argType)).Return(nil).Times(4)

		nsWatcher.AddNamespace(item1)
		nsWatcher.AddNamespace(item2)
		nsWatcher.AddNamespace(item3)
		nsWatcher.AddNamespace(item4)
		nsWatcher.AddNamespace(item5)

		Eventually(func() int {
			return len(listWatcher1.Calls) + len(listWatcher2.Calls)
		}, 5).Should(Equal(16))

		mock.AssertExpectationsForObjects(GinkgoT(), listWatcher1, listWatcher2)
	}, 30)

	It("should add and remove watchers by namespace (2)", func() {
		argType := "*context.cancelCtx"

		listWatcher1.On("Get", "foo").Return(listWatcher1)
		listWatcher1.On("Get", "foo2").Return(listWatcher1)
		listWatcher2.On("Get", "foo").Return(listWatcher2)
		listWatcher2.On("Get", "foo2").Return(listWatcher2)
		listWatcher1.On("Start", mock.AnythingOfType(argType)).Return(nil).Times(2)
		listWatcher2.On("Start", mock.AnythingOfType(argType)).Return(nil).Times(2)
		listWatcher1.On("Stop").Times(1)
		listWatcher2.On("Stop").Times(1)

		nsWatcher.AddNamespace(item1)
		nsWatcher.AddNamespace(item2)
		nsWatcher.AddNamespace(item3)

		Eventually(func() int {
			return len(listWatcher1.Calls) + len(listWatcher2.Calls)
		}, 5).Should(Equal(8))

		nsWatcher.RemoveNamespace(item2)

		Eventually(func() int {
			return len(listWatcher1.Calls) + len(listWatcher2.Calls)
		}, 5).Should(Equal(10))

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
