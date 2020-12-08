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

package apis_test

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const namespace = "openshift-redhat-marketplace"

func RandomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	chars := []rune("abcdefghijklmnopqrstuvwxyz" +
		"0123456789")
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	return b.String()
}

var _ = Describe("MeterBase", func() {
	var (
		key              types.NamespacedName
		created, fetched v1alpha1.MeterBase
	)

	AfterEach(func(){
		testHarness.Delete(context.TODO(), &created)
	})

	// Add Tests for OpenAPI validation (or additional CRD features) specified in
	// your API definition.
	// Avoid adding tests for vanilla CRUD operations because they would
	// test Kubernetes API server, which isn't the goal here.
	Context("Create API", func() {
		It("should create an object successfully", func() {
			key = types.NamespacedName{
				Name:      "foo-" + RandomString(5),
				Namespace: namespace,
			}
			created = v1alpha1.MeterBase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: v1alpha1.MeterBaseSpec{
					Enabled: true,
					Prometheus: &v1alpha1.PrometheusSpec{
						Storage: v1alpha1.StorageSpec{
							Size: resource.MustParse("30Gi"),
						},
						Replicas: ptr.Int32(2),
					},
				},
			}

			By("creating an API obj")
			Expect(testHarness.Create(context.TODO(), &created)).To(Succeed())

			fetched = v1alpha1.MeterBase{}
			Expect(testHarness.Get(context.TODO(), key, &fetched)).To(Succeed())
			Expect(fetched).ToNot(BeNil())
			Expect(fetched).To(MatchFields(IgnoreExtras, Fields{
				"ObjectMeta": MatchFields(IgnoreExtras, Fields{
					"Name":      Equal(created.Name),
					"Namespace": Equal(created.Namespace),
				}),
				"Spec": MatchFields(IgnoreExtras, Fields{
					"Enabled": BeTrue(),
					"Prometheus": PointTo(MatchFields(IgnoreExtras, Fields{
						"Storage": MatchAllFields(Fields{
							"Class":    BeNil(),
							"Size":     Equal(resource.MustParse("30Gi")),
							"EmptyDir": BeNil(),
						}),
						"Replicas": PointTo(Equal(int32(2))),
					})),
				}),
			}))

			By("deleting the created object")
			Expect(testHarness.Get(context.TODO(), key, &created)).To(Succeed())
		})
	})
})
