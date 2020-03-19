package utils

import (
	"os"
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
