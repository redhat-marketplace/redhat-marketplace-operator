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
	/* Resource Names */
	RAZEE_DEPLOY_JOB_NAME     = "razeedeploy-job"
	RAZEE_JOB_NAME            = "rhm-razeedeploy-job"
	PARENT_RRS3_RESOURCE_NAME = "parent"
	COS_READER_KEY_NAME       = "rhm-cos-reader-key"
	RAZEE_UNINSTALL_NAME      = "razee-uninstall-job"
	RHM_OPERATOR_SECRET_NAME  = "rhm-operator-secret"
	MARKETPLACECONFIG_NAME    = "rhm-marketplaceconfig"
	METERBASE_NAME            = "rhm-marketplaceconfig-meterbase"
	RAZEE_NAME                = "rhm-marketplaceconfig-razeedeployment"
	OPSRC_NAME                = "redhat-marketplace"
	OPERATOR_MKTPLACE_NS      = "openshift-marketplace"

	/* RBAC */
	CLUSTER_ROLE              = "redhat-marketplace-operator"
	CLUSTER_ROLE_BINDING      = "redhat-marketplace-operator"
	OPERATOR_SERVICE_ACCOUNT  = "redhat-marketplace-operator"
	RAZEE_SERVICE_ACCOUNT     = "redhat-marketplace-razeedeploy"
	METERBASE_SERVICE_ACCOUNT = "redhat-marketplace-metering"
	REPORTING_SERVICE_ACCOUNT = "redhat-marketplace-reporting"

	/* Razee Controller Values */
	PARENT_RRS3                = "parentRRS3"
	RAZEE_DEPLOYMENT_FINALIZER = "razeedeploy.finalizer.marketplace.redhat.com"
	DEFAULT_RAZEE_JOB_IMAGE    = "quay.io/razee/razeedeploy-delta:1.1.0"
	WATCH_KEEPER_VERSION       = "0.5.0"
	FEATURE_FLAG_VERSION       = "0.6.1"
	MANAGED_SET_VERSION        = "0.4.2"
	MUSTACHE_TEMPLATE_VERSION  = "0.6.3"
	REMOTE_RESOURCE_VERSION    = "0.4.2"
	REMOTE_RESOURCE_S3_VERSION = "0.5.2"
	IBM_COS_READER_KEY_FIELD   = "IBM_COS_READER_KEY"
	BUCKET_NAME_FIELD          = "BUCKET_NAME"
	IBM_COS_URL_FIELD          = "IBM_COS_URL"
	RAZEE_DASH_ORG_KEY_FIELD   = "RAZEE_DASH_ORG_KEY"
	CHILD_RRS3_YAML_FIELD      = "CHILD_RRS3_YAML_FILENAME"
	RAZEE_DASH_URL_FIELD       = "RAZEE_DASH_URL"
	FILE_SOURCE_URL_FIELD      = "FILE_SOURCE_URL"
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