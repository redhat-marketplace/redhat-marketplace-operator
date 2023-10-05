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
)

var _ = Describe("Testing with Ginkgo", func() {
	// idFn := func(element interface{}) string {
	// 	return fmt.Sprintf("%v", element)
	// }

	var (
		// name = "new-subscription"
		// arbitraryNamespace = "arbitrary-namespace"
		// sourceNamespace    = "source-namespace"
		uninstallSubName = "sub-uninstall"

		// req reconcile.Request
		// opts []StepOption
		// optsForDeletion          []StepOption
		// preExistingOperatorGroup *olmv1.OperatorGroup
		// subscription             *olmv1alpha1.Subscription
		subForDeletion         *olmv1alpha1.Subscription
		clusterServiceVersions *olmv1alpha1.ClusterServiceVersion
		// aNamespace               *corev1.Namespace
		// sNamespace               *corev1.Namespace
	)

	BeforeEach(func() {
		// req = reconcile.Request{
		// 	NamespacedName: types.NamespacedName{
		// 		Name:      name,
		// 		Namespace: namespace,
		// 	},
		// }
		// opts = []StepOption{
		// 	WithRequest(req),
		// }
		// optsForDeletion = []StepOption{
		// 	WithRequest(reconcile.Request{
		// 		NamespacedName: types.NamespacedName{
		// 			Name:      uninstallSubName,
		// 			Namespace: namespace,
		// 		},
		// 	}),
		// }

		// sNamespace = &corev1.Namespace{
		// 	ObjectMeta: v1.ObjectMeta{
		// 		Name: sourceNamespace,
		// 	},
		// }

		// aNamespace = &corev1.Namespace{
		// 	ObjectMeta: v1.ObjectMeta{
		// 		Name: arbitraryNamespace,
		// 	},
		// }

		// preExistingOperatorGroup = &olmv1.OperatorGroup{
		// 	ObjectMeta: v1.ObjectMeta{
		// 		Name:      "existing-group",
		// 		Namespace: operatorNamespace,
		// 	},
		// 	Spec: olmv1.OperatorGroupSpec{
		// 		TargetNamespaces: []string{
		// 			operatorNamespace,
		// 		},
		// 	},
		// }
		// subscription = &olmv1alpha1.Subscription{
		// 	ObjectMeta: v1.ObjectMeta{
		// 		Name:      name,
		// 		Namespace: operatorNamespace,
		// 		Labels: map[string]string{
		// 			utils.OperatorTag: "true",
		// 		},
		// 	},
		// 	Spec: &olmv1alpha1.SubscriptionSpec{
		// 		CatalogSource:          "source",
		// 		CatalogSourceNamespace: "source-namespace",
		// 		Package:                "source-package",
		// 	},
		// }

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

	// var setup = func(r *ReconcilerTest) error {
	// 	var log = logf.Log.WithName("subscription_controller")
	// 	r.Client = fake.NewClientBuilder().WithScheme(k8sScheme).WithObjects(r.GetGetObjects()...).Build()
	// 	r.Reconciler = &SubscriptionReconciler{Client: r.Client, Scheme: k8sScheme, Log: log}
	// 	return nil
	// }

	Context("subs and og", func() {
		BeforeEach(func() {
			Expect(k8sClient.Create(context.TODO(), subForDeletion.DeepCopy())).Should(Succeed(), "create subscription for deletion")
			Expect(k8sClient.Create(context.TODO(), clusterServiceVersions.DeepCopy())).Should(Succeed(), "create subscription for deletion")
		})

		// AfterEach(func() {
		// 	aNs := &corev1.Namespace{}
		// 	Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
		// 		Name:      arbitraryNamespace,
		// 		Namespace: arbitraryNamespace,
		// 	}, aNs)).Should(Succeed(), "get arbitrary namespace")
		// 	k8sClient.Delete(context.TODO(), aNs)

		// 	sNs := &corev1.Namespace{}
		// 	Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
		// 		Name:      sourceNamespace,
		// 		Namespace: sourceNamespace,
		// 	}, sNs)).Should(Succeed(), "get subscription")
		// 	k8sClient.Delete(context.TODO(), sNs)

		// 	Eventually(func() []string {
		// 		nsList := &corev1.NamespaceList{}
		// 		k8sClient.List(context.TODO(), nsList)

		// 		var nsNames []string
		// 		for _, ns := range nsList.Items {
		// 			nsNames = append(nsNames, ns.Name)
		// 		}

		// 		return nsNames
		// 	}, timeout, interval).Should(And(
		// 		HaveLen(0),
		// 	))

		// })

		It("test subscription delete", func(ctx SpecContext) {
			// Expect(k8sClient.Create(context.TODO(), subForDeletion.DeepCopy())).Should(Succeed(), "create subscription for deletion")
			// Expect(k8sClient.Create(context.TODO(), clusterServiceVersions.DeepCopy())).Should(Succeed(), "create subscription for deletion")
			Eventually(func() bool {
				sub := &olmv1alpha1.Subscription{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      uninstallSubName,
					Namespace: operatorNamespace,
				}, sub)).Should(Succeed(), "get sub for update")
				sub.Labels = map[string]string{
					utils.UninstallTag: "bogus",
				}
				sub.Status.InstalledCSV = "csv-1"
				Expect(k8sClient.Update(context.TODO(), sub)).Should(Succeed(), "update sub")

				upDatedSub := &olmv1alpha1.Subscription{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      uninstallSubName,
					Namespace: operatorNamespace,
				}, upDatedSub)).Should(Succeed(), "find updated sub")

				utils.PrettyPrint(upDatedSub)

				return upDatedSub.Status.InstalledCSV == "csv-1"
			}, timeout, interval).Should(BeTrue(), "check that sub was updated")

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
			))

			// Eventually(func() []string {
			// 	subList := &olmv1alpha1.SubscriptionList{}
			// 	k8sClient.List(context.TODO(), subList)

			// 	var subNames []string
			// 	for _, sub := range subList.Items {
			// 		subNames = append(subNames, sub.Name)
			// 	}

			// 	return subNames
			// }, timeout, interval).Should(And(
			// 	HaveLen(0),
			// ))

			// Eventually(func() []string {
			// 	opGrpList := &olmv1.OperatorGroupList{}
			// 	k8sClient.List(context.TODO(), opGrpList)

			// 	var ogNames []string
			// 	for _, og := range opGrpList.Items {
			// 		ogNames = append(ogNames, og.Name)
			// 	}

			// 	return ogNames
			// }, timeout, interval).Should(And(
			// 	HaveLen(0),
			// ))
		}, SpecTimeout(time.Second*120))

		// It("new subscription", func() {
		// 	Expect(k8sClient.Create(context.TODO(), subscription.DeepCopy())).Should(Succeed(), "create subscription for deletion")
		// 	Eventually(func() []string {
		// 		csvList := &olmv1.OperatorGroupList{}

		// 		labels := map[string]string{
		// 			utils.OperatorTag: "true",
		// 		}
		// 		listOpts := []client.ListOption{
		// 			client.MatchingLabels(labels),
		// 		}
		// 		k8sClient.List(context.TODO(), csvList, listOpts...)

		// 		var ogNames []string
		// 		for _, og := range csvList.Items {
		// 			ogNames = append(ogNames, og.Name)
		// 		}

		// 		return ogNames
		// 	}, timeout, interval).Should(And(
		// 		HaveLen(1),
		// 	))
		// })

		// It("new subscription with operator group", func() {
		// 	Expect(k8sClient.Create(context.TODO(), subscription.DeepCopy())).Should(Succeed(), "create subscription for deletion")
		// 	Expect(k8sClient.Create(context.TODO(), preExistingOperatorGroup.DeepCopy())).Should(Succeed(), "create subscription for deletion")

		// 	Eventually(func() []string {
		// 		csvList := &olmv1.OperatorGroupList{}

		// 		k8sClient.List(context.TODO(), csvList)

		// 		var ogNames []string
		// 		for _, og := range csvList.Items {
		// 			ogNames = append(ogNames, og.Name)
		// 		}

		// 		return ogNames
		// 	}, timeout, interval).Should(And(
		// 		HaveLen(1),
		// 		MatchAllElements(idFn, Elements{
		// 			"existing-group": Equal("existing-group"),
		// 		}),
		// 	))
		// })

		// It("delete operator group if too many", func() {
		// 	Expect(k8sClient.Create(context.TODO(), subscription.DeepCopy())).Should(Succeed(), "create subscription for deletion")
		// 	Expect(k8sClient.Create(context.TODO(), preExistingOperatorGroup.DeepCopy())).Should(Succeed(), "create subscription for deletion")

		// 	Eventually(func() []string {
		// 		ogList := &olmv1.OperatorGroupList{}

		// 		k8sClient.List(context.TODO(), ogList)

		// 		var ogNames []string
		// 		for _, og := range ogList.Items {
		// 			ogNames = append(ogNames, og.Name)
		// 		}

		// 		return ogNames
		// 	}, timeout, interval).Should(And(
		// 		HaveLen(0),
		// 	))
		// })

	})

})
