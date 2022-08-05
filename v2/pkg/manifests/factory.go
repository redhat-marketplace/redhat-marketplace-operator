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

package manifests

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	mathrand "math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/gotidy/ptr"
	osappsv1 "github.com/openshift/api/apps/v1"
	osimagev1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/assets"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/envvar"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	PrometheusOperatorDeploymentV46 = "prometheus-operator/deployment-v4.6.yaml"
	PrometheusOperatorServiceV46    = "prometheus-operator/service-v4.6.yaml"

	PrometheusAdditionalScrapeConfig = "prometheus/additional-scrape-configs.yaml"
	PrometheusHtpasswd               = "prometheus/htpasswd-secret.yaml"
	PrometheusRBACProxySecret        = "prometheus/kube-rbac-proxy-secret.yaml"
	PrometheusDeploymentV46          = "prometheus/prometheus-v4.6.yaml"
	PrometheusProxySecret            = "prometheus/proxy-secret.yaml"
	PrometheusService                = "prometheus/service.yaml"
	PrometheusDatasourcesSecret      = "prometheus/prometheus-datasources-secret.yaml"
	PrometheusServingCertsCABundle   = "prometheus/serving-certs-ca-bundle.yaml"
	PrometheusKubeletServingCABundle = "prometheus/kubelet-serving-ca-bundle.yaml"
	PrometheusServiceMonitor         = "prometheus/service-monitor.yaml"
	PrometheusMeterDefinition        = "prometheus/meterdefinition.yaml"

	ReporterJob             = "reporter/job.yaml"
	ReporterCronJob         = "reporter/cronjob.yaml"
	ReporterMeterDefinition = "reporter/meterdefinition.yaml"

	MetricStateDeployment        = "metric-state/deployment.yaml"
	MetricStateServiceMonitorV46 = "metric-state/service-monitor-v4.6.yaml"
	MetricStateService           = "metric-state/service.yaml"
	MetricStateMeterDefinition   = "metric-state/meterdefinition.yaml"

	// ose-prometheus v4.6
	MetricStateRHMOperatorSecret   = "metric-state/secret.yaml"
	KubeStateMetricsService        = "metric-state/kube-state-metrics-service.yaml"
	KubeStateMetricsServiceMonitor = "metric-state/kube-state-metrics-service-monitor.yaml"
	KubeletServiceMonitor          = "metric-state/kubelet-service-monitor.yaml"

	UserWorkloadMonitoringServiceMonitor  = "prometheus/user-workload-monitoring-service-monitor.yaml"
	UserWorkloadMonitoringMeterDefinition = "prometheus/user-workload-monitoring-meterdefinition.yaml"

	RRS3ControllerDeployment = "razee/rrs3-controller-deployment.yaml"
	WatchKeeperDeployment    = "razee/watch-keeper-deployment.yaml"
	DataServiceStatefulSet   = "dataservice/statefulset.yaml"
	DataServiceService       = "dataservice/service.yaml"
	DataServiceRoute         = "dataservice/route.yaml"
	DataServiceTLSSecret     = "dataservice/secret.yaml"

	MeterdefinitionFileServerDeploymentConfig = "catalog-server/deployment-config.yaml"
	MeterdefinitionFileServerService          = "catalog-server/service.yaml"
	MeterdefinitionFileServerImageStream      = "catalog-server/image-stream.yaml"
)

var log = logf.Log.WithName("manifests_factory")

var MustReadFileAsset = assets.MustReadFileAsset
var MustAssetReader = assets.MustAssetReader

type Factory struct {
	namespace      string
	config         *Config
	operatorConfig *config.OperatorConfig
	scheme         *runtime.Scheme
}

func NewFactory(
	oc *config.OperatorConfig,
	s *runtime.Scheme,
) *Factory {
	c := NewOperatorConfig(oc)
	return &Factory{
		namespace:      oc.DeployedNamespace,
		operatorConfig: oc,
		config:         c,
		scheme:         s,
	}
}

func find(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

func (f *Factory) ReplaceImages(container *corev1.Container) error {
	envChanges := envvar.Changes{}

	if err := f.updateContainerResources(container); err != nil {
		return err
	}

	envChanges.Append(envvar.AddHttpsProxy())

	switch {
	case strings.HasPrefix(container.Name, "kube-rbac-proxy"):
		container.Image = f.config.RelatedImages.KubeRbacProxy
	case container.Name == "metric-state":
		container.Image = f.config.RelatedImages.MetricState
	case container.Name == "authcheck":
		container.Image = f.config.RelatedImages.AuthChecker
		container.Args = []string{}
		container.LivenessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(28088),
				},
			},
			TimeoutSeconds:   5,
			PeriodSeconds:    30,
			FailureThreshold: 3,
		}
		container.ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/readyz",
					Port: intstr.FromInt(28088),
				},
			},
			TimeoutSeconds:      5,
			InitialDelaySeconds: 15,
			PeriodSeconds:       30,
			FailureThreshold:    3,
		}

		envChanges.Append(addPodName)
	case container.Name == "reporter":
		container.Image = f.config.RelatedImages.Reporter
	case container.Name == "prometheus-operator":
		container.Image = f.config.RelatedImages.PrometheusOperator
	case container.Name == "prometheus-proxy":
		container.Image = f.config.RelatedImages.OAuthProxy
	case container.Name == "rhm-data-service":
		container.Image = f.config.RelatedImages.DQLite
	case container.Name == "rhm-meterdefinition-file-server":
		container.Image = f.config.RelatedImages.MeterDefFileServer
	case container.Name == utils.RHM_REMOTE_RESOURCE_S3_DEPLOYMENT_NAME:
		container.Image = f.config.RelatedImages.RemoteResourceS3

		// watch-keeper and rrs3 doesn't use HTTPS_PROXY correctly
		// will fail; HTTP_PROXY will be used instead
		envChanges.Remove(corev1.EnvVar{
			Name: "HTTPS_PROXY",
		})
	case container.Name == "watch-keeper":
		container.Image = f.config.RelatedImages.WatchKeeper

		// watch-keeper and rrs3 doesn't use HTTPS_PROXY correctly
		// will fail; HTTP_PROXY will be used instead
		envChanges.Remove(corev1.EnvVar{
			Name: "HTTPS_PROXY",
		})
	}

	envChanges.Merge(container)
	return nil
}

func (f *Factory) UpdateEnvVar(container *corev1.Container, isDisconnected bool) {
	envChanges := envvar.Changes{}
	isDisconnectedEnvVar := envvar.Changes{
		envvar.Add(
			corev1.EnvVar{
				Name:  "IS_DISCONNECTED",
				Value: strconv.FormatBool(isDisconnected),
			},
		),
	}

	envChanges.Append(isDisconnectedEnvVar)
	envChanges.Merge(container)
}

var (
	addPodName = envvar.Changes{
		envvar.Add(corev1.EnvVar{
			Name: "POD_NAME",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		}),
		envvar.Add(corev1.EnvVar{
			Name: "POD_NAMESPACE",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		}),
	}
)

func (f *Factory) updateContainerResources(container *corev1.Container) error {
	if f.operatorConfig == nil || f.operatorConfig.Config.Resources == nil {
		return nil
	}

	if len(f.operatorConfig.Config.Resources.Containers) == 0 {
		return nil
	}

	if r, ok := f.operatorConfig.Config.Resources.Containers[container.Name]; ok {
		for k, v := range r.Limits {
			container.Resources.Limits[k] = v
		}

		for k, v := range r.Requests {
			container.Resources.Requests[k] = v
		}
	}

	return nil
}

func (f *Factory) NewDeployment(manifest io.Reader) (*appsv1.Deployment, error) {
	d, err := NewDeployment(manifest)
	if err != nil {
		return nil, err
	}

	if d.GetNamespace() == "" {
		d.SetNamespace(f.namespace)
	}

	if d.GetAnnotations() == nil {
		d.Annotations = make(map[string]string)
	}

	if d.Spec.Template.GetAnnotations() == nil {
		d.Spec.Template.Annotations = make(map[string]string)
	}

	maxSurge := intstr.FromString("25%")
	maxUnavailable := intstr.FromString("25%")

	d.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDeployment{
			MaxSurge:       &maxSurge,
			MaxUnavailable: &maxUnavailable,
		},
	}

	return d, nil
}

func (f *Factory) NewStatefulSet(manifest io.Reader) (*appsv1.StatefulSet, error) {
	d, err := NewStatefulSet(manifest)
	if err != nil {
		return nil, err
	}

	if d.GetNamespace() == "" {
		d.SetNamespace(f.namespace)
	}

	if d.GetAnnotations() == nil {
		d.Annotations = make(map[string]string)
	}

	if d.Spec.Template.GetAnnotations() == nil {
		d.Spec.Template.Annotations = make(map[string]string)
	}

	return d, nil
}

func (f *Factory) NewService(manifest io.Reader) (*corev1.Service, error) {
	d, err := NewService(manifest)
	if err != nil {
		return nil, err
	}

	if d.GetNamespace() == "" {
		d.SetNamespace(f.namespace)
	}

	return d, nil
}

func (f *Factory) NewConfigMap(manifest io.Reader) (*corev1.ConfigMap, error) {
	d, err := NewConfigMap(manifest)
	if err != nil {
		return nil, err
	}

	if d.GetNamespace() == "" {
		d.SetNamespace(f.namespace)
	}

	return d, nil
}

func (f *Factory) NewSecret(manifest io.Reader) (*v1.Secret, error) {
	s, err := NewSecret(manifest)
	if err != nil {
		return nil, err
	}

	if s.GetNamespace() == "" {
		s.SetNamespace(f.namespace)
	}

	return s, nil
}

func (f *Factory) NewJob(manifest io.Reader) (*batchv1.Job, error) {
	j, err := NewJob(manifest)
	if err != nil {
		return nil, err
	}

	if j.GetNamespace() == "" {
		j.SetNamespace(f.namespace)
	}

	return j, nil
}

func (f *Factory) NewImageStream(manifest io.Reader) (*osimagev1.ImageStream, error) {
	is, err := NewImageStream(manifest)
	if err != nil {
		return nil, err
	}

	if is.GetNamespace() == "" {
		is.SetNamespace(f.namespace)
	}

	if is.GetAnnotations() == nil {
		is.Annotations = make(map[string]string)
	}

	f.ReplaceImageStreamValues(is)

	return is, nil
}

func (f *Factory) NewDeploymentConfig(manifest io.Reader) (*osappsv1.DeploymentConfig, error) {
	d, err := NewDeploymentConfig(manifest)
	if err != nil {
		return nil, err
	}

	if d.GetNamespace() == "" {
		d.SetNamespace(f.namespace)
	}

	if d.GetAnnotations() == nil {
		d.Annotations = make(map[string]string)
	}

	if d.Spec.Template.GetAnnotations() == nil {
		d.Spec.Template.Annotations = make(map[string]string)
	}

	for i := range d.Spec.Template.Spec.Containers {
		f.ReplaceImages(&d.Spec.Template.Spec.Containers[i])
	}

	f.ReplaceDeploymentConfigValues(d)

	return d, nil
}

func (f *Factory) ReplaceDeploymentConfigValues(dc *osappsv1.DeploymentConfig) {
	triggers := osappsv1.DeploymentTriggerPolicies(dc.Spec.Triggers)
	trigger := triggers[1]

	trigger.ImageChangeParams.From.Name = f.operatorConfig.ImageStreamID
}

func (f *Factory) ReplaceImageStreamValues(is *osimagev1.ImageStream) {
	is.Spec.Tags[0].Annotations["openshift.io/imported-from"] = f.config.RelatedImages.MeterDefFileServer
	is.Spec.Tags[0].From.Name = f.config.RelatedImages.MeterDefFileServer
	is.Spec.Tags[0].Name = f.operatorConfig.ImageStreamTag
}

func (f *Factory) UpdateDeploymentConfigOnChange(clusterDC *osappsv1.DeploymentConfig) (updated bool) {
	logger := log.WithValues("func", "UpdateDeploymentConfigOnChange")

	triggers := osappsv1.DeploymentTriggerPolicies(clusterDC.Spec.Triggers)
	trigger := triggers[0]

	if trigger.ImageChangeParams != nil {
		if trigger.ImageChangeParams.From.Name != f.operatorConfig.ImageStreamID {
			logger.Info("DeploymentConfig docker image reference needs to be updated")
			logger.Info("ImageStreamID found on cluster", "imagestream ID", trigger.ImageChangeParams.From.Name)
			logger.Info("ImageStreamID found in config", "imagestream ID", f.operatorConfig.ImageStreamID)
			trigger.ImageChangeParams.From.Name = f.operatorConfig.ImageStreamID
			updated = true
		}
	}

	return updated
}

func (f *Factory) UpdateImageStreamOnChange(clusterIS *osimagev1.ImageStream) (updated bool) {
	logger := log.WithValues("func", "UpdateImageStreamOnChange")
	for _, tag := range clusterIS.Spec.Tags {
		if tag.From.Name != f.config.RelatedImages.MeterDefFileServer {
			logger.Info("ImageStream docker image reference needs to be updated")
			logger.Info("Docker image reference found on cluster", "image", tag.From.Name)
			logger.Info("Docker image reference found in config", "image", f.config.RelatedImages.MeterDefFileServer)
			tag.From.Name = f.config.RelatedImages.MeterDefFileServer
			updated = true
		}

		if tag.Name != f.operatorConfig.ImageStreamTag {
			logger.Info("ImageStream tag needs to be updated")
			logger.Info("ImageStream tag found on cluster", "tag", tag.Name)
			logger.Info("ImageStream tag found in config", "tag", f.operatorConfig.ImageStreamTag)
			tag.Name = f.operatorConfig.ImageStreamTag
			updated = true
		}

		if tag.Annotations["openshift.io/imported-from"] != f.config.RelatedImages.MeterDefFileServer {
			logger.Info("ImageStream imported-from annotation needs to be updated")
			logger.Info("ImageStream imported-from annotation on cluster", "value", tag.Annotations["openshift.io/imported-from"])
			logger.Info("ImageStream imported-from annotation in config", "value", f.config.RelatedImages.MeterDefFileServer)
			tag.Annotations["openshift.io/imported-from"] = f.config.RelatedImages.MeterDefFileServer
			updated = true
		}
	}

	return updated
}

func (f *Factory) NewCronJob(manifest io.Reader) (*batchv1beta1.CronJob, error) {
	j, err := NewCronJob(manifest)
	if err != nil {
		return nil, err
	}

	if j.GetNamespace() == "" {
		j.SetNamespace(f.namespace)
	}

	return j, nil
}

type dataServiceRef struct {
	Service, Namespace, PortName string
}

func (d *dataServiceRef) ToPrometheusArgs() []string {
	return []string{
		fmt.Sprintf("--prometheus-service=%s", d.Service),
		fmt.Sprintf("--prometheus-namespace=%s", d.Namespace),
		fmt.Sprintf("--prometheus-port=%s", d.PortName),
	}
}

var (
	marketplacePrometheus = &dataServiceRef{
		Service:  "rhm-prometheus-meterbase",
		PortName: "rbac",
	}
	thanosQuerier = &dataServiceRef{
		Service:   "thanos-querier",
		Namespace: "openshift-monitoring",
		PortName:  "web",
	}
)

func (f *Factory) NewReporterCronJob(userWorkloadEnabled bool, isDisconnected bool) (*batchv1beta1.CronJob, error) {
	j, err := f.NewCronJob(MustAssetReader(ReporterCronJob))
	if err != nil {
		return nil, err
	}

	if j.Spec.Schedule == "" {
		j.Spec.Schedule = fmt.Sprintf("%v * * * *", mathrand.Intn(15))
	}

	j.Spec.JobTemplate.Spec.BackoffLimit = f.operatorConfig.ReportController.RetryLimit
	container := &j.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	f.ReplaceImages(container)

	f.UpdateEnvVar(container, isDisconnected)

	dataServiceArgs := []string{
		"--dataServiceCertFile=/etc/configmaps/serving-certs-ca-bundle/service-ca.crt",
		"--dataServiceTokenFile=/etc/data-service-sa/data-service-token",
		"--cafile=/etc/configmaps/serving-certs-ca-bundle/service-ca.crt",
	}

	if userWorkloadEnabled {
		dataServiceArgs = append(dataServiceArgs, thanosQuerier.ToPrometheusArgs()...)
	} else {
		ref := marketplacePrometheus
		ref.Namespace = f.namespace
		dataServiceArgs = append(dataServiceArgs, ref.ToPrometheusArgs()...)
	}

	container.Args = append(container.Args, "--namespace", f.namespace)
	container.Args = append(container.Args, dataServiceArgs...)

	if len(f.operatorConfig.ReportController.UploadTargetsOverride) != 0 {
		container.Args = append(container.Args, "--uploadTargets", strings.Join(f.operatorConfig.ReportController.UploadTargetsOverride, ","))
	}

	if f.operatorConfig.ReportController.ReporterSchema != "" {
		container.Args = append(container.Args, "--reporterSchema", f.operatorConfig.ReportController.ReporterSchema)
	}

	dataServiceVolumeMounts := []v1.VolumeMount{
		{
			Name:      "serving-certs-ca-bundle",
			MountPath: "/etc/configmaps/serving-certs-ca-bundle",
			ReadOnly:  false,
		},
		{
			Name:      "data-service-token-vol",
			ReadOnly:  true,
			MountPath: "/etc/data-service-sa",
		},
	}

	container.VolumeMounts = append(container.VolumeMounts, dataServiceVolumeMounts...)

	dataServiceTokenVols := []v1.Volume{
		{
			Name: "serving-certs-ca-bundle",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "serving-certs-ca-bundle",
					},
				},
			},
		},
		{
			Name: "data-service-token-vol",
			VolumeSource: v1.VolumeSource{
				Projected: &v1.ProjectedVolumeSource{
					Sources: []v1.VolumeProjection{
						{
							ServiceAccountToken: &v1.ServiceAccountTokenProjection{
								Audience:          utils.DataServiceAudience(f.namespace),
								ExpirationSeconds: ptr.Int64(3600),
								Path:              "data-service-token",
							},
						},
					},
				},
			},
		},
	}

	volumes := &j.Spec.JobTemplate.Spec.Template.Spec.Volumes
	*volumes = append(*volumes, dataServiceTokenVols...)

	// Keep last 3 days of data
	j.Spec.JobTemplate.Spec.TTLSecondsAfterFinished = ptr.Int32(86400 * 3)

	return j, nil
}

func (f *Factory) NewRoute(manifest io.Reader) (*routev1.Route, error) {
	r, err := NewRoute(manifest)
	if err != nil {
		return nil, err
	}

	if r.GetNamespace() == "" {
		r.SetNamespace(f.namespace)
	}

	return r, nil
}

func (f *Factory) UpdateRoute(manifest io.Reader, r *routev1.Route) error {
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(r)
	if err != nil {
		return err
	}

	return nil
}

func (f *Factory) NewPrometheus(
	manifest io.Reader,
) (*monitoringv1.Prometheus, error) {
	p, err := NewPrometheus(manifest)
	if err != nil {
		return nil, err
	}

	if p.GetNamespace() == "" {
		p.SetNamespace(f.namespace)
	}

	return p, nil
}

func (f *Factory) NewMeterDefinition(
	manifest io.Reader,
) (*marketplacev1beta1.MeterDefinition, error) {
	m, err := NewMeterDefinition(manifest)
	if err != nil {
		return nil, err
	}

	if m.GetNamespace() == "" {
		m.SetNamespace(f.namespace)
	}

	return m, nil
}

func (f *Factory) PrometheusService(instanceName string) (*v1.Service, error) {
	s, err := f.NewService(MustAssetReader(PrometheusService))
	if err != nil {
		return nil, err
	}

	s.Namespace = f.namespace

	s.Labels["app"] = "prometheus"
	s.Labels["prometheus"] = instanceName

	s.Spec.Selector["prometheus"] = instanceName

	return s, nil
}

func (f *Factory) PrometheusRBACProxySecret() (*v1.Secret, error) {
	s, err := f.NewSecret(MustAssetReader(PrometheusRBACProxySecret))
	if err != nil {
		return nil, err
	}

	s.Namespace = f.namespace

	return s, nil
}

func (f *Factory) PrometheusProxySecret() (*v1.Secret, error) {
	s, err := f.NewSecret(MustAssetReader(PrometheusProxySecret))
	if err != nil {
		return nil, err
	}

	p, err := GeneratePassword(43)
	if err != nil {
		return nil, err
	}
	s.Data["session_secret"] = []byte(p)
	s.Namespace = f.namespace

	return s, nil
}

func (f *Factory) PrometheusAdditionalConfigSecret(data []byte) (*v1.Secret, error) {
	s, err := f.NewSecret(MustAssetReader(PrometheusAdditionalScrapeConfig))
	if err != nil {
		return nil, err
	}

	s.Data["meterdef.yaml"] = data
	s.Namespace = f.namespace

	return s, nil
}

func (f *Factory) prometheusOperatorDeployment() string {
	return PrometheusOperatorDeploymentV46
}

func (f *Factory) NewPrometheusOperatorDeployment(ns []string) (*appsv1.Deployment, error) {
	c := f.config.PrometheusOperatorConfig
	dep, err := f.NewDeployment(MustAssetReader(f.prometheusOperatorDeployment()))

	if len(c.NodeSelector) > 0 {
		dep.Spec.Template.Spec.NodeSelector = c.NodeSelector
	}

	if len(c.Tolerations) > 0 {
		dep.Spec.Template.Spec.Tolerations = c.Tolerations
	}

	if c.ServiceAccountName != "" {
		dep.Spec.Template.Spec.ServiceAccountName = c.ServiceAccountName
	}

	replacer := strings.NewReplacer(
		"{{NAMESPACE}}", f.namespace,
		"{{NAMESPACES}}", strings.Join(ns, ","),
		"{{CONFIGMAP_RELOADER_IMAGE}}", f.config.RelatedImages.ConfigMapReloader,
		"{{PROM_CONFIGMAP_RELOADER_IMAGE}}", f.config.RelatedImages.PrometheusConfigMapReloader,
	)

	for i := range dep.Spec.Template.Spec.Containers {
		container := &dep.Spec.Template.Spec.Containers[i]
		newArgs := []string{}

		err := f.ReplaceImages(container)

		if err != nil {
			return nil, err
		}

		for _, arg := range container.Args {
			newArg := replacer.Replace(arg)
			newArgs = append(newArgs, newArg)
		}

		container.Args = newArgs
	}

	return dep, err
}

func (f *Factory) prometheusDeployment() string {
	return PrometheusDeploymentV46
}

func (f *Factory) NewPrometheusDeployment(
	cr *marketplacev1alpha1.MeterBase,
	cfg *corev1.Secret,
) (*monitoringv1.Prometheus, error) {
	logger := log.WithValues("func", "NewPrometheusDeployment")
	p, err := f.NewPrometheus(MustAssetReader(f.prometheusDeployment()))

	if err != nil {
		logger.Error(err, "failed to read the file")
		return p, err
	}

	p.Name = cr.Name
	p.ObjectMeta.Name = cr.Name

	p.Spec.Image = &f.config.RelatedImages.Prometheus

	if cr.Spec.Prometheus != nil && cr.Spec.Prometheus.Replicas != nil {
		p.Spec.Replicas = cr.Spec.Prometheus.Replicas
	}

	if f.config.PrometheusConfig.Retention != "" {
		p.Spec.Retention = f.config.PrometheusConfig.Retention
	}

	//Set empty dir if present in the CR, will override a pvc specified (per prometheus docs)
	if cr.Spec.Prometheus != nil && cr.Spec.Prometheus.Storage.EmptyDir != nil {
		p.Spec.Storage.EmptyDir = cr.Spec.Prometheus.Storage.EmptyDir
	}

	storageClass := ptr.String("")
	if cr.Spec.Prometheus != nil && cr.Spec.Prometheus.Storage.Class != nil {
		storageClass = cr.Spec.Prometheus.Storage.Class
	}

	if cr.Spec.Prometheus != nil {
		quanBytes := cr.Spec.Prometheus.Storage.Size.DeepCopy()
		quanBytes.Sub(resource.MustParse("2Gi"))
		replacer := strings.NewReplacer("Mi", "MB", "Gi", "GB", "Ti", "TB")
		storageSize := replacer.Replace(quanBytes.String())
		p.Spec.RetentionSize = storageSize

		pvc, _ := utils.NewPersistentVolumeClaim(utils.PersistentVolume{
			ObjectMeta: &metav1.ObjectMeta{
				Name: "storage-volume",
			},
			StorageClass: storageClass,
			StorageSize:  &cr.Spec.Prometheus.Storage.Size,
		})

		p.Spec.Storage.VolumeClaimTemplate = monitoringv1.EmbeddedPersistentVolumeClaim{
			Spec: pvc.Spec,
		}
	}
	if cfg != nil {
		p.Spec.AdditionalScrapeConfigs = &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: cfg.GetName(),
			},
			Key: "meterdef.yaml",
		}
	}

	for i := range p.Spec.Containers {
		container := &p.Spec.Containers[i]
		err := f.ReplaceImages(container)

		if err != nil {
			return nil, err
		}
	}

	return p, err
}

func (f *Factory) prometheusOperatorService() string {
	return PrometheusOperatorServiceV46
}

func (f *Factory) NewPrometheusOperatorService() (*corev1.Service, error) {
	service, err := f.NewService(MustAssetReader(f.prometheusOperatorService()))

	return service, err
}

func (f *Factory) PrometheusKubeletServingCABundle(data string) (*v1.ConfigMap, error) {
	c, err := f.NewConfigMap(MustAssetReader(PrometheusKubeletServingCABundle))
	if err != nil {
		return nil, err
	}

	c.Namespace = f.namespace
	c.Data = map[string]string{
		"ca-bundle.crt": data,
	}

	return c, nil
}

func (f *Factory) PrometheusDatasources() (*v1.Secret, error) {
	s, err := f.NewSecret(MustAssetReader(PrometheusDatasourcesSecret))
	if err != nil {
		return nil, err
	}

	secret, err := GeneratePassword(255)

	if err != nil {
		return nil, err
	}

	if s.Data == nil {
		s.Data = make(map[string][]byte)
	}

	s.Data["basicAuthSecret"] = []byte(secret)

	s.Namespace = f.namespace

	return s, nil
}

func (f *Factory) PrometheusHtpasswdSecret(password string) (*v1.Secret, error) {
	s, err := f.NewSecret(MustAssetReader(PrometheusHtpasswd))
	if err != nil {
		return nil, err
	}

	f.generateHtpasswdSecret(s, password)
	return s, nil
}

func (f *Factory) generateHtpasswdSecret(s *v1.Secret, password string) {
	h := sha1.New()
	h.Write([]byte(password))
	s.Data["auth"] = []byte("internal:{SHA}" + base64.StdEncoding.EncodeToString(h.Sum(nil)))
	s.Namespace = f.namespace
}

func (f *Factory) PrometheusServingCertsCABundle() (*v1.ConfigMap, error) {
	c, err := f.NewConfigMap(MustAssetReader(PrometheusServingCertsCABundle))
	if err != nil {
		return nil, err
	}

	c.Namespace = f.namespace

	return c, nil
}

func (f *Factory) PrometheusMeterDefinition() (*marketplacev1beta1.MeterDefinition, error) {
	m, err := f.NewMeterDefinition(MustAssetReader(PrometheusMeterDefinition))
	if err != nil {
		return nil, err
	}

	m.Namespace = f.namespace

	return m, nil
}

func (f *Factory) PrometheusServiceMonitor() (*monitoringv1.ServiceMonitor, error) {
	sm, err := f.NewServiceMonitor(MustAssetReader(PrometheusServiceMonitor))
	if err != nil {
		return nil, err
	}

	sm.Spec.Endpoints[0].TLSConfig.ServerName = fmt.Sprintf("rhm-prometheus-meterbase.%s.svc", f.namespace)
	sm.Namespace = f.namespace

	return sm, nil
}

func (f *Factory) UserWorkloadMonitoringServiceMonitor() (*monitoringv1.ServiceMonitor, error) {
	sm, err := f.NewServiceMonitor(MustAssetReader(UserWorkloadMonitoringServiceMonitor))
	if err != nil {
		return nil, err
	}

	sm.Namespace = f.namespace

	return sm, nil
}

func (f *Factory) UserWorkloadMonitoringMeterDefinition() (*marketplacev1beta1.MeterDefinition, error) {
	m, err := f.NewMeterDefinition(MustAssetReader(UserWorkloadMonitoringMeterDefinition))
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (f *Factory) ReporterJob(
	report *marketplacev1alpha1.MeterReport,
	backoffLimit *int32,
	uploadTarget string,
) (*batchv1.Job, error) {
	j, err := f.NewJob(MustAssetReader(ReporterJob))

	if err != nil {
		return nil, err
	}

	j.Spec.BackoffLimit = backoffLimit
	container := &j.Spec.Template.Spec.Containers[0]
	container.Image = f.config.RelatedImages.Reporter
	f.ReplaceImages(container)

	j.Name = report.GetName()
	container.Args = append(container.Args,
		"--name",
		report.Name,
		"--namespace",
		report.Namespace,
	)

	if len(report.Spec.ExtraArgs) > 0 {
		container.Args = append(container.Args, report.Spec.ExtraArgs...)
	}

	if uploadTarget == "data-service" {
		dataServiceArgs := []string{"--dataServiceCertFile=/etc/configmaps/serving-certs-ca-bundle/service-ca.crt", "--dataServiceTokenFile=/etc/data-service-sa/data-service-token"}

		container.Args = append(container.Args, dataServiceArgs...)

		dataServiceVolumeMounts := []v1.VolumeMount{
			{
				Name:      "data-service-token-vol",
				ReadOnly:  true,
				MountPath: "/etc/data-service-sa",
			},
			{
				Name:      "serving-certs-ca-bundle",
				MountPath: "/etc/configmaps/serving-certs-ca-bundle",
				ReadOnly:  false,
			},
		}

		container.VolumeMounts = append(container.VolumeMounts, dataServiceVolumeMounts...)

		dataServiceTokenVols := []v1.Volume{
			{
				Name: "serving-certs-ca-bundle",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "serving-certs-ca-bundle",
						},
					},
				},
			},
			{
				Name: "data-service-token-vol",
				VolumeSource: v1.VolumeSource{
					Projected: &v1.ProjectedVolumeSource{
						Sources: []v1.VolumeProjection{
							{
								ServiceAccountToken: &v1.ServiceAccountTokenProjection{
									Audience:          utils.DataServiceAudience(f.namespace),
									ExpirationSeconds: ptr.Int64(3600),
									Path:              "data-service-token",
								},
							},
						},
					},
				},
			},
		}

		j.Spec.Template.Spec.Volumes = append(j.Spec.Template.Spec.Volumes, dataServiceTokenVols...)
	}

	// Keep last 3 days of data
	j.Spec.TTLSecondsAfterFinished = ptr.Int32(86400 * 3)

	return j, nil
}

func (f *Factory) ReporterMeterDefinition() (*marketplacev1beta1.MeterDefinition, error) {
	m, err := f.NewMeterDefinition(MustAssetReader(ReporterMeterDefinition))
	if err != nil {
		return nil, err
	}

	m.Namespace = f.namespace

	return m, nil
}

func (f *Factory) MetricStateDeployment() (*appsv1.Deployment, error) {
	d, err := f.NewDeployment(MustAssetReader(MetricStateDeployment))
	if err != nil {
		return nil, err
	}

	for i := range d.Spec.Template.Spec.Containers {
		container := &d.Spec.Template.Spec.Containers[i]
		err := f.ReplaceImages(container)

		if err != nil {
			return nil, err
		}
	}

	d.Namespace = f.namespace

	return d, nil
}

func (f *Factory) ServiceAccountPullSecret() (*corev1.Secret, error) {
	s, err := NewSecret(MustAssetReader(MetricStateRHMOperatorSecret))
	s.Namespace = f.namespace
	return s, err
}

func (f *Factory) MetricStateServiceMonitor(secretName *string) (*monitoringv1.ServiceMonitor, error) {
	sm, err := f.NewServiceMonitor(MustAssetReader(MetricStateServiceMonitorV46))
	if err != nil {
		return nil, err
	}

	sm.Namespace = f.namespace
	for i := range sm.Spec.Endpoints {
		endpoint := &sm.Spec.Endpoints[i]
		endpoint.TLSConfig.ServerName = fmt.Sprintf("rhm-metric-state-service.%s.svc", f.namespace)

		if secretName != nil && endpoint.BearerTokenFile == "" {
			addBearerToken(endpoint, *secretName)
		}
	}

	return sm, nil
}

func addBearerToken(endpoint *monitoringv1.Endpoint, secretName string) {
	endpoint.BearerTokenSecret = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: secretName,
		},
		Key: "token",
	}
}

func (f *Factory) MetricStateMeterDefinition() (*marketplacev1beta1.MeterDefinition, error) {
	m, err := f.NewMeterDefinition(MustAssetReader(MetricStateMeterDefinition))
	if err != nil {
		return nil, err
	}

	m.Namespace = f.namespace

	return m, nil
}

func (f *Factory) KubeStateMetricsService() (*corev1.Service, error) {
	s, err := f.NewService(MustAssetReader(KubeStateMetricsService))
	if err != nil {
		return nil, err
	}

	s.Namespace = f.namespace

	return s, nil
}

func (f *Factory) KubeStateMetricsServiceMonitor(secretName *string) (*monitoringv1.ServiceMonitor, error) {
	sm, err := f.NewServiceMonitor(MustAssetReader(KubeStateMetricsServiceMonitor))
	if err != nil {
		return nil, err
	}

	sm.Namespace = f.namespace

	for i := range sm.Spec.Endpoints {
		endpoint := &sm.Spec.Endpoints[i]

		if secretName != nil && endpoint.BearerTokenFile == "" {
			addBearerToken(endpoint, *secretName)
		}
	}

	return sm, nil
}

func (f *Factory) KubeletServiceMonitor(secretName *string) (*monitoringv1.ServiceMonitor, error) {
	sm, err := f.NewServiceMonitor(MustAssetReader(KubeletServiceMonitor))
	if err != nil {
		return nil, err
	}

	sm.Namespace = f.namespace

	for i := range sm.Spec.Endpoints {
		endpoint := &sm.Spec.Endpoints[i]

		if secretName != nil && endpoint.BearerTokenFile == "" {
			addBearerToken(endpoint, *secretName)
		}
	}

	return sm, nil
}

func (f *Factory) MetricStateService() (*v1.Service, error) {
	s, err := f.NewService(MustAssetReader(MetricStateService))
	if err != nil {
		return nil, err
	}

	s.Namespace = f.namespace

	return s, nil
}

func (f *Factory) NewDataServiceService() (*corev1.Service, error) {
	return f.NewService(MustAssetReader(DataServiceService))
}

func (f *Factory) UpdateDataServiceService(s *corev1.Service) error {
	s2, err := f.NewDataServiceService()
	if err != nil {
		return err
	}

	s.Spec.Ports = s2.Spec.Ports
	s.Spec.Selector = s2.Spec.Selector
	s.Annotations = s2.Annotations

	return nil
}

func (f *Factory) NewDataServiceStatefulSet() (*appsv1.StatefulSet, error) {
	sts, err := f.NewStatefulSet(MustAssetReader(DataServiceStatefulSet))
	if err != nil {
		return nil, err
	}

	f.UpdateDataServiceStatefulSet(sts)

	return sts, nil
}

func (f *Factory) UpdateDataServiceStatefulSet(sts *appsv1.StatefulSet) error {
	sts2, err := f.NewStatefulSet(MustAssetReader(DataServiceStatefulSet))
	if err != nil {
		return err
	}

	sts.Spec = sts2.Spec

	replacer := strings.NewReplacer(
		"{{NAMESPACE}}", f.namespace,
	)

	for i := range sts.Spec.Template.Spec.Containers {
		container := &sts.Spec.Template.Spec.Containers[i]
		newArgs := []string{}

		f.ReplaceImages(container)

		for _, arg := range container.Args {
			newArg := replacer.Replace(arg)
			newArgs = append(newArgs, newArg)
		}

		container.Args = newArgs
	}

	return nil
}

func (f *Factory) NewDataServiceRoute() (*routev1.Route, error) {
	return f.NewRoute(MustAssetReader(DataServiceRoute))
}

func (f *Factory) UpdateDataServiceRoute(r *routev1.Route) error {
	return f.UpdateRoute(MustAssetReader(DataServiceRoute), r)
}

func (f *Factory) NewMeterdefintionFileServerDeploymentConfig() (*osappsv1.DeploymentConfig, error) {
	return f.NewDeploymentConfig(MustAssetReader(MeterdefinitionFileServerDeploymentConfig))
}

func (f *Factory) NewMeterdefintionFileServerImageStream() (*osimagev1.ImageStream, error) {
	return f.NewImageStream(MustAssetReader(MeterdefinitionFileServerImageStream))
}

func (f *Factory) NewMeterdefintionFileServerService() (*corev1.Service, error) {
	s, err := f.NewService(MustAssetReader(MeterdefinitionFileServerService))
	if err != nil {
		return nil, err
	}

	if v, ok := s.GetAnnotations()["service.beta.openshift.io/serving-cert-secret-name"]; !ok || v != "rhm-meterdefinition-file-server-tls" {
		annotations := s.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}

		annotations["service.beta.openshift.io/serving-cert-secret-name"] = "rhm-meterdefinition-file-server-tls"

		s.SetAnnotations(annotations)
	}

	return s, nil
}

func (f *Factory) NewServiceMonitor(manifest io.Reader) (*monitoringv1.ServiceMonitor, error) {
	sm, err := NewServiceMonitor(manifest)
	if err != nil {
		return nil, err
	}

	if sm.GetNamespace() == "" {
		sm.SetNamespace(f.namespace)
	}

	return sm, nil
}

func NewMeterDefinition(manifest io.Reader) (*marketplacev1beta1.MeterDefinition, error) {
	sm := marketplacev1beta1.MeterDefinition{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&sm)
	if err != nil {
		return nil, err
	}

	return &sm, nil
}

func NewDeployment(manifest io.Reader) (*appsv1.Deployment, error) {
	d := appsv1.Deployment{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&d)
	if err != nil {
		return nil, err
	}

	return &d, nil
}

func NewStatefulSet(manifest io.Reader) (*appsv1.StatefulSet, error) {
	d := appsv1.StatefulSet{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&d)
	if err != nil {
		return nil, err
	}

	return &d, nil
}

func NewConfigMap(manifest io.Reader) (*v1.ConfigMap, error) {
	cm := v1.ConfigMap{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&cm)
	if err != nil {
		return nil, err
	}

	return &cm, nil
}

func NewService(manifest io.Reader) (*v1.Service, error) {
	s := v1.Service{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&s)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func NewPrometheus(manifest io.Reader) (*monitoringv1.Prometheus, error) {
	s := monitoringv1.Prometheus{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&s)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func NewSecret(manifest io.Reader) (*v1.Secret, error) {
	s := v1.Secret{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func NewJob(manifest io.Reader) (*batchv1.Job, error) {
	j := batchv1.Job{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&j)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func NewCronJob(manifest io.Reader) (*batchv1beta1.CronJob, error) {
	j := batchv1beta1.CronJob{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&j)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func NewRoute(manifest io.Reader) (*routev1.Route, error) {
	r := routev1.Route{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func NewImageStream(manifest io.Reader) (*osimagev1.ImageStream, error) {
	is := osimagev1.ImageStream{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&is)
	if err != nil {
		return nil, err
	}

	return &is, nil
}

func NewImageStreamTag(manifest io.Reader) (*osimagev1.ImageStreamTag, error) {
	it := osimagev1.ImageStreamTag{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&it)
	if err != nil {
		return nil, err
	}

	return &it, nil
}

func NewDeploymentConfig(manifest io.Reader) (*osappsv1.DeploymentConfig, error) {
	d := osappsv1.DeploymentConfig{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&d)
	if err != nil {
		return nil, err
	}

	return &d, nil
}

// GeneratePassword returns a base64 encoded securely random bytes.
func GeneratePassword(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), err
}

func NewServiceMonitor(manifest io.Reader) (*monitoringv1.ServiceMonitor, error) {
	sm := monitoringv1.ServiceMonitor{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&sm)
	if err != nil {
		return nil, err
	}

	return &sm, nil
}

type Owner metav1.Object

func (f *Factory) SetOwnerReference(owner Owner, obj metav1.Object) error {
	return controllerutil.SetOwnerReference(owner, obj, f.scheme)
}

func (f *Factory) SetControllerReference(owner Owner, obj metav1.Object) error {
	return controllerutil.SetControllerReference(owner, obj, f.scheme)
}

func (f *Factory) UpdateRemoteResourceS3Deployment(dep *appsv1.Deployment) error {
	if dep.GetNamespace() == "" {
		dep.SetNamespace(f.namespace)
	}

	var securityContext *corev1.PodSecurityContext
	if !f.operatorConfig.Infrastructure.HasOpenshift() {
		securityContext = &corev1.PodSecurityContext{
			FSGroup: ptr.Int64(1000),
		}
	}
	dep.Spec.Template.Spec.SecurityContext = securityContext

	for i := range dep.Spec.Template.Spec.Containers {
		container := &dep.Spec.Template.Spec.Containers[i]
		f.ReplaceImages(container)
	}

	return nil
}

func (f *Factory) NewRemoteResourceS3Deployment() (*appsv1.Deployment, error) {
	dep, err := f.NewDeployment(MustAssetReader(RRS3ControllerDeployment))
	if err != nil {
		return nil, err
	}
	err = f.UpdateRemoteResourceS3Deployment(dep)
	return dep, err
}

func (f *Factory) UpdateWatchKeeperDeployment(dep *appsv1.Deployment) error {
	if dep.GetNamespace() == "" {
		dep.SetNamespace(f.namespace)
	}

	var securityContext *corev1.PodSecurityContext
	if !f.operatorConfig.Infrastructure.HasOpenshift() {
		securityContext = &corev1.PodSecurityContext{
			FSGroup: ptr.Int64(1000),
		}
	}
	dep.Spec.Template.Spec.SecurityContext = securityContext

	for i := range dep.Spec.Template.Spec.Containers {
		container := &dep.Spec.Template.Spec.Containers[i]
		f.ReplaceImages(container)
	}

	return nil
}

func (f *Factory) NewWatchKeeperDeployment() (*appsv1.Deployment, error) {
	dep, err := f.NewDeployment(MustAssetReader(WatchKeeperDeployment))
	if err != nil {
		return nil, err
	}
	err = f.UpdateWatchKeeperDeployment(dep)
	return dep, err
}

func (f *Factory) NewDataServiceTLSSecret(commonNamePrefix string) (*v1.Secret, error) {
	s, err := f.NewSecret(MustAssetReader(DataServiceTLSSecret))
	if err != nil {
		return nil, err
	}

	nameParts := []string{commonNamePrefix, f.namespace, "svc", "cluster", "local"}
	commonName := strings.Join(nameParts, ".")

	caCertPEM, caKeyPEM, serverKeyPEM, serverCertPEM, err := newCertificateBundleSecret(commonName)
	if err != nil {
		return nil, err
	}

	if s.Data == nil {
		s.Data = make(map[string][]byte)
	}

	s.Data["ca.crt"] = caCertPEM
	s.Data["ca.key"] = caKeyPEM
	s.Data["tls.crt"] = serverKeyPEM
	s.Data["tls.key"] = serverCertPEM

	return s, nil
}

func init() {
	mathrand.Seed(time.Now().UnixNano())
}
