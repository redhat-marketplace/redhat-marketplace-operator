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

package marketplace

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Testing with Ginkgo", func() {
	var (
		name = rhmOperatorName

		subscription = &olmv1alpha1.Subscription{
			ObjectMeta: v1.ObjectMeta{
				Name:      name,
				Namespace: operatorNamespace,
				Labels: map[string]string{
					opreqControlLabel: "true",
				},
			},
			Spec: &olmv1alpha1.SubscriptionSpec{
				CatalogSource:          "source",
				CatalogSourceNamespace: "source-namespace",
				Package:                "source-package",
			},
		}
	)

	BeforeEach(func() {
		Expect(k8sClient.Create(context.TODO(), subscription.DeepCopy())).Should(Succeed(), "create subscription")
	})

	AfterEach(func() {
		subscription := &olmv1alpha1.Subscription{}
		Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
			Name:      name,
			Namespace: operatorNamespace,
		}, subscription)).Should(Succeed(), "get subscription")
		k8sClient.Delete(context.TODO(), subscription)
	})

	It("RHM subscription controller", func() {
		Eventually(func() []string {
			subList := &olmv1alpha1.SubscriptionList{}
			labels := map[string]string{
				doNotUninstallLabel: "true",
			}
			listOpts := []client.ListOption{
				client.MatchingLabels(labels),
			}
			k8sClient.List(context.TODO(), subList, listOpts...)

			var subNames []string
			for _, mdef := range subList.Items {
				subNames = append(subNames, mdef.Name)
			}

			return subNames
		}, timeout, interval).Should(And(
			HaveLen(1),
		))
	})
})
