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

package clusterserviceversion

import (
	"fmt"
	"reflect"
	"testing"

	utils "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/test/rectest"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestClusterServiceVersionController(t *testing.T) {

	logf.SetLogger(logf.ZapLogger(true))
	_ = olmv1alpha1.AddToScheme(scheme.Scheme)

	t.Run("Test New Cluster Service Version with installed CSV", testClusterServiceVersionWithInstalledCSV)
	t.Run("Test New Cluster Service Version without installed CSV", testClusterServiceVersionWithoutInstalledCSV)
	t.Run("Test New Cluster Service Version with Sub without labels", testClusterServiceVersionWithSubscriptionWithoutLabels)
}

var (
	csvName   = "new-clusterserviceversion"
	subName   = "new-subscription"
	namespace = "arbitrary-namespace"

	req = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      csvName,
			Namespace: namespace,
		},
	}
	opts = []StepOption{
		WithRequest(req),
	}

	clusterserviceversion = &olmv1alpha1.ClusterServiceVersion{
		ObjectMeta: v1.ObjectMeta{
			Name:      csvName,
			Namespace: namespace,
		},
	}
	subscription = &olmv1alpha1.Subscription{
		ObjectMeta: v1.ObjectMeta{
			Name:      subName,
			Namespace: namespace,
			Labels: map[string]string{
				operatorTag: "true",
			},
		},
		Status: olmv1alpha1.SubscriptionStatus{
			InstalledCSV: csvName,
		},
	}

	subscriptionWithoutLabels = &olmv1alpha1.Subscription{
		ObjectMeta: v1.ObjectMeta{
			Name:      subName,
			Namespace: namespace,
		},
		Status: olmv1alpha1.SubscriptionStatus{
			InstalledCSV: csvName,
		},
	}

	subscriptionDifferentCSV = &olmv1alpha1.Subscription{
		ObjectMeta: v1.ObjectMeta{
			Name:      subName,
			Namespace: namespace,
			Labels: map[string]string{
				operatorTag: "true",
			},
		},
		Status: olmv1alpha1.SubscriptionStatus{
			InstalledCSV: "dummy",
		},
	}
)

func setup(r *ReconcilerTest) error {
	r.Client = fake.NewFakeClient(r.GetGetObjects()...)
	r.Reconciler = &ReconcileClusterServiceVersion{client: r.Client, scheme: scheme.Scheme}
	return nil
}

func testClusterServiceVersionWithInstalledCSV(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, clusterserviceversion, subscription)
	reconcilerTest.TestAll(t,
		ReconcileStep(opts,
			ReconcileWithExpectedResults(DoneResult)),
		ListStep(opts,
			ListWithObj(&olmv1alpha1.ClusterServiceVersionList{}),
			ListWithFilter(
				client.InNamespace(namespace),
				client.MatchingLabels(map[string]string{
					watchTag: "lite",
				})),
			ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
				list, ok := i.(*olmv1alpha1.ClusterServiceVersionList)

				assert.Truef(t, ok, "expected cluster service version list got type %T", i)
				assert.Equal(t, 1, len(list.Items))
			}),
		),
	)
}

func testClusterServiceVersionWithoutInstalledCSV(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, clusterserviceversion, subscriptionDifferentCSV)
	reconcilerTest.TestAll(t,
		ReconcileStep(nil,
			ReconcileWithExpectedResults(DoneResult)),
		ListStep(nil,
			ListWithObj(&olmv1alpha1.ClusterServiceVersionList{}),
			ListWithFilter(
				client.InNamespace(namespace),
				client.MatchingLabels(map[string]string{
					watchTag: "lite",
				})),
			ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
				list, ok := i.(*olmv1alpha1.ClusterServiceVersionList)

				assert.Truef(t, ok, "expected cluster service version list got type %T", i)
				assert.Equal(t, 0, len(list.Items))
			}),
		),
	)
}

func testClusterServiceVersionWithSubscriptionWithoutLabels(t *testing.T) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, clusterserviceversion, subscriptionWithoutLabels)
	reconcilerTest.TestAll(t,
		ReconcileStep(opts,
			ReconcileWithExpectedResults(DoneResult)),
		ListStep(opts,
			ListWithObj(&olmv1alpha1.ClusterServiceVersionList{}),
			ListWithFilter(
				client.InNamespace(namespace),
				client.MatchingLabels(map[string]string{
					watchTag: "lite",
				})),
			ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
				list, ok := i.(*olmv1alpha1.ClusterServiceVersionList)

				assert.Truef(t, ok, "expected cluster service version list got type %T", i)
				assert.Equal(t, 0, len(list.Items))
			}),
		),
	)
}

func TestBuildMeterDefinitionFromString(t *testing.T) {
	meter := &marketplacev1alpha1.MeterDefinition{}
	name := "example-meterdefinition"
	group := "partner.metering.com"
	version := "v1alpha"
	kind := "App"
	serviceMeters := []string{"labels"}
	podMeters := []string{"kube_pod_container_resource_requests"}
	var ann = map[string]string{
		utils.CSV_ANNOTATION_NAME:      csvName,
		utils.CSV_ANNOTATION_NAMESPACE: namespace,
	}

	ogMeter := &marketplacev1alpha1.MeterDefinition{
		ObjectMeta: v1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: ann,
		},
		Spec: marketplacev1alpha1.MeterDefinitionSpec{
			Group:         group,
			Version:       version,
			Kind:          kind,
			ServiceMeters: serviceMeters,
			PodMeters:     podMeters,
		},
	}

	meterStr := "{\"metadata\":{\"name\":\"" + name + "\",\"namespace\":\"" + namespace + "\",\"creationTimestamp\":null},\"spec\":{\"group\":\"" + group + "\",\"version\":\"" + version + "\",\"kind\":\"" + kind + "\",\"serviceMeters\":[\"labels\"],\"podMeters\":[\"kube_pod_container_resource_requests\"]},\"status\":{\"serviceLabels\":null,\"podLabels\":null,\"serviceMonitors\":null,\"pods\":null}}"
	_, err := meter.BuildMeterDefinitionFromString(meterStr, csvName, namespace, utils.CSV_ANNOTATION_NAME, utils.CSV_ANNOTATION_NAMESPACE)
	if err != nil {
		t.Errorf("Failed to build MeterDefinition CR: %v", err)
	} else {
		if !reflect.DeepEqual(meter, ogMeter) {
			fmt.Println("OG METER: ")
			fmt.Println(ogMeter)
			fmt.Println("CREATED METER: ")
			fmt.Println(meter)
			t.Errorf("Expected MeterDefinition is different from actual MeterDefinition")
		}
	}
}
