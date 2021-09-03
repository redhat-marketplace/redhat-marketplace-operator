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
	"fmt"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = FDescribe("MeterDefinitionInstall Controller", func() {
	var (
		mdefInstallCtrl *MeterdefinitionInstallReconciler
		reqLogger logr.Logger
	)

	BeforeEach(func() {
		mdefInstallCtrl = &MeterdefinitionInstallReconciler{}
		mdefInstallCtrl.Log = ctrl.Log.WithName("controllers").WithName("MeterDefinitionInstall")
	})

	namespacedName := types.NamespacedName{
		Name: "foo",
		Namespace: "bar",
	}
	csvOnCluster := olmv1alpha1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Annotations: map[string]string{
				"operatorframework.io/properties": `{"properties":[{"type":"olm.gvk","value":{"group":"app.joget.com","kind":"JogetDX","version":"v1alpha1"}},{"type":"olm.package","value":{"packageName":"joget-dx-operator-rhmp","version":"0.0.13"}}]}`,
			},
		},
	}


	It("should parse out the package name if the olm annotations are provided",func(){
		request := reconcile.Request{
			NamespacedName: namespacedName,
		}

		packageName := mdefInstallCtrl.parsePackageName(&csvOnCluster,request,reqLogger)
		fmt.Println(packageName)
		Expect(packageName).To(Equal("joget-dx-operator-rhmp"))
	})
})
