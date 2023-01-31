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

package apis_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/redhat-marketplace-operator/tests/v2/harness"
	"github.com/redhat-marketplace/redhat-marketplace-operator/tests/v2/integration/testutils"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

var testHarness *harness.TestHarness

func TestApis(t *testing.T) {
	RegisterFailHandler(harness.PodFailHandler(testHarness))
	RunSpecs(t, "Apis Suite")
}

var _ = BeforeSuite(func() {
	var err error
	By("setting up env")

	testHarness, err = harness.NewTestHarness(harness.TestHarnessOptions{
		EnabledFeatures: []string{},
		Namespace:       "openshift-redhat-marketplace",
		WatchNamespace:  "",
		ProvideScheme: func(cfg *rest.Config) (*runtime.Scheme, error) {
			return testutils.GetScheme(), nil
		},
	})
	Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%v+", err))

	_, err = testHarness.Start()
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	testHarness.Stop()
}, 60)
