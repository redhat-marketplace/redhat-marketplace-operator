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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	emperrors "emperror.dev/errors"
	"github.com/gotidy/ptr"
	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8yaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/operrors"
	appsv1 "k8s.io/api/apps/v1"
)

type PersistentVolume struct {
	*metav1.ObjectMeta
	StorageClass *string
	StorageSize  *resource.Quantity
	AccessMode   *corev1.PersistentVolumeAccessMode
}

func NewPersistentVolumeClaim(values PersistentVolume) (corev1.PersistentVolumeClaim, error) {
	// set some defaults
	quantity := resource.MustParse("20Gi")
	accessMode := corev1.ReadWriteOnce
	defaults := PersistentVolume{
		ObjectMeta:   &metav1.ObjectMeta{},
		StorageClass: ptr.String(""),
		AccessMode:   &accessMode,
		StorageSize:  &quantity,
	}

	// merge values from pv into values
	if err := mergo.Merge(&values, defaults); err != nil {
		return corev1.PersistentVolumeClaim{}, err
	}

	return corev1.PersistentVolumeClaim{
		ObjectMeta: *values.ObjectMeta,
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				*values.AccessMode,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": *values.StorageSize,
				},
			},
			StorageClassName: values.StorageClass,
		},
	}, nil
}

// GetPodNames returns the pod names of the array of pods passed in
func GetPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// GetDefaultStorageClass attempts to return the default storage class
// of the cluster and errors if it cannot be found
func GetDefaultStorageClass(client client.Client) (string, error) {
	storageList := &storagev1.StorageClassList{}

	if err := client.List(context.TODO(), storageList); err != nil {
		return "", err
	}

	defaultStorageOptions := []string{}

	for _, storageClass := range storageList.Items {
		if storageClass.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			defaultStorageOptions = append(defaultStorageOptions, storageClass.Name)
		}
	}

	if len(defaultStorageOptions) == 0 {
		return "", emperrors.WithStack(operrors.DefaultStorageClassNotFound)
	}

	if len(defaultStorageOptions) > 1 {
		return "", emperrors.WithStack(operrors.MultipleDefaultStorageClassFound)
	}

	return defaultStorageOptions[0], nil
}

// MakeProbe creates a probe with the specified path and prot
func MakeProbe(path string, port, initialDelaySeconds, timeoutSeconds int32) *corev1.Probe {
	return &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: intstr.FromInt(int(port)),
			},
		},
		InitialDelaySeconds: initialDelaySeconds,
		TimeoutSeconds:      timeoutSeconds,
	}
}

// BuildMarketplaceConfigCR returns a new MarketplaceConfig
func BuildMarketplaceConfigCR(namespace, customerID string) *marketplacev1alpha1.MarketplaceConfig {
	return &marketplacev1alpha1.MarketplaceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MARKETPLACECONFIG_NAME,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.MarketplaceConfigSpec{
			RhmAccountID: customerID,
		},
	}
}

// BuildNewOpSrc returns a new Operator Source
func BuildNewOpSrc() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operators.coreos.com/v1",
			"kind":       "OperatorSource",
			"metadata": map[string]interface{}{
				"namespace": OPERATOR_MKTPLACE_NS,
				"name":      OPSRC_NAME,
			},
			"spec": map[string]interface{}{
				"DisplayName":       "Red Hat Marketplace",
				"Endpoint":          "https://quay.io/cnr",
				"Publisher":         "Red Hat Marketplace",
				"RegistryNamespace": "redhat-marketplace",
				"Type":              "appregistry",
			},
		},
	}
}

// BuildNewIBMCatalogSrc returns a new IBM Catalog Source
func BuildNewIBMCatalogSrc() *operatorsv1alpha1.CatalogSource {
	catalogSrc := &operatorsv1alpha1.CatalogSource{
		ObjectMeta: metav1.ObjectMeta{
			Name: IBM_CATALOGSRC_NAME,
			// Must always be openshift-marketplace
			Namespace: OPERATOR_MKTPLACE_NS,
		},
		Spec: operatorsv1alpha1.CatalogSourceSpec{
			DisplayName:    "IBM Operator Catalog",
			Publisher:      "IBM",
			SourceType:     "grpc",
			Image:          "docker.io/ibmcom/ibm-operator-catalog",
			UpdateStrategy: &operatorsv1alpha1.UpdateStrategy{RegistryPoll: &operatorsv1alpha1.RegistryPoll{Interval: &metav1.Duration{Duration: (time.Minute * 45)}}},
		},
	}

	return catalogSrc
}

// BuildNewOpencloudCatalogSrc returns a new Opencloud Catalog Source
func BuildNewOpencloudCatalogSrc() *operatorsv1alpha1.CatalogSource {
	catalogSrc := &operatorsv1alpha1.CatalogSource{
		ObjectMeta: metav1.ObjectMeta{
			Name: OPENCLOUD_CATALOGSRC_NAME,
			// Must always be openshift-marketplace
			Namespace: OPERATOR_MKTPLACE_NS,
		},
		Spec: operatorsv1alpha1.CatalogSourceSpec{
			DisplayName:    "IBMCS Operators",
			Publisher:      "IBM",
			SourceType:     "grpc",
			Image:          "docker.io/ibmcom/ibm-common-service-catalog",
			UpdateStrategy: &operatorsv1alpha1.UpdateStrategy{RegistryPoll: &operatorsv1alpha1.RegistryPoll{Interval: &metav1.Duration{Duration: (time.Minute * 45)}}},
		},
	}

	return catalogSrc
}

// BuildRazeeCrd returns a RazeeDeployment cr with default values
func BuildRazeeCr(namespace, clusterUUID string, deploySecretName *string, features *common.Features) *marketplacev1alpha1.RazeeDeployment {

	cr := &marketplacev1alpha1.RazeeDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RAZEE_NAME,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.RazeeDeploymentSpec{
			Enabled:          true,
			ClusterUUID:      clusterUUID,
			DeploySecretName: deploySecretName,
			Features:         features.DeepCopy(),
		},
	}

	return cr
}

// BuildMeterBaseCr returns a MeterBase cr with default values
func BuildMeterBaseCr(namespace string) *marketplacev1alpha1.MeterBase {
	cr := &marketplacev1alpha1.MeterBase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      METERBASE_NAME,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.MeterBaseSpec{
			Enabled: true,
			Prometheus: &marketplacev1alpha1.PrometheusSpec{
				Storage: marketplacev1alpha1.StorageSpec{
					Size: resource.MustParse("20Gi"),
				},
			},
		},
	}
	return cr
}

func ObjectToNamespaceNamed(obj runtime.Object) (types.NamespacedName, error) {
	key, err := client.ObjectKeyFromObject(obj)

	if err != nil {
		return types.NamespacedName{}, err
	}

	return key, nil
}

func LoadYAML(filename string, i interface{}) (interface{}, error) {
	dat, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	dec := k8yaml.NewYAMLOrJSONDecoder(bytes.NewReader(dat), 1000)
	var genericTypeVal interface{}

	switch v := i.(type) {
	case corev1.ConfigMap:
		genericTypeVal = &corev1.ConfigMap{}
	case monitoringv1.Prometheus:
		genericTypeVal = &monitoringv1.Prometheus{}
	default:
		return nil, fmt.Errorf("type not recognized %T", v)
	}

	if err := dec.Decode(&genericTypeVal); err != nil {
		return nil, err
	}

	return genericTypeVal, nil
}

// filterByNamespace returns the runtime.Object filtered by namespaces ListOptions
func FilterByNamespace(obj runtime.Object, namespaces []corev1.Namespace, rClient client.Client, options ...client.ListOption) error {
	var err error
	var listOpts []client.ListOption
	for _, opt := range options {
		listOpts = append(listOpts, opt)
	}

	if len(namespaces) == 0 {
		// if no namespaces are passed, return resources across all namespaces
		listOpts = append(listOpts, client.InNamespace(""))
		return getResources(obj, listOpts, rClient)

	}

	if len(namespaces) == 1 {
		//if passed a single namespace, return resources across that namespace
		listOpts = append(listOpts, client.InNamespace(namespaces[0].ObjectMeta.Name))
		err = getResources(obj, listOpts, rClient)
		return err
	}

	//if more than one namespaces is passed, loop through and append the resources
	switch listType := obj.(type) {
	case *corev1.PodList:
		for _, ns := range namespaces {
			temp := &corev1.PodList{}
			listOpts = append(listOpts, client.InNamespace(ns.ObjectMeta.Name))
			err = getResources(temp, listOpts, rClient)
			for _, i := range temp.Items {
				listType.Items = append(listType.Items, i)
			}
		}
	case *monitoringv1.ServiceMonitorList:
		for _, ns := range namespaces {
			temp := &monitoringv1.ServiceMonitorList{}
			listOpts = append(listOpts, client.InNamespace(ns.ObjectMeta.Name))
			err = getResources(temp, listOpts, rClient)
			for _, i := range temp.Items {
				listType.Items = append(listType.Items, i)
			}
		}
	case *corev1.ServiceList:
		for _, ns := range namespaces {
			temp := &corev1.ServiceList{}
			listOpts = append(listOpts, client.InNamespace(ns.ObjectMeta.Name))
			err = getResources(temp, listOpts, rClient)
			for _, i := range temp.Items {
				listType.Items = append(listType.Items, i)
			}
		}
	default:
		err = errors.New("type is not supported for filter aggregation")
	}
	return err
}

// getResources() is a helper function for FilterByNamespace(), it returns a the runtime.Object filled with resources from the requested namespaces
// the namespaces are preset in listOpts
func getResources(obj runtime.Object, listOpts []client.ListOption, rClient client.Client) error {

	err := rClient.List(context.TODO(), obj, listOpts...)
	if err != nil {
		return err
	}

	return nil

}

var eq = conversion.EqualitiesOrDie(
	func(a, b resource.Quantity) bool {
		// Ignore formatting, only care that numeric value stayed the same.
		// TODO: if we decide it's important, it should be safe to start comparing the format.
		//
		// Uninitialized quantities are equivalent to 0 quantities.
		return a.Cmp(b) == 0
	},
	func(a, b metav1.MicroTime) bool {
		return a.UTC() == b.UTC()
	},
	func(a, b metav1.Time) bool {
		return a.UTC() == b.UTC()
	},
	func(a, b labels.Selector) bool {
		return a.String() == b.String()
	},
	func(a, b fields.Selector) bool {
		return a.String() == b.String()
	},
	func(a, b corev1.ObjectFieldSelector) bool {
		if a.APIVersion == "" {
			a.APIVersion = "v1"
		}
		if b.APIVersion == "" {
			b.APIVersion = "v1"
		}
		return a.APIVersion == b.APIVersion && a.FieldPath == b.FieldPath
	},
)

func DeploymentNeedsUpdate(original, updated appsv1.Deployment) (bool, string) {
	if !eq.DeepEqual(updated.Labels, original.Labels) {
		return true, "labels"
	}

	if !eq.DeepEqual(updated.Annotations, original.Annotations) {
		return true, "annotations"
	}

	if !eq.DeepEqual(updated.Spec.Strategy, original.Spec.Strategy) {
		return true, "strategy"
	}

	if !eq.DeepEqual(updated.ObjectMeta.OwnerReferences, original.ObjectMeta.OwnerReferences) {
		return true, "owner"
	}

	if !eq.DeepEqual(original.Spec.Template.Spec.Volumes, updated.Spec.Template.Spec.Volumes) {
		return true, "spec.volumes"
	}

	if !eq.DeepEqual(original.Spec.Template.Spec.Containers, updated.Spec.Template.Spec.Containers) {
		return true, "spec.containers"
	}

	//return !eq.DeepEqual(original.Spec, updated.Spec), "spec"
	return false, ""
}
