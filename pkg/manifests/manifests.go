//go:generate go-bindata -o bindata.go -pkg manifests ../../assets/...

package manifests

import (
	"bytes"
	"io"

	corev1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	PrometheusOperatorDeployment    = "assets/prometheus-operator/deployment.yaml"
	PrometheusOperatorService       = "assets/prometheus-operator/service.yaml"
	PrometheusOperatorCertsCABundle = "assets/prometheus-operator/operator-certs-ca-bundle.yaml"
)

func MustAssetReader(asset string) io.Reader {
	return bytes.NewReader(MustAsset(asset))
}

type Factory struct {
	namespace string
	config    Config
}

func NewFactory(namespace string, c *Config) *Factory {
	return &Factory{
		namespace: namespace,
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

func (f *Factory) NewPrometheusOperatorDeployment() (*appsv1.Deployment, error) {
	dep, err := f.NewDeployment(MustAssetReader(PrometheusOperatorDeployment))

	if len(f.config.MarketplaceOperatorConfig.PrometheusOperatorConfig.NodeSelector) > 0 {
		dep.Spec.Template.Spec.NodeSelector = f.config.MarketplaceOperatorConfig.PrometheusOperatorConfig.NodeSelector
	}

	if len(f.config.MarketplaceOperatorConfig.PrometheusOperatorConfig.Tolerations) > 0 {
		dep.Spec.Template.Spec.Tolerations = f.config.MarketplaceOperatorConfig.PrometheusOperatorConfig.Tolerations
	}

	return dep, err
}

func (f *Factory) NewPrometheusOperatorService() (*corev1.Service, error) {
	service, err := f.NewService(MustAssetReader(PrometheusOperatorService))

	return service, err
}

func (f *Factory) NewPrometheusOperatorCertsCABundle() (*corev1.ConfigMap, error) {
	return f.NewConfigMap(MustAssetReader(PrometheusOperatorCertsCABundle))
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
