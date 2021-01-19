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

package v1alpha1

import (
	"fmt"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

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

var _ = Describe("MeterDefinition", func() {

	var definition *MeterDefinition

	BeforeEach(func() {
		definition = &MeterDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
				UID:       types.UID("a"),
			},
			Spec: MeterDefinitionSpec{
				Group:              "apps.partner.metering.com",
				Kind:               "App",
				WorkloadVertexType: WorkloadVertexOperatorGroup,
				Workloads: []Workload{
					{
						Name:         "foo",
						WorkloadType: WorkloadTypePod,
						MetricLabels: []MeterLabelQuery{
							{
								Label:       "rpc_durations_seconds_sum",
								Query:       "rpc_durations_seconds_sum",
								Aggregation: "sum",
							},
							{
								Label:       "rpc_durations_seconds_count",
								Query:       "my_query",
								Aggregation: "min",
							},
						},
						OwnerCRD: &common.GroupVersionKind{
							APIVersion: "apps.partner.metering.com/v1",
							Kind:       "App",
						},
					},
				},
			},
		}
	})

	It("should convert to/from v1beta1", func() {
		mdefBeta := &v1beta1.MeterDefinition{}
		err := definition.ConvertTo(mdefBeta)
		Expect(err).To(Succeed())
		Expect(len(mdefBeta.Spec.Meters)).To(Equal(2))

		v1beta1ID := func(element interface{}) string {
			return element.(v1beta1.MeterWorkload).Metric
		}

		resourceFilter := MatchAllFields(Fields{
			"Namespace":  PointTo(Equal(v1beta1.NamespaceFilter{UseOperatorGroup: true})),
			"Annotation": BeNil(),
			"Label":      BeNil(),
			"OwnerCRD": PointTo(Equal(v1beta1.OwnerCRDFilter{GroupVersionKind: common.GroupVersionKind{
				APIVersion: "apps.partner.metering.com/v1",
				Kind:       "App",
			}})),
			"WorkloadType": Equal(v1beta1.WorkloadTypePod),
		})

		k1 := Fields{
			"Metric":       Equal("rpc_durations_seconds_sum"),
			"Query":        Equal("rpc_durations_seconds_sum"),
			"Aggregation":  Equal("sum"),
			"WorkloadType": Equal(v1beta1.WorkloadTypePod),
		}

		k2 := Fields{
			"Metric":       Equal("rpc_durations_seconds_count"),
			"Query":        Equal("my_query"),
			"Aggregation":  Equal("min"),
			"WorkloadType": Equal(v1beta1.WorkloadTypePod),
		}

		fmt.Println(v1beta1ID(mdefBeta.Spec.Meters[0]))
		c := &counter{}

		Expect(mdefBeta.Spec).To(MatchAllFields(Fields{
			"Group": Equal("apps.partner.metering.com"),
			"Kind":  Equal("App"),
			"ResourceFilters": MatchAllElements(c.Identity,
				Elements{
					"0": resourceFilter,
				},
			),
			"Meters": MatchAllElements(v1beta1ID,
				Elements{
					"rpc_durations_seconds_sum":   MatchFields(IgnoreExtras, k1),
					"rpc_durations_seconds_count": MatchFields(IgnoreExtras, k2),
				},
			),
			"InstalledBy": BeNil(),
		}))

		newSource := &MeterDefinition{}
		err = newSource.ConvertFrom(mdefBeta)
		Expect(err).To(Succeed())
	})
})
