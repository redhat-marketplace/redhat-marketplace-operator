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
	RAZEE_UNINSTALL_NAME      = "razee-uninstall-job"
	RAZEE_NAMESPACE           = "razee"
	RHM_OPERATOR_SECRET_NAME  = "rhm-operator-secret"
	METERBASE_SERVICE_ACCOUNT = "redhat-marketplace-metering"
	REPORTING_SERVICE_ACCOUNT = "redhat-marketplace-reporting"
	RAZEE_JOB_NAME            = "rhm-razeedeploy-job"
	OPSRC_NAME                = "redhat-marketplace"
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
