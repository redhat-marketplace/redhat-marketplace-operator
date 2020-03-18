package utils

import (
	"os"

	opsrcv1 "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	marketplacev1alpha1 "github.ibm.com/symposium/marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MARKETPLACECONFIG_NAME = "example-marketplaceconfig"
	OPSRC_NAME             = "redhat-marketplace-operators"
	RAZEE_NAME             = "marketplaceconfig-razeedeployment"
	METERBASE_NAME         = "marketplaceconfig-meterbase"
	RAZEE_JOB_NAME         = "razeedeploy-job"
)

// Getenv will return the value for the passed key (which is typically an environment variable)
// If it is not found, return the fallback
func Getenv(key, fallback string) string {

	image, found := os.LookupEnv(key)
	if !found {
		return fallback
	}
	return image
}

// BuildNewOpSrc returns a new Operator Source
func BuildNewOpSrc(namespace string) *opsrcv1.OperatorSource {

	opsrc := &opsrcv1.OperatorSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OPSRC_NAME,
			Namespace: namespace,
		},
		Spec: opsrcv1.OperatorSourceSpec{
			DisplayName:       "Red Hat Marketplace",
			Endpoint:          "https://quay.io/cnr",
			Publisher:         "Red Hat Marketplace",
			RegistryNamespace: "redhat-marketplace",
			Type:              "appregistry",
		},
	}

	return opsrc
}

// BuildRazeeCrd returns a RazeeDeployment cr with default values
func BuildRazeeCr(namespace string) *marketplacev1alpha1.RazeeDeployment {

	cr := &marketplacev1alpha1.RazeeDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RAZEE_NAME,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.RazeeDeploymentSpec{
			Enabled: true,
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
