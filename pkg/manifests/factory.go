//go:generate go-bindata -o bindata.go -prefix "../../" -pkg manifests ../../assets/...

package manifests

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	PrometheusOperatorDeployment    = "assets/prometheus-operator/deployment.yaml"
	PrometheusOperatorService       = "assets/prometheus-operator/service.yaml"
	PrometheusOperatorCertsCABundle = "assets/prometheus-operator/operator-certs-ca-bundle.yaml"

	PrometheusHtpasswd               = "assets/prometheus/htpasswd-secret.yaml"
	PrometheusRBACProxySecret        = "assets/prometheus/kube-rbac-proxy-secret.yaml"
	PrometheusDeployment             = "assets/prometheus/prometheus.yaml"
	PrometheusProxySecret            = "assets/prometheus/proxy-secret.yaml"
	PrometheusService                = "assets/prometheus/service.yaml"
	PrometheusDatasourcesSecret      = "assets/prometheus/prometheus-datasources-secret.yaml"
	PrometheusServingCertsCABundle   = "assets/prometheus/serving-certs-ca-bundle.yaml"
	PrometheusKubeletServingCABundle = "assets/prometheus/kubelet-serving-ca-bundle.yaml"
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

func (f *Factory) PrometheusService() (*v1.Service, error) {
	s, err := f.NewService(MustAssetReader(PrometheusService))
	if err != nil {
		return nil, err
	}

	s.Namespace = f.namespace

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
	h := sha256.New()
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

// GeneratePassword returns a base64 encoded securely random bytes.
func GeneratePassword(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), err
}
