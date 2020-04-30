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

package marketplaceconfig

import (
	"testing"

	. "github.com/redhat-marketplace/redhat-marketplace-operator/test/controller"

	opsrcApi "github.com/operator-framework/operator-marketplace/pkg/apis"
	opsrcv1 "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	"github.com/spf13/viper"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestMarketplaceConfigController(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	defaultFeatures := []string{"razee", "meterbase"}
	viper.Set("assets", "../../../assets")
	viper.Set("features", defaultFeatures)

	t.Run("Test Clean Install", testCleanInstall)
}

var (
	name                 = "markeplaceconfig"
	namespace            = "redhat-marketplace-operator"
	customerID    string = "example-userid"
	razeeName            = "rhm-marketplaceconfig-razeedeployment"
	meterBaseName        = "rhm-marketplaceconfig-meterbase"
	req                  = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	opts = []TestCaseOption{
		WithRequest(req),
		WithNamespace(namespace),
		WithName(name),
	}
	marketplaceconfig = buildMarketplaceConfigCR(name, namespace, customerID)
	razeedeployment   = utils.BuildRazeeCr(namespace, marketplaceconfig.Spec.ClusterUUID, marketplaceconfig.Spec.DeploySecretName)
	meterbase         = utils.BuildMeterBaseCr(namespace)
)

func setup(r *ReconcilerTest) error {
	s := scheme.Scheme
	_ = opsrcApi.AddToScheme(s)
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, marketplaceconfig)
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, razeedeployment)
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, meterbase)

	r.Client = fake.NewFakeClient(r.GetRuntimeObjects()...)
	r.Reconciler = &ReconcileMarketplaceConfig{client: r.Client, scheme: s}
	return nil
}

func testCleanInstall(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, marketplaceconfig)
	reconcilerTest.TestAll(t,
		[]TestCaseStep{
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&marketplacev1alpha1.RazeeDeployment{}),
					WithName(razeeName))...),
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&marketplacev1alpha1.MeterBase{}),
					WithName(meterBaseName))...),
			NewReconcilerTestCase(
				append(opts,
					WithTestObj(&opsrcv1.OperatorSource{}),
					WithNamespace(utils.OPERATOR_MKTPLACE_NS),
					WithName(utils.OPSRC_NAME))...),
		})
}

// Test whether flags have been set or not
func TestMarketplaceConfigControllerFlags(t *testing.T) {
	flagset := FlagSet()

	if !flagset.HasFlags() {
		t.Errorf("no flags on flagset")
	}
}

func buildMarketplaceConfigCR(name, namespace, customerID string) *marketplacev1alpha1.MarketplaceConfig {
	return &marketplacev1alpha1.MarketplaceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.MarketplaceConfigSpec{
			RhmAccountID: customerID,
		},
	}
}
