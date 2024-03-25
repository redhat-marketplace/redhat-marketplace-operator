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

package selector_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/events"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/selector"
)

var _ = Describe("Selector", func() {
	var in, badin string

	var jp1, jp2, jp3, jp4, jp5, jp6, jp7 string
	//var jps1, jps3 selector.JsonPathsSelector

	var UsersSelector selector.UsersSelector
	var emptyUsersSelector selector.UsersSelector

	BeforeEach(func() {
		in = `{
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

		badin = `{unclosed`

		// Match: true
		jp1 = `$.properties.productId`
		jp2 = `$[?($.properties.source != null)]`
		jp3 = `$[?($.properties.unit == "Points")]`
		jp4 = `$[?($.properties.quantity >= 0)]`
		jp5 = `$[?($.timestamp != null)]`

		// Bad Syntax Parse err
		jp6 = `[?(`

		// Match: false
		jp7 = `$[?($.properties.unit == "Nothing")]`

		UsersSelector = []string{"user1", "user2", "user3", "user4", "user5"}

	})

	Describe("Matching JsonPathsSelector Requirements", func() {
		Context("with 5 jsonpath expressions", func() {
			It("should match 5 jsonpath expressions", func() {
				jps, _ := selector.NewJsonPathsSelector([]string{jp1, jp2, jp3, jp4, jp5})
				Expect(jps.Matches(in)).To(BeTrue())
			})
		})

		Context("with no jsonpath expressions", func() {
			It("should match with no expressions", func() {
				jps, _ := selector.NewJsonPathsSelector([]string{})
				Expect(jps.Matches(in)).To(BeTrue())
			})
		})

		Context("with a bad jsonpath expression", func() {
			It("should parse err", func() {
				Expect(selector.NewJsonPathsSelector([]string{jp1, jp2, jp3, jp4, jp5, jp6})).Error()
			})
		})

		Context("with a mismatch", func() {
			It("should not match all jsonpath expressions", func() {
				jps, _ := selector.NewJsonPathsSelector([]string{jp1, jp2, jp4, jp5, jp7})
				Expect(jps.Matches(in)).To(BeFalse())
			})
		})

		Context("with malformed json input", func() {
			It("should not match any jsonpath expressions", func() {
				jps, _ := selector.NewJsonPathsSelector([]string{jp1})
				Expect(jps.Matches(badin)).To(BeFalse())
			})
		})

	})

	Describe("Matching UsersSelector Requirements", func() {
		Context("with user1", func() {
			It("should match 5 jsonpath expressions", func() {
				Expect(UsersSelector.Matches("user1")).To(BeTrue())
			})
		})

		Context("with user6", func() {
			It("should not match user6", func() {
				Expect(UsersSelector.Matches("user6")).To(BeFalse())
			})
		})

		Context("with empty UsersSelector", func() {
			It("should match any user", func() {
				Expect(emptyUsersSelector.Matches("user7")).To(BeTrue())
			})
		})

	})

	Describe("Matching DataFilterSelector Requirements", func() {
		Context("with a bad jsonpath expression", func() {
			It("should parse err", func() {
				Expect(selector.NewDataFilterSelector(
					v1alpha1.Selector{MatchExpressions: []string{jp1, jp2, jp3, jp4, jp5, jp6}, MatchUsers: UsersSelector})).Error()
			})
		})

		Context("with match", func() {
			It("should match jsonpath and user", func() {
				sel, _ := selector.NewDataFilterSelector(
					v1alpha1.Selector{MatchExpressions: []string{jp1, jp2, jp3, jp4, jp5}, MatchUsers: UsersSelector})
				Expect(sel.Matches(events.Event{RawMessage: []byte(in), User: "user1"})).To(BeTrue())
			})
		})

		Context("with no expressions or users", func() {
			It("should match by default", func() {
				sel, _ := selector.NewDataFilterSelector(
					v1alpha1.Selector{MatchExpressions: []string{}, MatchUsers: []string{}})
				Expect(sel.Matches(events.Event{RawMessage: []byte(in), User: "user1"})).To(BeTrue())
			})
		})

		Context("with no match event", func() {
			It("should not match jsonpath", func() {
				sel, _ := selector.NewDataFilterSelector(
					v1alpha1.Selector{MatchExpressions: []string{jp1, jp2, jp3, jp4, jp5, jp7}, MatchUsers: UsersSelector})
				Expect(sel.Matches(events.Event{RawMessage: []byte(in), User: "user1"})).To(BeFalse())
			})
		})

		Context("with no matched user", func() {
			It("should not match user", func() {
				sel, _ := selector.NewDataFilterSelector(
					v1alpha1.Selector{MatchExpressions: []string{jp1, jp2, jp3, jp4, jp5}, MatchUsers: UsersSelector})
				Expect(sel.Matches(events.Event{RawMessage: []byte(in), User: "user6"})).To(BeFalse())
			})
		})

		Context("with no match bad input", func() {
			It("should not match bad json", func() {
				sel, _ := selector.NewDataFilterSelector(
					v1alpha1.Selector{MatchExpressions: []string{jp1, jp2, jp3, jp4, jp5}, MatchUsers: UsersSelector})
				Expect(sel.Matches(events.Event{RawMessage: []byte(badin), User: "user1"})).To(BeFalse())
			})
		})

	})

})
