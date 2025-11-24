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

package processors

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/mocks"
	pkgtypes "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/api/marketplace/v1beta1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/api/marketplace/v1beta1"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("status_processor", func() {

	var (
		ctx                   context.Context
		mockClient            *mocks.Client
		mockSubResourceWriter *mocks.SubResourceWriter
		sut                   *StatusProcessor

		obj1 cache.Delta
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = &mocks.Client{}
		mockSubResourceWriter = &mocks.SubResourceWriter{}
		obj1 = cache.Delta{Type: cache.Added, Object: &pkgtypes.MeterDefinitionEnhancedObject{
			Object: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podname",
					Namespace: "ns1",
					UID:       types.UID("pod1"),
				},
			},
			MeterDefinitions: []*marketplacev1beta1.MeterDefinition{
				&v1beta1.MeterDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name",
						Namespace: "ns1",
						UID:       types.UID("foo"),
					},
				},
			},
		}}

		sut = &StatusProcessor{
			kubeClient: mockClient,
			log:        logr.Discard(),
			resources:  make(map[client.ObjectKey]*updateableStatus),
		}
	})

	AfterEach(func() {
		mock.AssertExpectationsForObjects(GinkgoT(), mockClient)
	})

	It("should process status changes", func() {
		name := types.NamespacedName{Namespace: "ns1", Name: "name"}

		mockClient.On("Status").Return(mockSubResourceWriter)
		mockClient.On("Get", mock.Anything, name, mock.Anything).Return(nil)
		mockSubResourceWriter.On("Update", mock.Anything, mock.Anything).Return(nil)

		err := sut.Process(ctx, obj1)
		Expect(err).To(Succeed())
		Expect(sut.resources).To(HaveLen(1))
		Expect(sut.resources[name].needsUpdate).To(BeTrue())
		sut.flush(ctx)
		Expect(sut.resources[name].needsUpdate).To(BeFalse())
	})
})
