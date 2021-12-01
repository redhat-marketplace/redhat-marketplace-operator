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

package predicates

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Predicate tests", func() {
	Context("SyncedMapHandler", func() {
		var (
			sut              *SyncedMapHandler
			obj1, obj2, obj3 types.NamespacedName
			mapObj           client.Object
			result           reconcile.Request
		)

		BeforeEach(func() {
			sut = NewSyncedMapHandler(func(in types.NamespacedName) bool {
				return false
			})
			obj1 = types.NamespacedName{Name: "foo", Namespace: "foons"}
			obj2 = types.NamespacedName{Name: "bar", Namespace: "foons"}
			obj3 = types.NamespacedName{Name: "baz", Namespace: "foons"}
			mapObj = client.Object(&corev1.Pod{
				ObjectMeta: v1.ObjectMeta{Name: "foo", Namespace: "foons"},
			})
			result.NamespacedName = obj2

			sut.AddOrUpdate(obj1, obj2)
		})

		It("should map to nothing if no obj provided", func() {
			results := sut.Map(nil)
			Expect(results, HaveLen(0))
		})

		It("should map to nothing if no obj provided", func() {
			results := sut.Map(mapObj)
			Expect(results, HaveLen(1))
			Expect(results[0], Equal(result))
		})

		It("should add", func() {
			sut.AddOrUpdate(obj3, obj2)
			results := sut.Map(mapObj)
			Expect(results, HaveLen(1))
			Expect(results[0], Equal(result))
		})

		It("should cleanup", func() {
			sut.Cleanup()
			results := sut.Map(mapObj)
			Expect(results, HaveLen(0))
		})
	})
})
