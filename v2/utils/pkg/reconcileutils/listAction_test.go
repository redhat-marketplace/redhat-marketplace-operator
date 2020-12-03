// Copyright 2020 IBM Corp.
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

package reconcileutils

import (
	"context"

	"emperror.dev/errors"
	"github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/scheme"
	"github.com/golang/mock/gomock"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/test/mock/mock_client"
	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ListAction", func() {
	var (
		ctrl        *gomock.Controller
		client      *mock_client.MockClient
		podList     *corev1.PodList
		cc          ClientCommandRunner
		ctx         context.Context
		expectedErr error
	)

	BeforeEach(func() {
		logger := logf.Log.WithName("CreateAction")
		ctrl = gomock.NewController(GinkgoT())
		client = mock_client.NewMockClient(ctrl)
		podList = &corev1.PodList{}
		cc = NewClientCommand(client, scheme.Scheme, logger)
		ctx = context.TODO()
		expectedErr = errors.NewPlain("mock fail")
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should return a list", func() {
		client.EXPECT().List(ctx, podList).Return(nil).Times(1)

		result, err := cc.Do(ctx, ListAction(podList))

		Expect(err).To(Succeed())
		Expect(result).ToNot(BeNil())
		Expect(result.Status).To(Equal(Continue))
	})

	It("should handle error", func() {
		client.EXPECT().List(ctx, podList).Return(expectedErr).Times(1)

		result, err := cc.Do(ctx, ListAction(podList))

		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(expectedErr))
		Expect(result).ToNot(BeNil())
		Expect(result.Status).To(Equal(Error))
	})
})
