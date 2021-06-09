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

package common

import (
	"sort"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkloadResource", func() {
	It("should sort alphabetically", func() {
		testData := []WorkloadResource{
			{
				ReferencedWorkloadName: "",
				NamespacedNameReference: NamespacedNameReference{
					Namespace: "b",
					Name:      "c",
				},
			},
			{
				ReferencedWorkloadName: "",
				NamespacedNameReference: NamespacedNameReference{
					Namespace: "a",
					Name:      "b",
				},
			},
		}

		data := ByAlphabetical(testData)
		sort.Sort(data)

		Expect(data[0].NamespacedNameReference.Namespace).To(Equal("a"))
		Expect(data[0].NamespacedNameReference.Name).To(Equal("b"))
		Expect(data[1].NamespacedNameReference.Namespace).To(Equal("b"))
		Expect(data[1].NamespacedNameReference.Name).To(Equal("c"))
	})
})
