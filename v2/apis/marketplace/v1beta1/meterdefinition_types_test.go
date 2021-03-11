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

package v1beta1

import (
	"bytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var _ = Describe("meterdefinition", func() {
	var mdefYaml = `apiVersion: marketplace.redhat.com/v1beta1
kind: MeterDefinition
metadata:
  name: example-meterdefinition-4
spec:
  group: partner.metering.com
  kind: App
  resourceFilters:
    - namespace:
        useOperatorGroup: true
      ownerCRD:
        apiVersion: marketplace.redhat.com/v1alpha1
        kind: RazeeDeployment
      workloadType: Pod
  meters:
    - aggregation: sum
      period: 1h
      metricId: container_cpu_usage_core_avg
      query: rate(container_cpu_usage_seconds_total{cpu="total",container="db"}[5m])*100
      workloadType: Pod
`

	It("should jsonify", func() {
		mdef := &MeterDefinition{}
		err := yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(mdefYaml)), 100).Decode(mdef)
		Expect(err).To(Succeed())
	})
})
