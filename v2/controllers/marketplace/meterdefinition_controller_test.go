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

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MeterDefinitionController", func() {
	var (
		name            = "meterdefinition"
		meterdefinition = &marketplacev1beta1.MeterDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: operatorNamespace,
			},
			Spec: marketplacev1beta1.MeterDefinitionSpec{
				Group: "apps.partner.metering.com",
				Kind:  "App",
				ResourceFilters: []marketplacev1beta1.ResourceFilter{
					{
						WorkloadType: common.WorkloadTypePod,
						Label: &marketplacev1beta1.LabelFilter{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": "app",
								},
							},
						},
					},
				},
				Meters: []marketplacev1beta1.MeterWorkload{
					{
						Aggregation: "sum",
						Period: &metav1.Duration{
							Duration: time.Duration(time.Minute * 15),
						},
						Query:        "kube_pod_info{} or on() vector(0)",
						Metric:       "meterdef_controller_test_query",
						WorkloadType: common.WorkloadTypePod,
						Name:         "meterdef_controller_test_query",
					},
				},
			},
		}

		badMeterdefinition = &marketplacev1beta1.MeterDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: operatorNamespace,
			},
			Spec: marketplacev1beta1.MeterDefinitionSpec{
				Group: "apps.partner.metering.com",
				Kind:  "App",
			},
		}
	)

	It("not create badMeterdefinition", func() {
		Expect(k8sClient.Create(context.TODO(), badMeterdefinition.DeepCopy())).ShouldNot(Succeed(), "not create badMeterdefinition CR")
	})

	It("create meterdefinition", func() {
		Expect(k8sClient.Create(context.TODO(), meterdefinition.DeepCopy())).Should(Succeed(), "create meterdefinition CR")

		// Sig status updated
		Eventually(func() bool {
			md := &marketplacev1beta1.MeterDefinition{}
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      name,
				Namespace: operatorNamespace,
			}, md)).Should(Succeed(), "get meterdefinition")
			return md.Status.Conditions.GetCondition(common.MeterDefConditionTypeSignatureVerified) != nil
		}, timeout, interval).Should(BeTrue(), "meterdefinition has MeterDefConditionTypeSignatureVerified")

		// Further test path requires OpenShift ClusterVersion, UWM config, and MeterBase Status affirming UWM enabled
		// Once it encounters querypreview, would fail without a faked prom endpoint
	})
})
