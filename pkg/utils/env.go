package utils

import (
	"os"
)

const (
	MARKETPLACECONFIG_NAME    = "rhm-marketplaceconfig"
	METERBASE_NAME            = "rhm-marketplaceconfig-meterbase"
	RAZEE_NAME                = "rhm-marketplaceconfig-razeedeployment"
	CLUSTER_ROLE              = "redhat-marketplace-operator"
	CLUSTER_ROLE_BINDING      = "redhat-marketplace-operator"
	OPERATOR_SERVICE_ACCOUNT  = "redhat-marketplace-operator"
	RAZEE_SERVICE_ACCOUNT     = "redhat-marketplace-razeedeploy"
	METERBASE_SERVICE_ACCOUNT = "redhat-marketplace-metering"
	REPORTING_SERVICE_ACCOUNT = "redhat-marketplace-reporting"
	RAZEE_JOB_NAME            = "rhm-razeedeploy-job"
	OPSRC_NAME                = "redhat-marketplace-operators"
	OPERATOR_MKTPLACE_NS      = "openshift-marketplace"
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
