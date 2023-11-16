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
	"fmt"
	"os"
)

const (
	/* Resource Names */
	RHM_CONTROLLER_DEPLOYMENT_NAME         = "redhat-marketplace-controller-manager"
	RHM_METERING_DEPLOYMENT_NAME           = "ibm-metrics-operator-controller-manager"
	RHM_WATCHKEEPER_DEPLOYMENT_NAME        = "rhm-watch-keeper"
	RHM_REMOTE_RESOURCE_S3_DEPLOYMENT_NAME = "rhm-remoteresources3-controller"
	RHM_REMOTE_RESOURCE_DEPLOYMENT_NAME    = "rhm-remoteresource-controller"
	RAZEE_DEPLOY_JOB_NAME                  = "razeedeploy-job"
	RAZEE_JOB_NAME                         = "rhm-razeedeploy-job"
	PARENT_REMOTE_RESOURCE_NAME            = "parent"
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
	DATA_SERVICE_NAME                      = "rhm-data-service"
	METERBASE_PROMETHEUS_OPERATOR_NAME     = "prometheus-operator"
	METERBASE_PROMETHEUS_NAME              = "rhm-prometheus-meterbase"
	METERBASE_PROMETHEUS_SERVICE_NAME      = "rhm-prometheus-meterbase"
	OPERATOR_CERTS_CA_BUNDLE_NAME          = "serving-certs-ca-bundle"
	RHM_COS_UPLOADER_SECRET                = "rhm-cos-uploader-secret"
	METRICS_OP_METRICS_READER_SECRET       = "ibm-metrics-operator-servicemonitor-metrics-reader"
	METRICS_OP_CA_BUNDLE_CONFIGMAP         = "ibm-metrics-operator-serving-certs-ca-bundle"
	METRICS_OP_SERVICE_MONITOR             = "ibm-metrics-operator-controller-manager-metrics-monitor"
	METRICS_OP_METRICS_SERVICE             = "ibm-metrics-operator-controller-manager-metrics-service"
	RHM_OP_METRICS_READER_SECRET           = "redhat-marketplace-servicemonitor-metrics-reader"
	RHM_OP_CA_BUNDLE_CONFIGMAP             = "redhat-marketplace-serving-certs-ca-bundle"
	RHM_OP_SERVICE_MONITOR                 = "redhat-marketplace-controller-manager-metrics-monitor"
	RHM_OP_METRICS_SERVICE                 = "redhat-marketplace-controller-manager-metrics-service"

	/* RHOS Monitoring Resource Names */
	OPENSHIFT_MONITORING_NAMESPACE                              = "openshift-monitoring"
	OPENSHIFT_USER_WORKLOAD_MONITORING_NAMESPACE                = "openshift-user-workload-monitoring"
	OPENSHIFT_CLUSTER_MONITORING_CONFIGMAP_NAME                 = "cluster-monitoring-config"
	OPENSHIFT_USER_WORKLOAD_MONITORING_CONFIGMAP_NAME           = "user-workload-monitoring-config"
	OPENSHIFT_USER_WORKLOAD_MONITORING_STATEFULSET_NAME         = "prometheus-user-workload"
	OPENSHIFT_USER_WORKLOAD_MONITORING_SERVICE_NAME             = "prometheus-user-workload"
	OPENSHIFT_MONITORING_THANOS_QUERIER_SERVICE_NAME            = "thanos-querier"
	SERVING_CERTS_CA_BUNDLE_NAME                                = "ibm-metrics-operator-serving-certs-ca-bundle"
	KUBELET_SERVING_CA_BUNDLE_NAME                              = "kubelet-serving-ca-bundle"
	OPENSHIFT_USER_WORKLOAD_MONITORING_OPERATOR_SERVICE_ACCOUNT = "prometheus-operator"
	OPENSHIFT_USER_WORKLOAD_MONITORING_SERVICE_ACCOUNT          = "prometheus-user-workload"
	OPENSHIFT_USER_WORKLOAD_MONITORING_AUDIENCE                 = "prometheus-user-workload.openshift-user-workload-monitoring.svc"

	/* All Controllers */
	CONTROLLER_FINALIZER = "finalizer.marketplace.redhat.com"
	RR_FINALIZER         = "children.downloads.deploy.razee.io"

	/* RBAC */
	OPERATOR_SERVICE_ACCOUNT     = "ibm-metrics-operator-controller-manager"
	RAZEE_SERVICE_ACCOUNT        = "redhat-marketplace-remoteresources3deployment"
	METRIC_STATE_SERVICE_ACCOUNT = "ibm-metrics-operator-metric-state"
	REPORTING_SERVICE_ACCOUNT    = "ibm-metrics-operator-reporter"

	/* Razee Controller Values */
	RAZEE_DEPLOYMENT_FINALIZER                = "razeedeploy.finalizer.marketplace.redhat.com"
	DEFAULT_RHM_RRS3_DEPLOYMENT_IMAGE         = "us.icr.io/armada-master/remoteresource:2.1.19_7f655fe"
	DEFAULT_RHM_WATCH_KEEPER_DEPLOYMENT_IMAGE = "us.icr.io/armada-master/watch-keeper:0.8.10_7f655fe"
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

	RHMPullSecretName                = "redhat-marketplace-pull-secret"
	RHMOperatorSecretName            = "rhm-operator-secret"
	IBMEntitlementKeySecretName      = "ibm-entitlement-key"
	IBMEntitlementDataKey            = ".dockerconfigjson"
	IBMEntitlementProdKey            = "cp.icr.io"
	IBMEntitlementStageKey           = "stg.icr.io"
	IBMEntitlementKeyStatus          = "marketplace.redhat.com/ibm-entitlement-key"
	IBMEntitlementKeyMessage         = "marketplace.redhat.com/ibm-entitlement-key-message"
	IBMEntitlementKeyMissing         = "key with name '.dockerconfigjson' is missing in secret"
	IBMEntitlementKeyPasswordMissing = "password field is missing in entitlement key"
	RHMPullSecretKey                 = "PULL_SECRET"
	RHMPullSecretStatus              = "marketplace.redhat.com/rhm-operator-secret-status"
	RHMPullSecretMessage             = "marketplace.redhat.com/rhm-operator-secret-message"
	RHMPullSecretMissing             = "key with name 'PULL_SECRET' is missing in secret"
	ClusterDisplayNameKey            = "CLUSTER_DISPLAY_NAME"

	RazeeWatchResource    = "razee/watch-resource"
	RazeeWatchLevelLite   = "lite"
	RazeeWatchLevelDetail = "detail"

	LicenseServerTag = "marketplace.redhat.com/operator"
	OperatorTag      = "marketplace.redhat.com/operator"
	OperatorTagValue = "true"
	UninstallTag     = "marketplace.redhat.com/uninstall"

	/* Time and Date */
	DATE_FORMAT         = "2006-01-02"
	METER_REPORT_PREFIX = "meter-report-"

	/* Certificate */
	DQLITE_COMMONNAME_PREFIX = "*.rhm-data-service" // wildcard.ServiceName

	DeploymentConfigName = "rhm-meterdefinition-file-server"
	FileServerAudience   = "rhm-meterdefinition-file-server.openshift-redhat-marketplace.svc"
	ProductionURL        = "https://marketplace.redhat.com"
	StageURL             = "https://sandbox.marketplace.redhat.com"

	ProdEnv  = "production"
	StageEnv = "stage"

	UserWorkloadMonitoringMeterdef    = "prometheus-user-workload-uptime"
	MeterReportJobFailedMeterdef      = "rhm-meter-report-job-failed"
	MetricStateUptimeMeterdef         = "rhm-metric-state-uptime"
	PrometheusMeterbaseUptimeMeterdef = "rhm-prometheus-meterbase-uptime"

	/* Data Reporter */
	DATAREPORTERCONFIG_NAME   = "datareporterconfig"
	DATAREPORTER_SERVICE_NAME = "ibm-data-reporter-operator-controller-manager-metrics-service"
)

var (
	/* Metering Annotations */
	MeteredAnnotation = []string{"marketplace.redhat.com/metering", "true"}

	/* Labels*/
	LABEL_RHM_OPERATOR_WATCH = []string{"marketplace.redhat.com/watch", "true"}
)

func PrometheusAudience(ns string) string {
	return fmt.Sprintf("rhm-prometheus-meterbase.%s.svc", ns)
}
func DataServiceAudience(ns string) string {
	return fmt.Sprintf("rhm-data-service.%s.svc", ns)
}

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
