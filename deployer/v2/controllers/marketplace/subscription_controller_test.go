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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	// . "github.com/onsi/gomega/gstruct"
	// olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"

	// corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	// "k8s.io/apimachinery/pkg/types"
	// "sigs.k8s.io/controller-runtime/pkg/client"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Testing with Ginkgo", func() {
	// idFn := func(element interface{}) string {
	// 	return fmt.Sprintf("%v", element)
	// }

	var (
		uninstallSubName       = "sub-uninstall"
		subForDeletion         *olmv1alpha1.Subscription
		clusterServiceVersions *olmv1alpha1.ClusterServiceVersion
	)

	BeforeEach(func() {
		subForDeletion = &olmv1alpha1.Subscription{
			ObjectMeta: v1.ObjectMeta{
				Name:      uninstallSubName,
				Namespace: operatorNamespace,
			},
			Spec: &olmv1alpha1.SubscriptionSpec{
				CatalogSource:          "source",
				CatalogSourceNamespace: "source-namespace",
				Package:                "source-package",
			},
			Status: olmv1alpha1.SubscriptionStatus{
				InstalledCSV: "csv-1",
			},
		}

		clusterServiceVersions = &olmv1alpha1.ClusterServiceVersion{
			ObjectMeta: v1.ObjectMeta{
				Name:      "csv-1",
				Namespace: operatorNamespace,
			},
			Spec: olmv1alpha1.ClusterServiceVersionSpec{
				InstallStrategy: olmv1alpha1.NamedInstallStrategy{
					StrategySpec: olmv1alpha1.StrategyDetailsDeployment{
						DeploymentSpecs: []olmv1alpha1.StrategyDeploymentSpec{},
					},
				},
			},
		}

	})

	Context("subs and og", func() {
		BeforeEach(func() {
			Expect(k8sClient.Create(context.TODO(), subForDeletion.DeepCopy())).Should(Succeed(), "create subscription for deletion")
			Expect(k8sClient.Create(context.TODO(), clusterServiceVersions.DeepCopy())).Should(Succeed(), "create subscription for deletion")
		})

		It("test subscription delete", func(ctx SpecContext) {
			sub := &olmv1alpha1.Subscription{}
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      uninstallSubName,
				Namespace: operatorNamespace,
			}, sub)).Should(Succeed(), "get sub for update")

			sub.Status.InstalledCSV = "csv-1"
			sub.Status.LastUpdated = metav1.Now()
			Expect(k8sClient.Status().Update(context.TODO(), sub)).Should(Succeed(), "update sub status")

			sub.Labels = map[string]string{
				utils.UninstallTag: "true",
			}
			Expect(k8sClient.Update(context.TODO(), sub)).Should(Succeed(), "update sub labels")

			Eventually(func() bool {
				deletedSub := &olmv1alpha1.Subscription{}
				return k8serrors.IsNotFound(k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      uninstallSubName,
					Namespace: operatorNamespace,
				}, deletedSub))
			}, timeout, interval).Should(BeTrue(), "check that sub was deleted")

			Eventually(func() []string {
				csvList := &olmv1alpha1.ClusterServiceVersionList{}
				k8sClient.List(context.TODO(), csvList)

				var csvNames []string
				for _, csv := range csvList.Items {
					csvNames = append(csvNames, csv.Name)
				}

				return csvNames
			}, timeout, interval).Should(And(
				HaveLen(0),
			), "check that csv was deleted")

		}, SpecTimeout(time.Second*120))

	})
})
