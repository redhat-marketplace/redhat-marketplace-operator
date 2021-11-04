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

package merge

import (
	"github.com/imdario/mergo"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type testStruct struct {
	Foo string
	V   string
}

type testStructSlice []*testStruct

func (t *testStructSlice) Append(t2 *testStruct) {
	*t = append(*t, t2)
}

type testStructWrapper struct {
	Arr []*testStruct
}

type testStructWrapper2 struct {
	Arr testStructSlice
}

var _ = Describe("merge", func() {
	Describe("ReflectMergeSliceByFieldName", func() {
		It("should merge slices", func() {
			sliceA := testStructWrapper{
				Arr: []*testStruct{
					{
						Foo: "a",
						V:   "1",
					},
				},
			}
			sliceB := testStructWrapper{
				Arr: []*testStruct{
					{
						Foo: "b",
						V:   "2",
					},
				},
			}
			sliceC := sliceB

			Expect(mergo.Merge(&sliceB,
				sliceA,
				mergo.WithTransformers(MergeSliceByFieldName{FieldName: "Foo"}))).To(Succeed())
			Expect(sliceB.Arr).To(HaveLen(2))
			Expect(sliceB.Arr).To(ContainElement(sliceC.Arr[0]))
			Expect(sliceB.Arr).To(ContainElement(sliceA.Arr[0]))
		})
		It("should merge slices", func() {
			sliceA := testStructWrapper{
				Arr: []*testStruct{
					{
						Foo: "a",
						V:   "1",
					},
				},
			}
			sliceB := testStructWrapper{
				Arr: []*testStruct{
					{
						Foo: "a",
						V:   "2",
					},
				},
			}
			sliceC := sliceB

			Expect(mergo.Merge(&sliceB,
				sliceA,
				mergo.WithTransformers(MergeSliceByFieldName{FieldName: "Foo"}))).To(Succeed())
			Expect(sliceB.Arr).To(HaveLen(1))
			Expect(sliceB.Arr).ToNot(ContainElement(sliceC.Arr[0]))
			Expect(sliceB.Arr).To(ContainElement(sliceA.Arr[0]))
		})
	})

	Describe("MergeSliceFunc", func() {
		It("it should merge fields with funcs", func() {
			sliceA := testStructWrapper2{
				Arr: testStructSlice{
					{
						Foo: "a",
						V:   "1",
					},
				},
			}
			sliceB := testStructWrapper2{
				Arr: testStructSlice{
					{
						Foo: "b",
						V:   "2",
					},
				},
			}
			sliceC := sliceB

			Expect(mergo.Merge(&sliceB,
				sliceA,
				mergo.WithTransformers(
					MergeSliceFunc{
						SliceType: testStructSlice{},
						FuncName:  "Append",
					}))).To(Succeed())
			Expect(sliceB.Arr).To(HaveLen(2))
			Expect(sliceB.Arr).To(ContainElement(sliceA.Arr[0]))
			Expect(sliceB.Arr).To(ContainElement(sliceC.Arr[0]))
		})
	})
})
