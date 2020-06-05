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
	"testing"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/spf13/viper"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	namespace             = "redhat-marketplace-operator"
	customerID     string = "example-userid"
	testNamespace1        = "testing-namespace-1"

	marketplaceconfig = BuildMarketplaceConfigCR(testNamespace1, customerID)
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
