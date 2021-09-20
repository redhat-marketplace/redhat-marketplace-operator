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

package catalog

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega/ghttp"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"

	. "github.com/onsi/gomega"
	// . "github.com/redhat-marketplace/redhat-marketplace-operator/v2/tests/mock/mock_query"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Catalog Client", func() {

	var (
		catalogClientMockServer      *ghttp.Server
		systemMeterdefsPath = "/" + GetSystemMeterdefinitionTemplatesEndpoint
		namespace = "default"
		systemMeterDef1Name  = "system-meterdef" + "-" + "pod-count"
		
	)

	serviceKey := types.NamespacedName{Name: utils.DeploymentConfigName,Namespace: namespace}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceKey.Name,
			Namespace: serviceKey.Namespace,
		},
	}

	servingCertsCMKey := types.NamespacedName{Name: "serving-certs-cs-bundle",Namespace: namespace}
	servingCertsCm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: servingCertsCMKey.Name,
			Namespace: servingCertsCMKey.Namespace,
		},
		Data: map[string]string{
			"ca.cert" : "test",
		},
	}

	serviceAccount := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-service",
			Namespace: namespace,
		},
	}

	Status200 := http.StatusOK

	systemMeterDef1 := marketplacev1beta1.MeterDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "MeterDefinition",
			APIVersion: "marketplace.redhat.com/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      systemMeterDef1Name,
			Namespace: "default",
			Annotations: map[string]string{
				"versionRange": "<=0.0.1",
			},
			Labels: map[string]string{
				"marketplace.redhat.com/installedOperatorNameTag": "csv-1",
				"marketplace.redhat.com/isSystemMeterDefinition":  "1",
			},
		},
		Spec: marketplacev1beta1.MeterDefinitionSpec{
			Group: "apps.partner.metering.com",
			Kind:  "App",
			ResourceFilters: []marketplacev1beta1.ResourceFilter{
				{
					WorkloadType: marketplacev1beta1.WorkloadTypeService,
					OwnerCRD: &marketplacev1beta1.OwnerCRDFilter{
						GroupVersionKind: common.GroupVersionKind{
							APIVersion: "test_package_1.com/v2",
							Kind:       "test_package_1Cluster",
						},
					},
					Namespace: &marketplacev1beta1.NamespaceFilter{
						UseOperatorGroup: true,
					},
				},
			},
			Meters: []marketplacev1beta1.MeterWorkload{
				{
					Aggregation: "sum",
					GroupBy:     []string{"namespace"},
					Period: &metav1.Duration{
						Duration: time.Duration(time.Hour * 1),
					},
					Query:        "kube_service_labels{}",
					Metric:       "test_package_1_cluster_count",
					WorkloadType: marketplacev1beta1.WorkloadTypeService,
					Without:      []string{"label_test_package_1_cluster", "label_app", "label_operator_test_package_1_com_version"},
				},
			},
		},
	}

	BeforeEach(func() {
		catalogClientMockServer.Start()
		Expect(k8sClient.Create(context.TODO(),servingCertsCm)).Should(Succeed())
		Expect(k8sClient.Create(context.TODO(),service)).Should(Succeed())
		Expect(k8sClient.Create(context.TODO(),&serviceAccount)).Should(Succeed())


		returnedSystemMeterDefSlice := []marketplacev1beta1.MeterDefinition{*systemMeterDef1.DeepCopy()}
		systemMeterDefBody, err := json.Marshal(returnedSystemMeterDefSlice)
		if err != nil {
			log.Fatal(err)
		}

		catalogClientMockServer.RouteToHandler(
			"POST", systemMeterdefsPath, ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", systemMeterdefsPath),
				ghttp.RespondWithPtr(&Status200, &systemMeterDefBody),
			))

	})

	AfterEach(func(){
		catalogClientMockServer.Close()
	})

	It("should query a range", func() {
	
	})
})
