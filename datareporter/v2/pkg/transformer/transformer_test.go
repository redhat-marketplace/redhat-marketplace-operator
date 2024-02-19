// Copyright 2024 IBM Corp.
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

package transformer_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	transformer "github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/transformer"
)

var _ = Describe("Transformer", func() {

	Describe("Supported Transformer Requirements", func() {
		Context("with kazaam", func() {
			It("should initialize", func() {
				_, err := transformer.NewTransformer("kazaam", "")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("with unsupported one", func() {
			It("should throw error", func() {
				Expect(transformer.NewTransformer("abc", "")).Error()
			})
		})

	})

	Describe("Transformer Text Requirements", func() {
		Context("with valid one", func() {
			It("should initialize", func() {
				_, err := transformer.NewTransformer("kazaam", `[{"operation": "shift", "spec": {"instances[0].endTime": "timestamp", "instances[0].instanceId": "properties.source", "instances[0].metricUsage[0].metricId": "properties.unit", "instances[0].metricUsage[0].quantity": "properties.quantity", "instances[0].startTime": "timestamp", "subscriptionId": "properties.productId"}}]`)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("with invalid one", func() {
			It("should throw error", func() {
				Expect(transformer.NewTransformer("kazaam", `[{"operation": "INVALID", "spec": {"instances[0].endTime": "timestamp", "instances[0].instanceId": "properties.source", "instances[0].metricUsage[0].metricId": "properties.unit", "instances[0].metricUsage[0].quantity": "properties.quantity", "instances[0].startTime": "timestamp", "subscriptionId": "properties.productId"}}]`)).Error()
			})
		})

	})

})
