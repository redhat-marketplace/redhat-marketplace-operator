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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("filter", func() {
	var (
		filters FilterRuntimeObjects
		filter  *mockFilter
	)

	BeforeEach(func() {
		filter = &mockFilter{}
		filters = FilterRuntimeObjects{
			filter,
			filter,
			filter,
		}
	})

	AfterEach(func() {
		mock.AssertExpectationsForObjects(GinkgoT(), filter)
	})

	It("should short circuit on false", func() {
		filter.On("Filter", nil).Return(false, nil).Times(1)
		pass, i, err := filters.Test(nil)
		Expect(pass).To(BeFalse())
		Expect(i).To(Equal(0))
		Expect(err).To(Succeed())
	})

	It("should exhaustively search", func() {
		filter.On("Filter", nil).Return(true, nil).Once()
		filter.On("Filter", nil).Return(false, nil).Once()

		pass, i, err := filters.Test(nil)
		Expect(pass).To(BeFalse())
		Expect(i).To(Equal(1))
		Expect(err).To(Succeed())
	})

	It("should pass if all are successful", func() {
		filter.On("Filter", nil).Return(true, nil).Times(3)
		pass, i, err := filters.Test(nil)
		Expect(pass).To(BeTrue())
		Expect(i).To(Equal(-1))
		Expect(err).To(Succeed())
	})
})

var _ = Describe("label_filter", func() {
	var obj client.Object

	BeforeEach(func() {
		obj = &corev1.Pod{}
		obj.SetLabels(map[string]string{
			"app.kubernetes.io/name": "a",
		})
	})

	It("should match labels", func() {
		selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/name": "a",
			},
		})
		Expect(err).To(Succeed())
		sut := &WorkloadLabelFilter{
			labelSelector: selector,
		}

		Expect(sut.Filter(obj)).To(BeTrue())
	})
})

type mockFilter struct {
	mock.Mock
}

func (f *mockFilter) Filter(obj interface{}) (bool, error) {
	args := f.Called(obj)
	return args.Get(0).(bool), args.Error(1)
}
