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

package metrics

import (
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/api/marketplace/v1beta1"
)

func ComposeMetricGenFuncs(familyGens []FamilyGenerator) func(interface{}, []*marketplacev1beta1.MeterDefinition) []FamilyByteSlicer {
	return func(obj interface{}, meterDefinitions []*marketplacev1beta1.MeterDefinition) []FamilyByteSlicer {
		families := make([]FamilyByteSlicer, len(familyGens))

		for i, gen := range familyGens {
			family := gen.GenerateMeterFunc(obj, meterDefinitions)
			family.Name = gen.Name
			families[i] = family
		}

		return families
	}
}

// ExtractMetricFamilyHeaders takes in a slice of FamilyGenerator metrics and
// returns the extracted headers.
func ExtractMetricFamilyHeaders(families []FamilyGenerator) []string {
	headers := make([]string, len(families))

	for i, f := range families {
		headers[i] = f.generateHeader()
	}

	return headers
}
