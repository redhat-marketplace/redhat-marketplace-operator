// Copyright 2021 Google LLC
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

package checkpath

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("checkpath", func() {
	var obj1, obj2 map[string]interface{}

	BeforeEach(func() {
		obj1 = map[string]interface{}{
			"foo":  "bar",
			"int":  1,
			"bool": true,
			"jar":  []string{"a", "b", "c"},
			"car": map[string]interface{}{
				"wheels": true,
				"count":  4,
				"parts":  []string{"a", "b", "c"},
				"nestedArray": []map[string]interface{}{
					{
						"count": 4,
					},
					{
						"count": 2,
					},
				},
			},
		}

		obj2 = map[string]interface{}{
			"foo":  "bar",
			"bool": true,
			"jar":  []string{"a", "b", "c"},
			"tar":  []string{"a", "b", "c"},
			"car": map[string]interface{}{
				"wheels": true,
				"count":  4,
				"parts":  []string{"a", "b", "c"},
				"nestedArray": []map[string]interface{}{
					{
						"count": 3,
					},
					{
						"count": 2,
					},
				},
			},
		}
	})

	It("should find nothing wrong", func() {
		updates := &CheckUpdatePath{
			Root: "$.foo",
		}
		changed, _, err := updates.Eval(obj1, obj2)
		Expect(err).ToNot(HaveOccurred())
		Expect(changed).To(BeFalse())
	})

	It("find one thing wrong", func() {
		updates := &CheckUpdatePath{
			Root: "$.foo",
			Update: func() {
				obj2["foo"] = obj1["foo"]
			},
		}
		obj1["foo"] = "b"
		changed, _, err := updates.Eval(obj1, obj2)
		Expect(err).ToNot(HaveOccurred())
		Expect(changed).To(BeTrue())

		// check if updates have run
		changed, _, err = updates.Eval(obj1, obj2)
		Expect(changed).To(BeFalse())
	})

	It("find if an array is missing", func() {
		updates := &CheckUpdatePath{
			Root: "$.tar[:].var",
		}
		changed, _, err := updates.Eval(obj1, obj2)
		Expect(err).ToNot(HaveOccurred())
		Expect(changed).To(BeTrue())
	})

	It("find something to update in a list", func() {
		updates := &CheckUpdatePath{
			Root: "$.car.nestedArray[:]",
			Paths: []*CheckUpdatePath{
				{
					Root: "$.count",
				},
			},
			Update: func() {
				obj1["car"].(map[string]interface{})["nestedArray"] = obj2["car"].(map[string]interface{})["nestedArray"]
			},
		}
		changed, _, err := updates.Eval(obj1, obj2)
		Expect(err).ToNot(HaveOccurred())
		Expect(changed).To(BeTrue())

		// check if changes have been applied
		changed, _, err = updates.Eval(obj1, obj2)
		Expect(err).ToNot(HaveOccurred())
		Expect(changed).To(BeFalse())
	})

	It("it will handle missing fields", func() {
		updates := &CheckUpdatePath{
			Root: "$.int",
		}

		changed, _, err := updates.Eval(obj1, obj2)
		Expect(err).ToNot(HaveOccurred())
		Expect(changed).To(BeTrue())
	})
})
