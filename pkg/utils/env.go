package utils

import (
	"os"
)

const (
	MARKETPLACECONFIG_NAME = "rhm-marketplaceconfig"
	METERBASE_NAME         = "rhm-marketplaceconfig-meterbase"
	RAZEE_NAME             = "rhm-marketplaceconfig-razeedeployment"
	RAZEE_JOB_NAME         = "rhm-razeedeploy-job"
	OPSRC_NAME             = "redhat-marketplace-operators"
	OPERATOR_MKTPLACE_NS   = "openshift-marketplace"
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
