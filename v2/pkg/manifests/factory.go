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
	"context"
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
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/assets"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/envvar"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"dario.cat/mergo"
	"k8s.io/client-go/util/retry"
)

const (
	ReporterCronJob         = "reporter/cronjob.yaml"
	ReporterMeterDefinition = "reporter/meterdefinition.yaml"

	MetricStateDeployment        = "metric-state/deployment.yaml"
	MetricStateServiceMonitorV46 = "metric-state/service-monitor-v4.6.yaml"
	MetricStateService           = "metric-state/service.yaml"
	MetricStateMeterDefinition   = "metric-state/meterdefinition.yaml"

	KubeStateMetricsService = "metric-state/kube-state-metrics-service.yaml"

	UserWorkloadMonitoringMeterDefinition = "prometheus/user-workload-monitoring-meterdefinition.yaml"

	DataServiceStatefulSet         = "dataservice/statefulset.yaml"
	DataServiceService             = "dataservice/service.yaml"
	DataServiceRoute               = "dataservice/route.yaml"
	DataServiceTLSSecret           = "dataservice/secret.yaml"
	DataServicePodDisruptionBudget = "dataservice/pdb.yaml"

	// ibm-metrics-operator olm manifests
	MOServiceMonitorMetricsReaderSecret = "ibm-metrics-operator/servicemonitor-metrics-reader-secret.yaml"
	MOMetricsServiceMonitor             = "ibm-metrics-operator/metrics-service-monitor.yaml"
	MOMetricsService                    = "ibm-metrics-operator/metrics-service.yaml"
	MOCABundleConfigMap                 = "ibm-metrics-operator/metrics-ca-bundle-configmap.yaml"

	// redhat-marketplace-operator olm manifests
	RHMOServiceMonitorMetricsReaderSecret = "redhat-marketplace-operator/servicemonitor-metrics-reader-secret.yaml"
	RHMOMetricsServiceMonitor             = "redhat-marketplace-operator/metrics-service-monitor.yaml"
	RHMOMetricsService                    = "redhat-marketplace-operator/metrics-service.yaml"
	RHMOCABundleConfigMap                 = "redhat-marketplace-operator/metrics-ca-bundle-configmap.yaml"
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
	case container.Name == "rhm-data-service":
		container.Image = f.config.RelatedImages.DQLite
	case container.Name == "rhm-meterdefinition-file-server":
		container.Image = f.config.RelatedImages.MeterDefFileServer
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

func (f *Factory) NewCronJob(manifest io.Reader) (*batchv1.CronJob, error) {
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

func (f *Factory) NewReporterCronJob(userWorkloadEnabled bool, isDisconnected bool) (*batchv1.CronJob, error) {
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
		"--dataServiceCertFile=/etc/configmaps/ibm-metrics-operator-serving-certs-ca-bundle/service-ca.crt",
		"--dataServiceTokenFile=/etc/data-service-sa/data-service-token",
		"--cafile=/etc/configmaps/ibm-metrics-operator-serving-certs-ca-bundle/service-ca.crt",
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
			Name:      "ibm-metrics-operator-serving-certs-ca-bundle",
			MountPath: "/etc/configmaps/ibm-metrics-operator-serving-certs-ca-bundle",
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
			Name: "ibm-metrics-operator-serving-certs-ca-bundle",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "ibm-metrics-operator-serving-certs-ca-bundle",
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

	// Inject the SubscriptionConfig overrides
	if err := injectSubscriptionConfig(&j.Spec.JobTemplate.Spec.Template.Spec, f.operatorConfig.Infrastructure.SubscriptionConfig()); err != nil {
		return nil, fmt.Errorf("failed to inject subscription config - name=%s - %v", j.Name, err)
	}

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

func (f *Factory) NewPodDisruptionBudget(manifest io.Reader) (*policyv1.PodDisruptionBudget, error) {
	p, err := NewPodDisruptionBudget(manifest)
	if err != nil {
		return nil, err
	}

	if p.GetNamespace() == "" {
		p.SetNamespace(f.namespace)
	}

	return p, nil
}

func (f *Factory) UpdatePodDisruptionBudget(manifest io.Reader, p *policyv1.PodDisruptionBudget) error {
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(p)
	if err != nil {
		return err
	}

	return nil
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

func (f *Factory) generateHtpasswdSecret(s *v1.Secret, password string) {
	h := sha1.New()
	h.Write([]byte(password))
	s.Data["auth"] = []byte("internal:{SHA}" + base64.StdEncoding.EncodeToString(h.Sum(nil)))
	s.Namespace = f.namespace
}

func (f *Factory) UserWorkloadMonitoringMeterDefinition() (*marketplacev1beta1.MeterDefinition, error) {
	m, err := f.NewMeterDefinition(MustAssetReader(UserWorkloadMonitoringMeterDefinition))
	if err != nil {
		return nil, err
	}

	return m, nil
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

	// Inject the SubscriptionConfig overrides
	if err := injectSubscriptionConfig(&d.Spec.Template.Spec, f.operatorConfig.Infrastructure.SubscriptionConfig()); err != nil {
		return nil, fmt.Errorf("failed to inject subscription config - name=%s - %v", d.Name, err)
	}

	return d, nil
}

func (f *Factory) MetricStateServiceMonitor(secretName *string) (*monitoringv1.ServiceMonitor, error) {
	sm, err := f.NewServiceMonitor(MustAssetReader(MetricStateServiceMonitorV46))
	if err != nil {
		return nil, err
	}

	sm.Namespace = f.namespace

	for i := range sm.Spec.Endpoints {
		endpoint := &sm.Spec.Endpoints[i]
		endpoint.TLSConfig.ServerName = ptr.String(fmt.Sprintf("rhm-metric-state-service.%s.svc", f.namespace))

		if secretName != nil && endpoint.Authorization == nil {
			addBearerToken(endpoint, *secretName)
		}
	}

	return sm, nil
}

func addBearerToken(endpoint *monitoringv1.Endpoint, secretName string) {
	endpoint.Authorization.Credentials = &corev1.SecretKeySelector{
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

func (f *Factory) NewDataServiceStatefulSet(storageClassName *string) (*appsv1.StatefulSet, error) {
	sts, err := f.NewStatefulSet(MustAssetReader(DataServiceStatefulSet))
	if err != nil {
		return nil, err
	}

	f.UpdateDataServiceStatefulSet(sts, storageClassName)

	return sts, nil
}

func (f *Factory) UpdateDataServiceStatefulSet(sts *appsv1.StatefulSet, storageClassName *string) error {
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

	for i := range sts.Spec.VolumeClaimTemplates {
		volumeClaimTemplate := &sts.Spec.VolumeClaimTemplates[i]
		volumeClaimTemplate.Spec.StorageClassName = storageClassName
	}

	// Inject the SubscriptionConfig overrides
	if err := injectSubscriptionConfig(&sts.Spec.Template.Spec, f.operatorConfig.Infrastructure.SubscriptionConfig()); err != nil {
		return fmt.Errorf("failed to inject subscription config - name=%s - %v", sts.Name, err)
	}

	// Allow env DATA_SERVICE_REPLICAS to set replicas on sts
	sts.Spec.Replicas = ptr.Int32(int32(f.operatorConfig.DataServiceReplicas))

	return nil
}

func (f *Factory) NewDataServiceRoute() (*routev1.Route, error) {
	return f.NewRoute(MustAssetReader(DataServiceRoute))
}

func (f *Factory) UpdateDataServiceRoute(r *routev1.Route) error {
	return f.UpdateRoute(MustAssetReader(DataServiceRoute), r)
}

func (f *Factory) NewDataServicePodDisruptionBudget() (*policyv1.PodDisruptionBudget, error) {
	return f.NewPodDisruptionBudget(MustAssetReader(DataServicePodDisruptionBudget))
}

func (f *Factory) UpdateDataServicePodDisruptionBudget(p *policyv1.PodDisruptionBudget) error {
	return f.UpdatePodDisruptionBudget(MustAssetReader(DataServicePodDisruptionBudget), p)
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

func NewCronJob(manifest io.Reader) (*batchv1.CronJob, error) {
	j := batchv1.CronJob{}
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

func NewPodDisruptionBudget(manifest io.Reader) (*policyv1.PodDisruptionBudget, error) {
	p := policyv1.PodDisruptionBudget{}
	err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&p)
	if err != nil {
		return nil, err
	}
	return &p, nil
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

func (f *Factory) NewMOServiceMonitorMetricsReaderSecret() (*v1.Secret, error) {
	return f.NewSecret(MustAssetReader(MOServiceMonitorMetricsReaderSecret))
}

func (f *Factory) NewMOMetricsServiceMonitor() (*monitoringv1.ServiceMonitor, error) {
	return f.NewServiceMonitor(MustAssetReader(MOMetricsServiceMonitor))
}

func (f *Factory) NewMOMetricsService() (*corev1.Service, error) {
	return f.NewService(MustAssetReader(MOMetricsService))
}

func (f *Factory) NewMOCABundleConfigMap() (*corev1.ConfigMap, error) {
	return f.NewConfigMap(MustAssetReader(MOCABundleConfigMap))
}

func (f *Factory) NewRHMOServiceMonitorMetricsReaderSecret() (*v1.Secret, error) {
	return f.NewSecret(MustAssetReader(RHMOServiceMonitorMetricsReaderSecret))
}

func (f *Factory) NewRHMOMetricsServiceMonitor() (*monitoringv1.ServiceMonitor, error) {
	return f.NewServiceMonitor(MustAssetReader(RHMOMetricsServiceMonitor))
}

func (f *Factory) NewRHMOMetricsService() (*corev1.Service, error) {
	return f.NewService(MustAssetReader(RHMOMetricsService))
}

func (f *Factory) NewRHMOCABundleConfigMap() (*corev1.ConfigMap, error) {
	return f.NewConfigMap(MustAssetReader(RHMOCABundleConfigMap))
}

func init() {
	mathrand.Seed(time.Now().UnixNano())
}

// Common reconcile pattern, create or update to match object from no-arg factory func
func (f *Factory) CreateOrUpdate(c client.Client, owner metav1.Object, fn func() (client.Object, error)) error {

	obj, err := fn()
	if err != nil {
		return err
	}

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, err := controllerutil.CreateOrUpdate(context.TODO(), c, obj, func() error {
			updateObj, _ := fn()
			if owner != nil {
				controllerutil.SetControllerReference(owner, updateObj, f.scheme)
			}
			return mergo.Merge(obj, updateObj, mergo.WithOverride)
		})
		return err
	}); err != nil {
		return err
	}

	return nil
}
