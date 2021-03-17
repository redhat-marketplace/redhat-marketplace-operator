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
	RHM_WATCHKEEPER_DEPLOYMENT_NAME        = "rhm-watch-keeper"
	RHM_REMOTE_RESOURCE_S3_DEPLOYMENT_NAME = "rhm-remoteresources3-controller"
	RAZEE_DEPLOY_JOB_NAME                  = "razeedeploy-job"
	RAZEE_JOB_NAME                         = "rhm-razeedeploy-job"
	PARENT_RRS3_RESOURCE_NAME              = "parent"
	COS_READER_KEY_NAME                    = "rhm-cos-reader-key"
	RAZEE_UNINSTALL_NAME                   = "razee-uninstall-job"
	RHM_OPERATOR_SECRET_NAME               = "rhm-operator-secret"
	MARKETPLACECONFIG_NAME                 = "marketplaceconfig"
	METERBASE_NAME                         = "rhm-marketplaceconfig-meterbase"
	RAZEE_NAME                             = "rhm-marketplaceconfig-razeedeployment"
	OPSRC_NAME                             = "redhat-marketplace"
	IBM_CATALOGSRC_NAME                    = "ibm-operator-catalog"
	OPENCLOUD_CATALOGSRC_NAME              = "opencloud-operators"
	OPERATOR_MKTPLACE_NS                   = "openshift-marketplace"
	RAZEE_CLUSTER_METADATA_NAME            = "razee-cluster-metadata"
	WATCH_KEEPER_NON_NAMESPACED_NAME       = "watch-keeper-non-namespaced"
	WATCH_KEEPER_LIMITPOLL_NAME            = "watch-keeper-limit-poll"
	WATCH_KEEPER_CONFIG_NAME               = "watch-keeper-config"
	WATCH_KEEPER_SECRET_NAME               = "watch-keeper-secret"
	PROMETHEUS_METERBASE_NAME              = "rhm-prometheus-meterbase"
	OPERATOR_CERTS_CA_BUNDLE_NAME          = "operator-certs-ca-bundle"

	/* All Controllers */
	CONTROLLER_FINALIZER = "finalizer.marketplace.redhat.com"

	/* RBAC */
	CLUSTER_ROLE              = "redhat-marketplace-operator"
	CLUSTER_ROLE_BINDING      = "redhat-marketplace-operator"
	OPERATOR_SERVICE_ACCOUNT  = "redhat-marketplace-operator"
	RAZEE_SERVICE_ACCOUNT     = "redhat-marketplace-razeedeploy"
	METERBASE_SERVICE_ACCOUNT = "redhat-marketplace-metering"
	REPORTING_SERVICE_ACCOUNT = "redhat-marketplace-reporting"

	/* Razee Controller Values */
	RAZEE_DEPLOYMENT_FINALIZER                = "razeedeploy.finalizer.marketplace.redhat.com"
	DEFAULT_RHM_RRS3_DEPLOYMENT_IMAGE         = "quay.io/razee/remoteresources3:0.6.2"
	DEFAULT_RHM_WATCH_KEEPER_DEPLOYMENT_IMAGE = "quay.io/razee/watch-keeper:0.6.6"
	IBM_COS_READER_KEY_FIELD                  = "IBM_COS_READER_KEY"
	BUCKET_NAME_FIELD                         = "BUCKET_NAME"
	IBM_COS_URL_FIELD                         = "IBM_COS_URL"
	RAZEE_DASH_ORG_KEY_FIELD                  = "RAZEE_DASH_ORG_KEY"
	CHILD_RRS3_YAML_FIELD                     = "CHILD_RRS3_YAML_FILENAME"
	RAZEE_DASH_URL_FIELD                      = "RAZEE_DASH_URL"
	FILE_SOURCE_URL_FIELD                     = "FILE_SOURCE_URL"

	/* CSV Controller Values */
	CSV_FINALIZER                  = "csv.finalizer.marketplace.redhat.com"
	CSV_NAME                       = "redhat-marketplace-operator"
	CSV_ANNOTATION_NAME            = "csvName"
	CSV_ANNOTATION_NAMESPACE       = "csvNamespace"
	CSV_METERDEFINITION_ANNOTATION = "marketplace.redhat.com/meterDefinition"

	RHMPullSecretName     = "redhat-marketplace-pull-secret"
	RHMOperatorSecretName = "rhm-operator-secret"
	RHMPullSecretKey      = "PULL_SECRET"
	RHMPullSecretStatus   = "marketplace.redhat.com/rhm-operator-secret-status"
	RHMPullSecretMessage  = "marketplace.redhat.com/rhm-operator-secret-message"
	ClusterDisplayNameKey = "CLUSTER_DISPLAY_NAME"

	RazeeWatchResource    = "razee/watch-resource"
	RazeeWatchLevelLite   = "lite"
	RazeeWatchLevelDetail = "detail"

	LicenseServerTag = "marketplace.redhat.com/operator"

	/* Time and Date */
	DATE_FORMAT         = "2006-01-02"
	METER_REPORT_PREFIX = "meter-report-"

	/* Auth */
	PrometheusAudience = "rhm-prometheus-meterbase.openshift-redhat-marketplace.svc"
)

var (
	/* Metering Annotations */
	MeteredAnnotation = []string{"marketplace.redhat.com/metering", "true"}

	/* Labels*/
	LABEL_RHM_OPERATOR_WATCH = []string{"marketplace.redhat.com/watch", "true"}
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

func GetMapKeyValue(a []string) (string, string) {
	return a[0], a[1]
}

func SetMapKeyValue(inMap map[string]string, a []string) {
	key, value := GetMapKeyValue(a)
	inMap[key] = value
}

func HasMapKey(inMap map[string]string, a []string) bool {
	key, _ := GetMapKeyValue(a)
	_, ok := inMap[key]
	return ok
}
