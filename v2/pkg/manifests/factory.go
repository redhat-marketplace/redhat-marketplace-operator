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
	"strings"

	"github.com/gotidy/ptr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/assets"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"golang.org/x/net/http/httpproxy"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
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
	PrometheusOperatorDeploymentV45 = "prometheus-operator/deployment-v4.5.yaml"
	PrometheusOperatorDeploymentV46 = "prometheus-operator/deployment-v4.6.yaml"
	PrometheusOperatorServiceV45    = "prometheus-operator/service-v4.5.yaml"
	PrometheusOperatorServiceV46    = "prometheus-operator/service-v4.6.yaml"
	PrometheusOperatorCertsCABundle = "prometheus-operator/operator-certs-ca-bundle.yaml"

	PrometheusAdditionalScrapeConfig = "prometheus/additional-scrape-configs.yaml"
	PrometheusHtpasswd               = "prometheus/htpasswd-secret.yaml"
	PrometheusRBACProxySecret        = "prometheus/kube-rbac-proxy-secret.yaml"
	PrometheusDeploymentV45          = "prometheus/prometheus-v4.5.yaml"
	PrometheusDeploymentV46          = "prometheus/prometheus-v4.6.yaml"
	PrometheusProxySecret            = "prometheus/proxy-secret.yaml"
	PrometheusService                = "prometheus/service.yaml"
	PrometheusDatasourcesSecret      = "prometheus/prometheus-datasources-secret.yaml"
	PrometheusServingCertsCABundle   = "prometheus/serving-certs-ca-bundle.yaml"
	PrometheusKubeletServingCABundle = "prometheus/kubelet-serving-ca-bundle.yaml"
	PrometheusServiceMonitor         = "prometheus/service-monitor.yaml"
	PrometheusMeterDefinition        = "prometheus/meterdefinition.yaml"

	ReporterJob                       = "reporter/job.yaml"
	ReporterUserWorkloadMonitoringJob = "reporter/user-workload-monitoring-job.yaml"
	ReporterMeterDefinition           = "reporter/meterdefinition.yaml"

	MetricStateDeployment        = "metric-state/deployment.yaml"
	MetricStateServiceMonitorV45 = "metric-state/service-monitor-v4.5.yaml"
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

func (f *Factory) ReplaceImages(container *corev1.Container) {
	switch {
	case strings.HasPrefix(container.Name, "kube-rbac-proxy"):
		container.Image = f.config.RelatedImages.KubeRbacProxy
	case container.Name == "metric-state":
		container.Image = f.config.RelatedImages.MetricState
	case container.Name == "authcheck":
		container.Image = f.config.RelatedImages.AuthChecker
		container.Args = append(container.Args, "--namespace", f.namespace)
		container.LivenessProbe = &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8089),
				},
			},
			InitialDelaySeconds: 15,
			PeriodSeconds:       20,
		}
		container.ReadinessProbe = &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/readyz",
					Port: intstr.FromInt(8089),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		}
		container.Env = []v1.EnvVar{
			{
				Name: "POD_NAMESPACE",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name: "POD_NAME",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
		}
	case container.Name == "prometheus-operator":
		container.Image = f.config.RelatedImages.PrometheusOperator
	case container.Name == "prometheus-proxy":
		container.Image = f.config.RelatedImages.OAuthProxy
	}

	if container.Env == nil {
		container.Env = []corev1.EnvVar{}
	}

	proxyInfo := httpproxy.FromEnvironment()

	if proxyInfo.HTTPProxy != "" {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "HTTP_PROXY",
			Value: proxyInfo.HTTPProxy,
		})
	}

	if proxyInfo.HTTPSProxy != "" {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "HTTPS_PROXY",
			Value: proxyInfo.HTTPSProxy,
		})
	}

	if proxyInfo.NoProxy != "" {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "NO_PROXY",
			Value: proxyInfo.NoProxy,
		})
	}
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
	if f.operatorConfig.HasOpenshift() && f.operatorConfig.Infrastructure.OpenshiftParsedVersion().GTE(utils.ParsedVersion460) {
		return PrometheusOperatorDeploymentV46
	}
	return PrometheusOperatorDeploymentV45
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

		f.ReplaceImages(container)

		for _, arg := range container.Args {
			newArg := replacer.Replace(arg)
			newArgs = append(newArgs, newArg)
		}

		container.Args = newArgs
	}

	return dep, err
}

func (f *Factory) prometheusDeployment() string {
	if f.operatorConfig.HasOpenshift() && f.operatorConfig.Infrastructure.OpenshiftParsedVersion().GTE(utils.ParsedVersion460) {
		return PrometheusDeploymentV46
	}
	return PrometheusDeploymentV45
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

	if cr.Spec.Prometheus.Replicas != nil {
		p.Spec.Replicas = cr.Spec.Prometheus.Replicas
	}

	if f.config.PrometheusConfig.Retention != "" {
		p.Spec.Retention = f.config.PrometheusConfig.Retention
	}

	//Set empty dir if present in the CR, will override a pvc specified (per prometheus docs)
	if cr.Spec.Prometheus.Storage.EmptyDir != nil {
		p.Spec.Storage.EmptyDir = cr.Spec.Prometheus.Storage.EmptyDir
	}

	storageClass := ptr.String("")
	if cr.Spec.Prometheus.Storage.Class != nil {
		storageClass = cr.Spec.Prometheus.Storage.Class
	}

	quanBytes := cr.Spec.Prometheus.Storage.Size.DeepCopy()
	quanBytes.Sub(resource.MustParse("2Gi"))
	replacer := strings.NewReplacer("Mi", "MB", "Gi", "GB", "Ti", "TB")
	storageSize := replacer.Replace(quanBytes.String())
	p.Spec.RetentionSize = storageSize

	pvc, err := utils.NewPersistentVolumeClaim(utils.PersistentVolume{
		ObjectMeta: &metav1.ObjectMeta{
			Name: "storage-volume",
		},
		StorageClass: storageClass,
		StorageSize:  &cr.Spec.Prometheus.Storage.Size,
	})

	p.Spec.Storage.VolumeClaimTemplate = monitoringv1.EmbeddedPersistentVolumeClaim{
		Spec: pvc.Spec,
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
		f.ReplaceImages(&p.Spec.Containers[i])
	}

	return p, err
}

func (f *Factory) prometheusOperatorService() string {
	if f.operatorConfig.HasOpenshift() && f.operatorConfig.Infrastructure.OpenshiftParsedVersion().GTE(utils.ParsedVersion460) {
		return PrometheusOperatorServiceV46
	}
	return PrometheusOperatorServiceV45
}

func (f *Factory) NewPrometheusOperatorService() (*corev1.Service, error) {
	service, err := f.NewService(MustAssetReader(f.prometheusOperatorService()))

	return service, err
}

func (f *Factory) NewPrometheusOperatorCertsCABundle() (*corev1.ConfigMap, error) {
	return f.NewConfigMap(MustAssetReader(PrometheusOperatorCertsCABundle))
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
) (*batchv1.Job, error) {
	j, err := f.NewJob(MustAssetReader(ReporterJob))

	if err != nil {
		return nil, err
	}

	j.Spec.BackoffLimit = backoffLimit
	container := j.Spec.Template.Spec.Containers[0]
	container.Image = f.config.RelatedImages.Reporter

	proxyInfo := httpproxy.FromEnvironment()

	if container.Env == nil {
		container.Env = []corev1.EnvVar{}
	}

	if proxyInfo.HTTPProxy != "" {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "HTTP_PROXY",
			Value: proxyInfo.HTTPProxy,
		})
	}

	if proxyInfo.HTTPSProxy != "" {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "HTTPS_PROXY",
			Value: proxyInfo.HTTPSProxy,
		})
	}

	if proxyInfo.NoProxy != "" {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "NO_PROXY",
			Value: proxyInfo.NoProxy,
		})
	}

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

	// Keep last 3 days of data
	j.Spec.TTLSecondsAfterFinished = ptr.Int32(86400 * 3)
	j.Spec.Template.Spec.Containers[0] = container

	return j, nil
}

func (f *Factory) ReporterUserWorkloadMonitoringJob(
	report *marketplacev1alpha1.MeterReport,
	backoffLimit *int32,
) (*batchv1.Job, error) {
	j, err := f.NewJob(MustAssetReader(ReporterUserWorkloadMonitoringJob))

	if err != nil {
		return nil, err
	}

	j.Spec.BackoffLimit = backoffLimit
	container := j.Spec.Template.Spec.Containers[0]
	container.Image = f.config.RelatedImages.Reporter

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

	// Keep last 3 days of data
	j.Spec.TTLSecondsAfterFinished = ptr.Int32(86400 * 3)
	j.Spec.Template.Spec.Containers[0] = container

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
		f.ReplaceImages(&d.Spec.Template.Spec.Containers[i])
	}

	d.Namespace = f.namespace

	return d, nil
}

func (f *Factory) MetricStateServiceMonitor(pod *corev1.Pod) (*monitoringv1.ServiceMonitor, error) {
	fileName := MetricStateServiceMonitorV45
	isValidOpenShiftVersion := false
	if f.operatorConfig.HasOpenshift() && f.operatorConfig.Infrastructure.OpenshiftParsedVersion().GTE(utils.ParsedVersion460) {
		fileName = MetricStateServiceMonitorV46
		isValidOpenShiftVersion = true
	}

	sm, err := f.NewServiceMonitor(MustAssetReader(fileName))
	if err != nil {
		return nil, err
	}

	sm.Namespace = f.namespace

	var secretName *string

	if isValidOpenShiftVersion && pod != nil {
		for _, volume := range pod.Spec.Volumes {
			if volume.Secret != nil && strings.Contains(volume.Secret.SecretName, "redhat-marketplace-operator-token-") {
				secretName = &volume.Secret.SecretName
			}
		}
	}

	for i := range sm.Spec.Endpoints {
		endpoint := &sm.Spec.Endpoints[i]
		endpoint.TLSConfig.ServerName = fmt.Sprintf("rhm-metric-state-service.%s.svc", f.namespace)

		if secretName != nil {
			endpoint.BearerTokenSecret = corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: *secretName,
				},
				Key: "token",
			}
		}
	}

	return sm, nil
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

func (f *Factory) KubeStateMetricsServiceMonitor() (*monitoringv1.ServiceMonitor, error) {
	sm, err := f.NewServiceMonitor(MustAssetReader(KubeStateMetricsServiceMonitor))
	if err != nil {
		return nil, err
	}

	sm.Namespace = f.namespace

	return sm, nil
}

func (f *Factory) KubeletServiceMonitor() (*monitoringv1.ServiceMonitor, error) {
	sm, err := f.NewServiceMonitor(MustAssetReader(KubeletServiceMonitor))
	if err != nil {
		return nil, err
	}

	sm.Namespace = f.namespace

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

func NewDeployment(manifest io.Reader) (*appsv1.Deployment, error) {
	d := appsv1.Deployment{}
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

func NewMeterDefinition(manifest io.Reader) (*marketplacev1beta1.MeterDefinition, error) {
	sm := marketplacev1beta1.MeterDefinition{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&sm)
	if err != nil {
		return nil, err
	}

	return &sm, nil
}

func (f *Factory) NewWatchKeeperDeployment(instance *marketplacev1alpha1.RazeeDeployment) *appsv1.Deployment {
	var securityContext *corev1.PodSecurityContext
	if !f.operatorConfig.Infrastructure.HasOpenshift() {
		securityContext = &corev1.PodSecurityContext{
			FSGroup: ptr.Int64(1000),
		}
	}
	rep := ptr.Int32(1)
	maxSurge := intstr.FromString("25%")
	maxUnavailable := intstr.FromString("25%")

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.RHM_WATCHKEEPER_DEPLOYMENT_NAME,
			Namespace: f.namespace,
			Labels: map[string]string{
				"razee/watch-resource": "lite",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: rep,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":      utils.RHM_WATCHKEEPER_DEPLOYMENT_NAME,
					"owned-by": "marketplace.redhat.com-razee",
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: "RollingUpdate",
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                  utils.RHM_WATCHKEEPER_DEPLOYMENT_NAME,
						"razee/watch-resource": "lite",
						"owned-by":             "marketplace.redhat.com-razee",
					},
					Name: utils.RHM_WATCHKEEPER_DEPLOYMENT_NAME,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "redhat-marketplace-watch-keeper",
					Containers: []corev1.Container{
						{
							Image:           f.config.RelatedImages.AuthChecker,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Name:            "authcheck",
							Env: []v1.EnvVar{
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name: "POD_NAME",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt(8089),
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       20,
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/readyz",
										Port: intstr.FromInt(8089),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("20m"),
									corev1.ResourceMemory: resource.MustParse("40Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("20Mi"),
								},
							},
							Args: []string{
								"--namespace", f.namespace,
							},
							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						},
						{
							Image:                    f.config.RelatedImages.WatchKeeper,
							ImagePullPolicy:          corev1.PullIfNotPresent,
							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("400m"),
									corev1.ResourceMemory: resource.MustParse("500Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
							},

							Env: []corev1.EnvVar{
								{
									Name: "NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath:  "metadata.namespace",
											APIVersion: "v1",
										},
									},
								},
								{
									Name:  "NODE_ENV",
									Value: "production",
								},
							},
							Name: "watch-keeper",
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{"sh/liveness.sh"},
									},
								},
								InitialDelaySeconds: 600,
								PeriodSeconds:       300,
								TimeoutSeconds:      30,
								SuccessThreshold:    1,
								FailureThreshold:    1,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      utils.WATCH_KEEPER_CONFIG_NAME,
									MountPath: "/home/node/envs/watch-keeper-config",
									ReadOnly:  true,
								},
								{
									Name:      utils.WATCH_KEEPER_SECRET_NAME,
									MountPath: "/home/node/envs/watch-keeper-secret",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: utils.WATCH_KEEPER_CONFIG_NAME,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: utils.WATCH_KEEPER_CONFIG_NAME,
									},
									DefaultMode: ptr.Int32(0440),
									Optional:    ptr.Bool(false),
								},
							},
						},
						{
							Name: utils.WATCH_KEEPER_SECRET_NAME,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  utils.WATCH_KEEPER_SECRET_NAME,
									DefaultMode: ptr.Int32(0400),
									Optional:    ptr.Bool(false),
								},
							},
						},
					},
					SecurityContext: securityContext,
				},
			},
		},
	}

	for _, containerObj := range dep.Spec.Template.Spec.Containers {
		container := &containerObj

		if container.Env == nil {
			container.Env = []corev1.EnvVar{}
		}

		proxyInfo := httpproxy.FromEnvironment()

		if proxyInfo.HTTPProxy != "" {
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  "HTTP_PROXY",
				Value: proxyInfo.HTTPProxy,
			})
		}

		if proxyInfo.HTTPSProxy != "" {
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  "HTTPS_PROXY",
				Value: proxyInfo.HTTPSProxy,
			})
		}

		if proxyInfo.NoProxy != "" {
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  "NO_PROXY",
				Value: proxyInfo.NoProxy,
			})
		}
	}

	return dep
}

type Owner metav1.Object

func (f *Factory) SetOwnerReference(owner Owner, obj metav1.Object) error {
	return controllerutil.SetOwnerReference(owner, obj, f.scheme)
}

func (f *Factory) SetControllerReference(owner Owner, obj metav1.Object) error {
	return controllerutil.SetControllerReference(owner, obj, f.scheme)
}

func (f *Factory) NewRemoteResourceS3Deployment(instance *marketplacev1alpha1.RazeeDeployment) *appsv1.Deployment {
	var securityContext *corev1.PodSecurityContext
	if !f.operatorConfig.Infrastructure.HasOpenshift() {
		securityContext = &corev1.PodSecurityContext{
			FSGroup: ptr.Int64(1000),
		}
	}
	rep := ptr.Int32(1)
	maxSurge := intstr.FromString("25%")
	maxUnavailable := intstr.FromString("25%")

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.RHM_REMOTE_RESOURCE_S3_DEPLOYMENT_NAME,
			Namespace: f.namespace,
			Labels: map[string]string{
				"razee/watch-resource": "lite",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: rep,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":      utils.RHM_REMOTE_RESOURCE_S3_DEPLOYMENT_NAME,
					"owned-by": "marketplace.redhat.com-razee",
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: "RollingUpdate",
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                  utils.RHM_REMOTE_RESOURCE_S3_DEPLOYMENT_NAME,
						"razee/watch-resource": "lite",
						"owned-by":             "marketplace.redhat.com-razee",
					},
					Name: utils.RHM_REMOTE_RESOURCE_S3_DEPLOYMENT_NAME,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "redhat-marketplace-remoteresources3deployment",
					Containers: []corev1.Container{
						{
							Image:           f.config.RelatedImages.AuthChecker,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Name:            "authcheck",
							Env: []v1.EnvVar{
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name: "POD_NAME",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("20m"),
									corev1.ResourceMemory: resource.MustParse("40Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("20Mi"),
								},
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt(8089),
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       20,
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/readyz",
										Port: intstr.FromInt(8089),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
							},
							Args: []string{
								"--namespace", f.namespace,
							},
							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						},
						{
							Image:                    f.config.RelatedImages.RemoteResourceS3,
							ImagePullPolicy:          corev1.PullIfNotPresent,
							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("200Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("40m"),
									corev1.ResourceMemory: resource.MustParse("75Mi"),
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "CRD_WATCH_TIMEOUT_SECONDS",
									ValueFrom: &corev1.EnvVarSource{
										ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "razeedeploy-overrides",
											},
											Key:      "CRD_WATCH_TIMEOUT_SECONDS",
											Optional: ptr.Bool(true),
										},
									},
								},
								{
									Name:  "GROUP",
									Value: "marketplace.redhat.com",
								},
								{
									Name:  "VERSION",
									Value: "v1alpha1",
								},
							},
							Name: utils.RHM_REMOTE_RESOURCE_S3_DEPLOYMENT_NAME,
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{"sh/liveness.sh"},
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       150,
								TimeoutSeconds:      30,
								FailureThreshold:    1,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/usr/src/app/download-cache",
									Name:      "cache-volume",
								},
								{
									MountPath: "/usr/src/app/config",
									Name:      "razeedeploy-config",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "cache-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									Medium: corev1.StorageMediumDefault,
								},
							},
						},
						{
							Name: "razeedeploy-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "razeedeploy-config",
									},
									DefaultMode: ptr.Int32(440),
									Optional:    ptr.Bool(true),
								},
							},
						},
					},
					SecurityContext: securityContext,
				},
			},
		},
	}

	for _, containerObj := range dep.Spec.Template.Spec.Containers {
		container := &containerObj

		if container.Env == nil {
			container.Env = []corev1.EnvVar{}
		}

		proxyInfo := httpproxy.FromEnvironment()

		if proxyInfo.HTTPProxy != "" {
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  "HTTP_PROXY",
				Value: proxyInfo.HTTPProxy,
			})
		}

		if proxyInfo.HTTPSProxy != "" {
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  "HTTPS_PROXY",
				Value: proxyInfo.HTTPSProxy,
			})
		}

		if proxyInfo.NoProxy != "" {
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  "NO_PROXY",
				Value: proxyInfo.NoProxy,
			})
		}
	}

	return dep
}
