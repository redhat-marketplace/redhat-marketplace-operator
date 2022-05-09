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

package authchecker

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/redhat-marketplace-operator/authchecker/v2/mocks"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("authchecker", func() {
	var (
		ctx        context.Context
		mockClient *mocks.NamespaceableResourceInterface
		sut        *AuthChecker
	)

	BeforeEach(func() {
		ctx = context.TODO()
		mockClient = &mocks.NamespaceableResourceInterface{}

		sut = &AuthChecker{
			Logger: logr.Discard(),
			Client: mockClient,
			tokenReview: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
		}
	})

	It("should check token", func() {
		mockClient.On("Create", ctx, mock.Anything, mock.Anything).Times(1).Return(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"status": map[string]interface{}{
					"authenticated": true,
				},
			},
		}, nil)

		err := sut.CheckToken(ctx)
		Expect(err).To(Succeed())
		Expect(sut.Check(nil)).To(Succeed())

		mock.AssertExpectationsForObjects(GinkgoT(), mockClient)
	})

	It("should error if token is invalid", func() {
		mockClient.On("Create", ctx, mock.Anything, mock.Anything).Times(1).Return(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"status": map[string]interface{}{
					"authenticated": false,
				},
			},
		}, nil)

		err := sut.CheckToken(ctx)
		Expect(err).To(HaveOccurred())
		Expect(sut.Check(nil)).To(Equal(err))

		mock.AssertExpectationsForObjects(GinkgoT(), mockClient)
	})

	It("should error if client errors", func() {
		clientErr := fmt.Errorf("failed")
		mockClient.On("Create", ctx, mock.Anything, mock.Anything).Times(1).Return(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"status": map[string]interface{}{
					"authenticated": false,
				},
			},
		}, clientErr)

		err := sut.CheckToken(ctx)

		Expect(err).To(HaveOccurred())
		Expect(sut.Check(nil)).To(Equal(err))
		Expect(err).To(Equal(clientErr))

		mock.AssertExpectationsForObjects(GinkgoT(), mockClient)
	})
})
