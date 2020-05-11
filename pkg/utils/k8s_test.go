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

package utils

import (
	"context"
	"testing"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/operator-framework/operator-sdk/pkg/status"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	marketplaceConfigName        = "rhm-marketplaceconfig"
	namespace                    = "redhat-marketplace-operator"
	customerID            string = "example-userid"
	testNamespace1               = "testing-namespace-1"

	marketplaceconfig = buildMarketplaceConfigCR(marketplaceConfigName, testNamespace1, customerID)
	razeedeployment   = BuildRazeeCr(testNamespace1, marketplaceconfig.Spec.ClusterUUID, marketplaceconfig.Spec.DeploySecretName)
	meterbase         = BuildMeterBaseCr(testNamespace1)
)

// setup returns a fakeClient for testing purposes
func setup() client.Client {
	defaultFeatures := []string{"razee", "meterbase"}
	viper.Set("assets", "../../../assets")
	viper.Set("features", defaultFeatures)
	objs := []runtime.Object{
		marketplaceconfig,
		razeedeployment,
		meterbase,
	}
	s := scheme.Scheme
	_ = monitoringv1.AddToScheme(s)
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, marketplaceconfig)
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, razeedeployment)
	s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, meterbase)
	client := fake.NewFakeClient(objs...)
	return client
}

func TestPersistenVolumeClaim(t *testing.T) {
	pvc, err := NewPersistentVolumeClaim(PersistentVolume{})

	if err != nil {
		t.Errorf("failed with error %v", err)
	}

	if len(pvc.Spec.AccessModes) == 0 {
		t.Error("no defined access modes")
	}

	if pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
		t.Errorf("expect %v but got %v", corev1.ReadWriteOnce, pvc.Spec.AccessModes[0])
	}

	val := corev1.ReadWriteMany
	pvc, err = NewPersistentVolumeClaim(PersistentVolume{
		AccessMode: &val,
	})

	if err != nil {
		t.Errorf("failed with error %v", err)
	}

	if len(pvc.Spec.AccessModes) == 0 {
		t.Error("no defined access modes")
	}

	if pvc.Spec.AccessModes[0] != corev1.ReadWriteMany {
		t.Errorf("expect %v but got %v", corev1.ReadWriteMany, pvc.Spec.AccessModes[0])
	}
}

func TestUpdateConfigConditionsRazee(t *testing.T) {

	client := setup()
	var err error

	message := "Razee Install starting"
	reason := marketplacev1alpha1.ReasonRazeeStartInstall
	if razeedeployment.Status.RazeeConditions.GetCondition(marketplacev1alpha1.ConditionInstalling) == nil {
		razeedeployment.Status.RazeeConditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionInstalling,
			Status:  corev1.ConditionTrue,
			Reason:  reason,
			Message: message,
		})

		err = client.Status().Update(context.TODO(), razeedeployment)
		if err != nil {
			t.Errorf("Error updating resources: %v", err)
		}
	}

	err = UpdateConfigConditions(client, razeedeployment, testNamespace1, message, reason)
	if err != nil {
		t.Errorf("Error updating MarketplaceConfig Conditions %v:", err)
	}

	err = client.Get(context.TODO(), types.NamespacedName{Name: marketplaceConfigName, Namespace: testNamespace1}, marketplaceconfig)
	if err != nil {
		t.Errorf("failed to get marketplaceconfig error %v:", err)
	}

	cond1 := razeedeployment.Status.RazeeConditions.GetCondition(marketplacev1alpha1.ConditionInstalling)
	cond2 := marketplaceconfig.Status.RazeeSubConditions.GetCondition(marketplacev1alpha1.ConditionInstalling)
	if cond2 == nil {
		t.Errorf("Error: RazeeSubConditions for MarketplaceConfig are not set")
	}
	if cond1.Message != cond2.Message {
		t.Errorf("Error: Message for RazeeCondition and RazeeSubConditions are not the same")
	}
}

func TestUpdateConfigConditionsMeterBase(t *testing.T) {

	client := setup()
	var err error

	message := "Meter Base install starting"
	reason := marketplacev1alpha1.ReasonMeterBaseStartInstall
	if meterbase.Status.MeterBaseConditions.GetCondition(marketplacev1alpha1.ConditionInstalling) == nil {
		meterbase.Status.MeterBaseConditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionInstalling,
			Status:  corev1.ConditionTrue,
			Reason:  reason,
			Message: message,
		})

		err = client.Status().Update(context.TODO(), meterbase)
		if err != nil {
			t.Errorf("Error updating resources: %v", err)
		}
	}

	err = UpdateConfigConditions(client, meterbase, testNamespace1, message, reason)
	if err != nil {
		t.Errorf("Error updating MarketplaceConfig Conditions %v:", err)
	}

	err = client.Get(context.TODO(), types.NamespacedName{Name: marketplaceConfigName, Namespace: testNamespace1}, marketplaceconfig)
	if err != nil {
		t.Errorf("failed to get marketplaceconfig error %v:", err)
	}

	cond1 := meterbase.Status.MeterBaseConditions.GetCondition(marketplacev1alpha1.ConditionInstalling)
	cond2 := marketplaceconfig.Status.MeterBaseSubConditions.GetCondition(marketplacev1alpha1.ConditionInstalling)
	if cond2 == nil {
		t.Errorf("Error: MeterBaseSubConditions for MarketplaceConfig are not set")
	}
	if cond1.Message != cond2.Message {
		t.Errorf("Error: Message for MeterBaseConditions and MeterBaseSubConditions are not the same")
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
