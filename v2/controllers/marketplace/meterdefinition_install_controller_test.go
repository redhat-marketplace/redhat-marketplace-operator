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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = FDescribe("MeterDefinitionInstall Controller", func() {
	var (
		mdefInstallCtrl *MeterdefinitionInstallReconciler
		// reqLogger logr.Logger
		namespace = "default"
	)

	const (
		jogetCsvName string = "joget-openshift-operator.v0.0.13"
		jogetCsvSplitName string = "joget-openshift-operator"
		// jogetVersion = "0.0.13"
		jogetRhmpPackageName string = "joget-dx-operator-rhmp"
		jogetRhmpSubName string = "joget-sub"
	)

	jogetCsvKey := types.NamespacedName{
		Name: jogetCsvName,
		Namespace: namespace,
	}

	jogetCsv := olmv1alpha1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jogetCsvKey.Name,
			Namespace: jogetCsvKey.Namespace,
			Annotations: map[string]string{
				"operatorframework.io/properties": fmt.Sprintf(`{"properties":[{"type":"olm.gvk","value":{"group":"app.joget.com","kind":"JogetDX","version":"v1alpha1"}},{"type":"olm.package","value":{"packageName":"%v","version":"0.0.13"}}]}`,jogetRhmpPackageName),
			},
		},
	}

	subs := []olmv1alpha1.Subscription{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jogetRhmpSubName,
				Namespace: namespace,
				Labels: map[string]string{
					operatorTag: "true",
				},
			},
			Spec: &olmv1alpha1.SubscriptionSpec{
				CatalogSource:          "redhat-marketplace",
				CatalogSourceNamespace: "default",
				Package:                "joget-dx-operator-rhmp",
			},
			Status: olmv1alpha1.SubscriptionStatus{
				InstalledCSV: jogetCsvName,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sub-2",
				Namespace: namespace,
				Labels: map[string]string{
					operatorTag: "true",
				},
			},
			Spec: &olmv1alpha1.SubscriptionSpec{
				CatalogSource:          "wrong-catalog",
				CatalogSourceNamespace: "default",
				Package:                "csv2-package",
			},
		},
		// {
		// 	ObjectMeta: metav1.ObjectMeta{
		// 		Name:      "memcached-subscription",
		// 		Namespace: namespace,
		// 		Labels: map[string]string{
		// 			"marketplace.redhat.com/operator": "true",
		// 		},
		// 	},
		// 	Spec: &olmv1alpha1.SubscriptionSpec{
		// 		Channel:                "alpha",
		// 		InstallPlanApproval:    olmv1alpha1.ApprovalManual,
		// 		Package:                "memcached-operator-rhmp",
		// 		CatalogSource:          "redhat-marketplace",
		// 		CatalogSourceNamespace: "openshift-redhat-marketplace",
		// 	},
		// },
	}

	// catalogSource := &olmv1alpha1.CatalogSource{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name:      "redhat-marketplace",
	// 		Namespace: "openshift-redhat-marketplace",
	// 	},
	// 	Spec: olmv1alpha1.CatalogSourceSpec{
	// 		SourceType: olmv1alpha1.SourceType(olmv1alpha1.SourceTypeGrpc),
	// 		Image:      "quay.io/mxpaspa/memcached-ansible-index:1.0.1",
	// 	},
	// }

	// memcachedSub := &olmv1alpha1.Subscription{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name:      "memcached-subscription",
	// 		Namespace: "openshift-redhat-marketplace",
	// 		Labels: map[string]string{
	// 			"marketplace.redhat.com/operator": "true",
	// 		},
	// 	},

	// 	Spec: &olmv1alpha1.SubscriptionSpec{
	// 		Channel:                "alpha",
	// 		InstallPlanApproval:    olmv1alpha1.ApprovalManual,
	// 		Package:                "memcached-operator-rhmp",
	// 		CatalogSource:          "redhat-marketplace",
	// 		CatalogSourceNamespace: "openshift-redhat-marketplace",
	// 	},
	// }

	BeforeEach(func() {
		mdefInstallCtrl = &MeterdefinitionInstallReconciler{}

		mdefInstallCtrl.Log = ctrl.Log.WithName("controllers").WithName("MeterDefinitionInstall")

		operatorConfig, err := config.GetConfig()
		Expect(err).NotTo(HaveOccurred(),"get operator config")
		mdefInstallCtrl.cfg = operatorConfig

		mdefInstallCtrl.Client = k8sManager.GetClient()
		
		// for _, s := range subs {
		// 	Expect(k8sClient.Create(context.TODO(),&s)).Should(Succeed(),"create subscriptions")
		// }

		Expect(k8sClient.Create(context.TODO(),&jogetCsv),"create csv")
		// Expect(k8sClient.Create(context.TODO(),catalogSource),"create catalog source")
		// time.Sleep(time.Second * 30)
	})

	It("should parse out the package name and select the matching subscription",func(){
		// _jogetSub := &olmv1alpha1.Subscription{}
		// Expect(k8sClient.Get(context.TODO(),types.NamespacedName{Name: jogetRhmpSubName,Namespace: namespace},_jogetSub)).Should(Succeed())

		// _jogetSub.Status.InstalledCSV = jogetCsvName
		// Expect(k8sClient.Update(context.TODO(),_jogetSub)).Should(Succeed(),"update sub with installedCSV")

		// utils.PrettyPrint(_jogetSub)
		request := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: "foo",
				Namespace: namespace,
			},
		}

		reqLogger := mdefInstallCtrl.Log.WithValues("Request.Name", request.Name, "Request.Namespace", request.Namespace)

		packageName := mdefInstallCtrl.parsePackageName(&jogetCsv,request,reqLogger)
		Expect(packageName).To(Equal("joget-dx-operator-rhmp"))

		result, foundSub := mdefInstallCtrl.matchCsvToSub(packageName, subs,&jogetCsv,reqLogger)
		Expect(foundSub).To(Not(BeNil()))
		utils.PrettyPrint(foundSub)
		Expect((foundSub.Name)).To(Equal(jogetRhmpSubName))
		Expect(result).To(Equal(&ExecResult{
			Status: ActionResultStatus(Continue), 
		}))

	})
})
