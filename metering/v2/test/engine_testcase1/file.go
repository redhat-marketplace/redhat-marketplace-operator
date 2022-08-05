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

package testcase1

import (
	"embed"

	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/test"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (

	//go:embed *
	fs embed.FS

	testCase1 test.UnstructuredFS = test.UnstructuredFS{
		FS: fs,
	}

	MdefChargeBack    *unstructured.Unstructured = testCase1.MustGetUnstructured("mdef-chargeback.yaml")
	CSVLicensing      *unstructured.Unstructured = testCase1.MustGetUnstructured("csv-licensing-service.yaml")
	ServiceInstance   *unstructured.Unstructured = testCase1.MustGetUnstructured("service-ibm-licensing-service-instance.yaml")
	ServicePrometheus *unstructured.Unstructured = testCase1.MustGetUnstructured("service-ibm-licensing-service-prometheus.yaml")
	MDefExample       *unstructured.Unstructured = testCase1.MustGetUnstructured("mdef-pod.yaml")
	Pod               *unstructured.Unstructured = testCase1.MustGetUnstructured("pod.yaml")
	OperatorGroup     *unstructured.Unstructured = testCase1.MustGetUnstructured("operatorgroup.yaml")
)
