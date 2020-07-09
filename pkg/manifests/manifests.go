//go:generate go-bindata -o bindata.go -prefix "../../" -pkg manifests ../../assets/...

package manifests

import (
	"bytes"
	"io"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	PrometheusOperatorDeployment    = "assets/prometheus-operator/deployment.yaml"
	PrometheusOperatorService       = "assets/prometheus-operator/service.yaml"
	PrometheusOperatorCertsCABundle = "assets/prometheus-operator/operator-certs-ca-bundle.yaml"

	PrometheusDeployment    = "assets/prometheus/prometheus.yaml"
	PrometheusServingCertsCABundle     = "assets/prometheus/serving-certs-ca-bundle.yaml"
	PrometheusKubeletServingCABundle   = "assets/prometheus/kubelet-serving-ca-bundle.yaml"
)

func MustAssetReader(asset string) io.Reader {
	return bytes.NewReader(MustAsset(asset))
}

type Factory struct {
	namespace string
	config    *Config
}

func NewFactory(namespace string, c *Config) *Factory {
	return &Factory{
		namespace: namespace,
		config:    c,
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

func (f *Factory) NewPrometheus(manifest io.Reader) (*monitoringv1.Prometheus, error) {
	d, err := NewPrometheus(manifest)
	if err != nil {
		return nil, err
	}

	if d.GetNamespace() == "" {
		d.SetNamespace(f.namespace)
	}

	return d, nil
}

func (f *Factory) NewPrometheusOperatorDeployment() (*appsv1.Deployment, error) {
	c := f.config.MarketplaceOperatorConfig.PrometheusOperatorConfig
	dep, err := f.NewDeployment(MustAssetReader(PrometheusOperatorDeployment))

	if len(c.NodeSelector) > 0 {
		dep.Spec.Template.Spec.NodeSelector = c.NodeSelector
	}

	if len(c.Tolerations) > 0 {
		dep.Spec.Template.Spec.Tolerations = c.Tolerations
	}

	if c.ServiceAccountName != "" {
		dep.Spec.Template.Spec.ServiceAccountName = c.ServiceAccountName
	}

	return dep, err
}

func (f *Factory) NewPrometheusDeployment() (*monitoringv1.Prometheus, error) {
	d, err := f.NewPrometheus(MustAssetReader(PrometheusDeployment))

	return d, err
}

func (f *Factory) NewPrometheusOperatorService() (*corev1.Service, error) {
	service, err := f.NewService(MustAssetReader(PrometheusOperatorService))

	return service, err
}

func (f *Factory) NewPrometheusOperatorCertsCABundle() (*corev1.ConfigMap, error) {
	return f.NewConfigMap(MustAssetReader(PrometheusOperatorCertsCABundle))
}

func (f *Factory) PrometheusKubeletServingCABundle(data map[string]string) (*v1.ConfigMap, error) {
	c, err := f.NewConfigMap(MustAssetReader(PrometheusKubeletServingCABundle))
	if err != nil {
		return nil, err
	}

	c.Namespace = f.namespace
	c.Data = data

	return c, nil
}

func (f *Factory) PrometheusServingCertsCABundle() (*v1.ConfigMap, error) {
	c, err := f.NewConfigMap(MustAssetReader(PrometheusServingCertsCABundle))
	if err != nil {
		return nil, err
	}

	c.Namespace = f.namespace

	return c, nil
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
