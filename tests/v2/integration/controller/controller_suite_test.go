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

package controller_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	// register tests
	"github.com/redhat-marketplace/redhat-marketplace-operator/tests/v2/harness"
	"github.com/redhat-marketplace/redhat-marketplace-operator/tests/v2/integration/testutils"
)

const (
	Namespace = "openshift-redhat-marketplace"
)

var (
	cfg         *rest.Config
	testHarness *harness.TestHarness
	testEnv     *envtest.Environment
	cc          reconcileutils.ClientCommandRunner
	stop        chan struct{}
)

func TestEnv(t *testing.T) {
	logf.SetLogger(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	RegisterFailHandler(harness.PodFailHandler(testHarness))

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller EnvTest Suite",
		[]Reporter{printer.NewlineReporter{}})
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
