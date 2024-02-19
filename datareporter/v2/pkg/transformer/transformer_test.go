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

const (
	goodJson = `{
		"anonymousId": "f5d26e39-d421-4367-b623-4c8a7bbf6cbf",
		"event": "Account Contractual Usage",
		"properties": {
			"accountId": "US_123456",
			"accountIdType": "countryCode_ICN",
			"accountPlan": "STL",
			"chargePlanType": 2,
			"daysToExpiration": 9999,
			"environment": "PRODUCTION",
			"eventId": "1234567890123",
			"frequency": "Hourly",
			"hyperscalerChannel": "ibm",
			"hyperscalerFormat": "saas",
			"hyperscalerProvider": "aws",
			"instanceGuid": "123456789012-testcorp",
			"instanceName": "",
			"productCode": "WW1234",
			"productCodeType": "WWPC",
			"productId": "1234-M12",
			"productTitle": "Test Application Suite",
			"productVersion": "1.23.4",
			"quantity": 123,
			"salesOrderNumber": "1234X",
			"source": "123456789abc",
			"subscriptionId": "1234",
			"tenantId": "1234",
			"unit": "Points",
			"unitDescription": "Points Description",
			"unitMetadata": {
				"data": {
					"assist-Users": 0,
					"core-Install": 100,
					"health-AuthorizedUsers": 0,
					"health-ConcurrentUsers": 0,
					"health-MAS-Internal-Scores": 0,
					"hputilities-MAS-AIO-Models": 0,
					"hputilities-MAS-AIO-Scores": 0,
					"hputilities-MAS-External-Scores": 0,
					"manage-Database-replicas": 0,
					"manage-Install": 150,
					"manage-MAS-Base": 0,
					"manage-MAS-Base-Authorized": 0,
					"manage-MAS-Limited": 0,
					"manage-MAS-Limited-Authorized": 0,
					"manage-MAS-Premium": 0,
					"manage-MAS-Premium-Authorized": 5,
					"monitor-IOPoints": 0,
					"monitor-KPIPoints": 0,
					"predict-MAS-Models-Trained": 0,
					"predict-MAS-Predictions-Count": 0,
					"visualinspection-MAS-Images-Inferred": 0,
					"visualinspection-MAS-Models-Trained": 0,
					"visualinspection-MAS-Videos-Inferred": 0
				},
				"version": "1"
			},
			"UT30": "12BH3"
		},
		"timestamp": "2024-01-18T11:00:11.159442+00:00",
		"type": "track",
		"userId": "ABCid-123456789abc-owner",
		"writeKey": "write-key"
	}`

	goodResult = `{"instances":[{"metricUsage":[{"quantity":123,"metricId":"Points"}],"instanceId":"123456789abc","startTime":"2024-01-18T11:00:11.159442+00:00","endTime":"2024-01-18T11:00:11.159442+00:00"}],"subscriptionId":"1234-M12"}`
)

var _ = Describe("Transformer", func() {

	goodJsonBytes := []byte(goodJson)
	goodResultBytes := []byte(goodResult)

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

	Describe("Transformer Transform", func() {
		Context("with valid json", func() {
			It("should transform", func() {
				t, err := transformer.NewTransformer("kazaam", `[{"operation": "shift", "spec": {"instances[0].endTime": "timestamp", "instances[0].instanceId": "properties.source", "instances[0].metricUsage[0].metricId": "properties.unit", "instances[0].metricUsage[0].quantity": "properties.quantity", "instances[0].startTime": "timestamp", "subscriptionId": "properties.productId"}}]`)
				Expect(err).NotTo(HaveOccurred())
				result, err := t.Transform(goodJsonBytes)
				Expect(err).NotTo(HaveOccurred())
				Expect(checkJSONBytesEqual(result, goodResultBytes)).To(BeTrue())
			})
		})
	})

})
