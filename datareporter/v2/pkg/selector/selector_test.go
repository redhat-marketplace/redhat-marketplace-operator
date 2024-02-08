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

		badin = `{unclosed`

		// Match: true
		jp1 = `$.properties.productId`
		jp2 = `$[?($.properties.source != null)]`
		jp3 = `$[?($.properties.unit == "AppPoints")]`
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
