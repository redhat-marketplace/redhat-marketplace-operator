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

package controller_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/onsi/gomega/gstruct"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"

	. "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MeterDefController reconcile", func() {
	Context("Meterdefinition reconcile", func() {

		var meterdef *v1beta1.MeterDefinition

		BeforeEach(func() {
			meterdef = &v1beta1.MeterDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "meterdef-controller-test",
					Namespace: Namespace,
				},
				Spec: v1beta1.MeterDefinitionSpec{
					Group: "marketplace.redhat.com",
					Kind:  "Pod",

					ResourceFilters: []v1beta1.ResourceFilter{
						{
							WorkloadType: v1beta1.WorkloadTypePod,
							Label: &v1beta1.LabelFilter{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app.kubernetes.io/name": "rhm-metric-state",
									},
								},
							},
						},
					},
					Meters: []v1beta1.MeterWorkload{
						{
							Aggregation: "sum",
							Period: &metav1.Duration{
								Duration: time.Duration(time.Minute*15),
							},
							Query:        "kube_pod_info{} or on() vector(0)",
							Metric:       "meterdef_controller_test_query",
							WorkloadType: v1beta1.WorkloadTypePod,
							Name:         "meterdef_controller_test_query",
						},
					},
				},
			}

			Expect(testHarness.Create(context.TODO(), meterdef)).Should(SucceedOrAlreadyExist, "create the meterdef")
		})

		AfterEach(func() {
			testHarness.Delete(context.TODO(), meterdef)
		})

		It("Should query prometheus and append metric data to meterdef status", func() {
			Expect(testHarness.Get(context.TODO(), types.NamespacedName{Name: "meterdef-controller-test", Namespace: Namespace}, meterdef)).Should(Succeed(), "find a meterdef")

			c := &counter{}
			Eventually(func() *v1beta1.MeterDefinition {
				err := testHarness.Get(context.TODO(), types.NamespacedName{Name: "meterdef-controller-test", Namespace: Namespace}, meterdef)

				if err != nil {
					return nil
				}

				return meterdef
			}, 300).Should(
				WithTransform(
					func(i interface{}) interface{} { c = &counter{}; return i },
					runAssertionOnMeterDef(c)))
		})
	})
})

type counter struct {
	sync.Mutex
	count int
}

func (c *counter) Next() int {
	c.Lock()
	defer c.Unlock()

	next := c.count
	c.count = c.count + 1
	fmt.Println(next)
	return next
}

func (c *counter) Identity(element interface{}) string {
	return fmt.Sprintf("%v", c.Next())
}

func runAssertionOnMeterDef(c *counter) GomegaMatcher {
	return And(
		WithTransform(func(meterdef *v1beta1.MeterDefinition) []common.Result {
			return meterdef.Status.Results
		}, And(Not(BeNil()), HaveLen(1),
			MatchAllElements(func(i interface{}) string {
				result := i.(common.Result)
				return result.MetricName
			}, Elements{
				"meterdef_controller_test_query": MatchFields(IgnoreExtras, Fields{
					"MetricName": Equal("meterdef_controller_test_query"),
					"Query":      Equal(`sum by (pod,namespace) (avg(meterdef_pod_info{meter_def_name="meterdef-controller-test",meter_def_namespace="openshift-redhat-marketplace"}) without (pod_uid,pod_ip,instance,image_id,host_ip,node,container,job,service,endpoint,cluster_ip) * on(pod,namespace) group_right kube_pod_info{} or on() vector(0)) * on(pod,namespace) group_right group without(pod_uid,pod_ip,instance,image_id,host_ip,node,container,job,service,endpoint,cluster_ip) (kube_pod_info{} or on() vector(0))`),
					"Values": MatchElements(c.Identity, IgnoreExtras,
						Elements{
							"0": MatchFields(IgnoreExtras, Fields{
								"Timestamp": Not(BeZero()),
								"Value":     Equal("1"),
							}),
						},
					),
				}),
			}),
		)))
}
