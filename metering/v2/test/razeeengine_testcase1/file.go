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

	//	ClusterServiceVersion *unstructured.Unstructured = testCase1.MustGetUnstructured("clusterserviceversion.yaml")
	//	ClusterVersion        *unstructured.Unstructured = testCase1.MustGetUnstructured("clusterversion.yaml")
	//	Console               *unstructured.Unstructured = testCase1.MustGetUnstructured("console.yaml")
	Deployment *unstructured.Unstructured = testCase1.MustGetUnstructured("deployment.yaml")

//	Infrastructure        *unstructured.Unstructured = testCase1.MustGetUnstructured("infrastructure.yaml")
//	MarketplaceConfig     *unstructured.Unstructured = testCase1.MustGetUnstructured("marketplaceconfig.yaml")
//	Node                  *unstructured.Unstructured = testCase1.MustGetUnstructured("node.yaml")
//	Subscription          *unstructured.Unstructured = testCase1.MustGetUnstructured("subscription.yaml")
)
