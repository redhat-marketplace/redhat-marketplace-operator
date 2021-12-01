// Copyright 2021 IBM Corp.
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

package fileserver

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("filter", func() {
	DescribeTable("should parse",
		func(test, a, op, b string, err error) {
			fs := Filters{}
			if err != nil {
				Expect(fs.UnmarshalText([]byte(test))).To(MatchError(err))
			} else {
				Expect(fs.UnmarshalText([]byte(test))).To(Succeed())
				Expect(fs).To(HaveLen(1))

				f := fs[0]
				Expect(f.Left).To(Equal(a))
				Expect(f.Operator).To(Equal(FilterOperator(op)))
				Expect(f.Right).To(Equal(b))
			}
		},
		Entry("simple1", "a==b", "a", "==", "b", nil),
		Entry("simple2", "a == b", "a", "==", "b", nil),
		Entry("simple3", "a > 10", "a", ">", "10", nil),
		Entry("simple4", `a > "foo"`, "a", ">", `"foo"`, nil),
		Entry("simple5", `a > 10.0`, "a", ">", `10.0`, nil),
		Entry("harder1", `a.C.d.e != "foo.bar"`, "a.C.d.e", "!=", `"foo.bar"`, nil),
		Entry("harder2", `! != "foo.bar"`, "a.C.d.e", "!=", `"foo.bar"`, ErrFilterBadFormat),
	)

	DescribeTable("should parse filters",
		func(test string, filters []interface{}, err error) {
			fs := Filters{}
			if err != nil {
				Expect(fs.UnmarshalText([]byte(test))).To(MatchError(err))
			} else {
				Expect(fs.UnmarshalText([]byte(test))).To(Succeed())
				Expect(fs).To(ConsistOf(filters...))
			}
		},
		Entry("filters1", "arc==b && c==d",
			[]interface{}{
				&Filter{Left: "arc", Operator: FilterEqual, Right: "b", NextFilterOperator: FilterAnd},
				&Filter{Left: "c", Operator: FilterEqual, Right: "d", NextFilterOperator: FilterBooleanOperator("")},
			}, nil),
		Entry("filters2", `a < "a" || c!=d && aa.bb <= "awe some"`,
			[]interface{}{
				&Filter{Left: "a", Operator: FilterLessThan, Right: `"a"`, NextFilterOperator: FilterOr},
				&Filter{Left: "c", Operator: FilterNotEqual, Right: "d", NextFilterOperator: FilterAnd},
				&Filter{Left: "aa.bb", Operator: FilterLessThanEqual, Right: `"awe some"`},
			}, nil),
		Entry("filtersFailure", "&& a==b && c==d", []interface{}{}, ErrFilterBadFormat),
	)
})
