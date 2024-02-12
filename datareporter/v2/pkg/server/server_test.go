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

package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/server"
)

var _ = Describe("Selector", func() {

	/*
		in := `{
			"anonymousId": "22d09868-c4dd-4e22-9dcc-fb3dc3670592",
			"context": {
				"library": {
					"name": "unknown",
					"version": "unknown"
				},
				"protocols": {
					"sourceId": "GDbmc9ffEx"
				}
			},
			"event": "Account Contractual Usage",
			"integrations": {
			},
			"messageId": "api-2FzZT63MioGiVg2ocBLkWgfYfaG",
			"originalTimestamp": "2022-10-11T14:00:10.535087+00:00",
			"properties": {
				"accountId": "U***_I***_I***",
				"accountIdType": "countryCode_ICN",
				"accountPlan": "STL",
				"chargePlanType": 2,
				"daysToExpiration": 95,
				"environment": "PRODUCTION",
				"frequency": "Hourly",
				"productId": "5737-M66",
				"productTitle": "Maximo Application Suite",
				"productVersion": "8.8.1",
				"quantity": 450,
				"quantityEntitled": 105,
				"salesOrderNumber": "None",
				"source": "10005a141d2b",
				"unit": "AppPoints",
				"unitDescription": "Account level AppPoint usage",
				"unitMetadata": {
					"data": {
						"concurrent": 0,
						"denied": 0,
						"reserved": 450
					},
					"version": "1"
				}
			},
			"receivedAt": "2022-10-11T14:01:16.860Z",
			"timestamp": "2022-10-11T14:00:10.535Z",
			"type": "track",
			"userId": "M***-1***-o***"
			}`
	*/

	BeforeEach(func() {

	})

	Describe("Testing Event Handler", func() {
		Context("with valid json", func() {
			It("should receive 200, should receive transformed data", func() {
				req := httptest.NewRequest("POST", "https://localhost/v1/event", strings.NewReader(testData))
				req.Header.Add("x-remote-user", "testuser")
				w := httptest.NewRecorder()
				server.EventHandler(eventEngine, eventConfig, dataFilters, w, req)
				resp := w.Result()
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			})
		})

		Context("with invalid json", func() {
			It("should 400", func() {
				req := httptest.NewRequest("POST", "https://localhost/v1/event", strings.NewReader(testDataBad))
				req.Header.Add("x-remote-user", "testuser")
				w := httptest.NewRecorder()
				server.EventHandler(eventEngine, eventConfig, dataFilters, w, req)
				resp := w.Result()
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		Context("with no user header", func() {
			It("should 400", func() {
				req := httptest.NewRequest("POST", "https://localhost/v1/event", strings.NewReader(testData))
				w := httptest.NewRecorder()
				server.EventHandler(eventEngine, eventConfig, dataFilters, w, req)
				resp := w.Result()
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})
	})

	Describe("Testing Status Handler", func() {
		Context("GET /v1/status", func() {
			It("should 200", func() {
				req := httptest.NewRequest("GET", "https://localhost/v1/status", nil)
				w := httptest.NewRecorder()
				server.StatusHandler(w, req)
				resp := w.Result()
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			})
		})
	})

})
