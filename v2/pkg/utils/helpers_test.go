// Copyright 2020 IBM Corp.
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

package utils

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Testing with Ginkgo", func() {
	It("apply annotation", func() {

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "myns",
			},
		}

		ApplyAnnotation(cm)
		assert.Contains(GinkgoT(), cm.ObjectMeta.Annotations, RhmAnnotationKey, "Annotations does not contain key")
	})

	It("should chunk by", func() {
		Expect(ChunkBy([]interface{}{}, 2)).To(HaveLen(0))
		Expect(ChunkBy([]interface{}{"a", "b"}, 2)).To(HaveLen(1))
		Expect(ChunkBy([]interface{}{"a", "b", nil, nil}, 2)).To(HaveLen(2))
		chunks := ChunkBy([]interface{}{"a", "b", "c", "d", "e", "f"}, 2)
		Expect(chunks).To(HaveLen(3))
		Expect(chunks[0]).To(Equal([]interface{}{"a", "b"}))
		Expect(chunks[1]).To(Equal([]interface{}{"c", "d"}))
		Expect(chunks[2]).To(Equal([]interface{}{"e", "f"}))
		Expect(func() {
			ChunkBy([]interface{}{"a", "b", nil}, 2)
		}).To(PanicWith("items length is not chunkable by the size"))
	})

	It("Should return a list of meterdefs that is the diff between current and latest", func() {
		testMeterdef1 := marketplacev1beta1.MeterDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "meterdef-1",
				Namespace: "namespace",
			},
		}

		testMeterdef2 := marketplacev1beta1.MeterDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "meterdef-2",
				Namespace: "namespace",
			},
		}

		meterdefsOnCluster := []marketplacev1beta1.MeterDefinition{testMeterdef1, testMeterdef2}
		latestFromCatalog := []marketplacev1beta1.MeterDefinition{testMeterdef1}
		diff := FindMeterdefSliceDiff(meterdefsOnCluster, latestFromCatalog)
		Expect(len(diff)).To(Equal(1))
		Expect(diff[0].Name).To(Equal("meterdef-2"))

		latestFromCatalog = append(latestFromCatalog, testMeterdef2)
		diff = FindMeterdefSliceDiff(meterdefsOnCluster, latestFromCatalog)
		Expect(len(diff)).To(Equal(0))
	})
})
