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

package matcher

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Matcher functions", func() {
	var (
		namespace = "default"
		jogetv13 *olmv1alpha1.ClusterServiceVersion
		jogetv14 *olmv1alpha1.ClusterServiceVersion
	)

	const (
		jogetv13Name string = "joget-openshift-operator.v0.0.13"
		jogetv14Name string = "joget-openshift-operator.v0.0.14"
		jogetCsvSplitName string = "joget-openshift-operator"
		jogetRhmpPackageName string = "joget-dx-operator-rhmp"
		jogetRhmpSubName string = "joget-sub"
		catalogName string = "redhat-marketplace"
		operatorTag = "marketplace.redhat.com/operator"
	)

	jogetV13CsvKey := types.NamespacedName{
		Name: jogetv13Name,
		Namespace: namespace,
	}

	jogetV14CsvKey := types.NamespacedName{
		Name: jogetv14Name,
		Namespace: namespace,
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
	}


	Context("successfully parse and match csv to subscription",func() {
		BeforeEach(func(){
			jogetv13 = &olmv1alpha1.ClusterServiceVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jogetV13CsvKey.Name,
					Namespace: jogetV13CsvKey.Namespace,
					Annotations: map[string]string{
						"operatorframework.io/properties": fmt.Sprintf(`{"properties":[{"type":"olm.gvk","value":{"group":"app.joget.com","kind":"JogetDX","version":"v1alpha1"}},{"type":"olm.package","value":{"packageName":"%v","version":"0.0.13"}}]}`,jogetRhmpPackageName),
					},
				},
			}
			subs[0].Status.CurrentCSV = jogetv13Name
			subs[0].Status.InstalledCSV = jogetv13Name
		})

		It("should parse out the package name and select the matching subscription",func(){
			packageName,err := ParsePackageName(jogetv13)
			Expect(packageName).To(Equal(jogetRhmpPackageName))
			Expect(err).NotTo(HaveOccurred())
	
			foundSub, err := MatchCsvToSub(catalogName,packageName,subs,jogetv13)
			Expect(foundSub).To(Not(BeNil()))
			Expect((foundSub.Name)).To(Equal(jogetRhmpSubName))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("subscription status is being updated",func() {
		BeforeEach(func() {
			jogetv14 = &olmv1alpha1.ClusterServiceVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jogetV14CsvKey.Name,
					Namespace: jogetV14CsvKey.Namespace,
					Annotations: map[string]string{
						"operatorframework.io/properties": fmt.Sprintf(`{"properties":[{"type":"olm.gvk","value":{"group":"app.joget.com","kind":"JogetDX","version":"v1alpha1"}},{"type":"olm.package","value":{"packageName":"%v","version":"0.0.14"}}]}`,jogetRhmpPackageName),
					},
				},
			}

			subs[0].Status.CurrentCSV = jogetv14Name
			subs[0].Status.InstalledCSV = jogetv13Name
		})
	
		It("should return with an updating error message",func(){
			packageName,err := ParsePackageName(jogetv14)
			Expect(packageName).To(Equal(jogetRhmpPackageName))
			Expect(err).NotTo(HaveOccurred())
			
			foundSub, err := MatchCsvToSub(catalogName,packageName,subs,jogetv14)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ErrSubscriptionIsUpdating))
			Expect(foundSub).To(BeNil())			
		})
	})

	Context("no olm annotations",func() {
		BeforeEach(func() {
			jogetv14 = &olmv1alpha1.ClusterServiceVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jogetV14CsvKey.Name,
					Namespace: jogetV14CsvKey.Namespace,
				},
			}

			subs[0].Status.CurrentCSV = jogetv14Name
			subs[0].Status.InstalledCSV = jogetv14Name
		})
	
		It("should try and match the csv to subscription without using olm annotations",func(){
			packageName,_ := ParsePackageName(jogetv14)
			Expect(packageName).To(BeEmpty())
	
			foundSub, err := MatchCsvToSub(catalogName,packageName,subs,jogetv14)
			Expect(foundSub).To(Not(BeNil()))
			Expect((foundSub.Name)).To(Equal(jogetRhmpSubName))
			Expect(err).NotTo(HaveOccurred())			
		})
	})
})
