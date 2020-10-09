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

package reporter

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Query", func() {

	var (
		sut      *MarketplaceReporter
		start, _ = time.Parse(time.RFC3339, "2020-04-19T13:00:00Z")
		end, _   = time.Parse(time.RFC3339, "2020-04-19T16:00:00Z")

		rpcDurationSecondsQuery *PromQuery
	)

	BeforeEach(func() {
		rpcDurationSecondsQuery = &PromQuery{
			Metric: "rpc_durations_seconds_count",
			Query:  `foo{bar="true"}`,
			Start:  start,
			End:    end,
			Step:   time.Minute * 60,
		}

		v1api := getTestAPI(mockResponseRoundTripper("../../test/mockresponses/prometheus-query-range.json"))
		sut = &MarketplaceReporter{
			api: v1api,
		}
	})

	It("should query a range", func() {
		result, warnings, err := sut.queryRange(rpcDurationSecondsQuery)
		Expect(err).To(Succeed())
		Expect(warnings).To(BeEmpty(), "warnings should be empty")
		Expect(model.ValMatrix).To(Equal(result.Type()), "value type matrix expected")

		matrixResult, ok := result.(model.Matrix)

		Expect(ok).To(BeTrue(), "result is not a matrix")
		Expect(len(matrixResult)).To(Equal(2))
	})

	It("should build a query", func() {
		q1 := &PromQuery{
			Metric: "foo",
			Query:  "kube_persistentvolumeclaim_resource_requests_storage_bytes",
			MeterDef: types.NamespacedName{
				Name:      "foo",
				Namespace: "foons",
			},
			AggregateFunc: "sum",
			Type:          v1alpha1.WorkloadTypePVC,
		}

		expected := "sum by (persistentvolumeclaim,namespace) (avg(meterdef_persistentvolumeclaim_info{meter_def_name=\"foo\",meter_def_namespace=\"foons\",phase=\"Bound\"}) without (instance, container, endpoint, job, service) * on(persistentvolumeclaim,namespace) group_right kube_persistentvolumeclaim_resource_requests_storage_bytes)"
		Expect(q1.String()).To(Equal(expected), "failed to create query for pvc")
	})

	// It("should include additional fields in the query", func() {
	// 	meterdefinition := &marketplacev1alpha1.MeterDefinition{
	// 		ObjectMeta: metav1.ObjectMeta{
	// 			Name:      "test-meterdef",
	// 			Namespace: "namespace",
	// 		},
	// 		Spec: marketplacev1alpha1.MeterDefinitionSpec{
	// 			Group:   "apps.partner.metering.com",
	// 			Kind:    "App",
	// 			Workloads: []marketplacev1alpha1.Workload{
	// 				{
	// 					MetricLabels: []marketplacev1alpha1.MeterLabelQuery{
	// 						{
	// 							Query: "rate(container_cpu_usage_seconds_total{}[5m])*100",
	// 						},
							
	// 					},
	// 				},
	// 			},
	// 		},
	// 	}

	// 	utils.PrettyPrint(meterdefinition)
	// 	q1 := &PromQuery{
	// 		Metric: "foo",
	// 		Query:  *&meterdefinition.Spec.Workloads[0].MetricLabels[0].Query,
	// 		MeterDef: types.NamespacedName{
	// 			Name:      "foo",
	// 			Namespace: "foons",
	// 		},
	// 		AggregateFunc: "sum",
	// 		Type:          v1alpha1.WorkloadTypePVC,
	// 	}
		
	// 	sut.query()
		
	
	// })

	PIt("should build a query", func() {
		By("building a query with no args")
		q1 := &PromQuery{
			Metric: "foo",
		}

		Expect(q1.String()).To(Equal("foo"), "failed to create query with no args")

		By("building a complicated query")

		expectedFields := []string{`rpc_durations_seconds_count`, `meter_kind="App"`, `meter_domain="apps.partner.metering.com"`}

		for _, field := range expectedFields {
			Expect(rpcDurationSecondsQuery).To(ContainSubstring(field))
		}
	})
})
