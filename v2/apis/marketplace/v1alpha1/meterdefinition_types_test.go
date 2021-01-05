package v1alpha1

import (
	"fmt"

	"github.com/imdario/mergo"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

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

		k := Fields{
			"ResourceFilters": MatchAllFields(Fields{
				"Namespace":  PointTo(Equal(v1beta1.NamespaceFilter{UseOperatorGroup: true})),
				"Annotation": BeNil(),
				"Label":      BeNil(),
				"OwnerCRD": PointTo(Equal(v1beta1.OwnerCRDFilter{GroupVersionKind: common.GroupVersionKind{
					APIVersion: "apps.partner.metering.com/v1",
					Kind:       "App",
				}})),
				"WorkloadType": Equal(v1beta1.WorkloadTypePod),
			}),
		}

		k1 := Fields{
			"Metric":      Equal("rpc_durations_seconds_sum"),
			"Query":       Equal("rpc_durations_seconds_sum"),
			"Aggregation": Equal("sum"),
		}

		k2 := Fields{
			"Metric":      Equal("rpc_durations_seconds_count"),
			"Query":       Equal("my_query"),
			"Aggregation": Equal("min"),
		}

		mergo.Merge(&k1, k)
		mergo.Merge(&k2, k)

		fmt.Println(v1beta1ID(mdefBeta.Spec.Meters[0]))

		Expect(mdefBeta.Spec).To(MatchAllFields(Fields{
			"Group": Equal("apps.partner.metering.com"),
			"Kind":  Equal("App"),
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
