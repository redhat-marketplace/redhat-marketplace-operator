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

package prometheus

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"emperror.dev/errors"
	"github.com/blang/semver"
	logf "github.com/go-logr/logr"
	v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus-operator/prometheus-operator/pkg/assets"
	"github.com/prometheus-operator/prometheus-operator/pkg/operator"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	kubernetesSDRoleEndpoint        = "endpoints"
	kubernetesSDRolePod             = "pod"
	kubernetesSDRoleIngress         = "ingress"
	defaultReplicaExternalLabelName = "prometheus_replica"
	confOutDir                      = "/etc/prometheus/config_out"
	tlsAssetsDir                    = "/etc/prometheus/certs"
	rulesDir                        = "/etc/prometheus/rules"
)

var (
	invalidLabelCharRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)
)

type configGenerator struct {
	logger logf.Logger
}

func NewConfigGenerator(logger logf.Logger) *configGenerator {
	cg := &configGenerator{
		logger: logger,
	}
	return cg
}

func sanitizeLabelName(name string) string {
	return invalidLabelCharRE.ReplaceAllString(name, "_")
}

func honorLabels(userHonorLabels, overrideHonorLabels bool) bool {
	if userHonorLabels && overrideHonorLabels {
		return false
	}
	return userHonorLabels
}

func (cg *configGenerator) GenerateConfig(
	p *v1.Prometheus,
	sMons map[string]*v1.ServiceMonitor,
	basicAuthSecrets map[string]assets.BasicAuthCredentials,
	bearerTokens map[string]assets.BearerToken,
	ruleConfigMapNames []string,
) ([]byte, error) {
	versionStr := p.Spec.Version
	if versionStr == "" {
		versionStr = operator.DefaultPrometheusVersion
	}

	version, err := semver.ParseTolerant(versionStr)
	if err != nil {
		return nil, errors.Wrap(err, "parse version")
	}

	cfg := yaml.MapSlice{}

	scrapeInterval := "30s"
	if p.Spec.ScrapeInterval != "" {
		scrapeInterval = p.Spec.ScrapeInterval
	}

	evaluationInterval := "30s"
	if p.Spec.EvaluationInterval != "" {
		evaluationInterval = p.Spec.EvaluationInterval
	}

	globalItems := yaml.MapSlice{
		{Key: "evaluation_interval", Value: evaluationInterval},
		{Key: "scrape_interval", Value: scrapeInterval},
		{Key: "external_labels", Value: buildExternalLabels(p)},
	}

	if version.GTE(semver.MustParse("2.16.0")) && p.Spec.QueryLogFile != "" {
		globalItems = append(globalItems, yaml.MapItem{
			Key: "query_log_file", Value: p.Spec.QueryLogFile,
		})
	}

	cfg = append(cfg, yaml.MapItem{Key: "global", Value: globalItems})

	ruleFilePaths := []string{}
	for _, name := range ruleConfigMapNames {
		ruleFilePaths = append(ruleFilePaths, rulesDir+"/"+name+"/*.yaml")
	}
	cfg = append(cfg, yaml.MapItem{
		Key:   "rule_files",
		Value: ruleFilePaths,
	})

	sMonIdentifiers := make([]string, len(sMons))
	i := 0
	for k := range sMons {
		sMonIdentifiers[i] = k
		i++
	}

	// Sorting ensures, that we always generate the config in the same order.
	sort.Strings(sMonIdentifiers)

	apiserverConfig := p.Spec.APIServerConfig

	var scrapeConfigs []yaml.MapSlice
	for _, identifier := range sMonIdentifiers {
		for i, ep := range sMons[identifier].Spec.Endpoints {
			scrapeConfigs = append(scrapeConfigs,
				cg.generateServiceMonitorConfig(
					version,
					sMons[identifier],
					ep, i,
					apiserverConfig,
					basicAuthSecrets,
					bearerTokens,
					p.Spec.OverrideHonorLabels,
					p.Spec.OverrideHonorTimestamps,
					p.Spec.IgnoreNamespaceSelectors))
		}
	}

	return yaml.Marshal(scrapeConfigs)
}

func honorTimestamps(cfg yaml.MapSlice, userHonorTimestamps *bool, overrideHonorTimestamps bool) yaml.MapSlice {
	// Ensuring backwards compatibility by checking if user set any option
	if userHonorTimestamps == nil && !overrideHonorTimestamps {
		return cfg
	}

	honor := false
	if userHonorTimestamps != nil {
		honor = *userHonorTimestamps
	}

	return append(cfg, yaml.MapItem{Key: "honor_timestamps", Value: honor && !overrideHonorTimestamps})
}

func addTLStoYaml(cfg yaml.MapSlice, namespace string, tls *v1.TLSConfig) yaml.MapSlice {
	if tls != nil {
		pathPrefix := path.Join(tlsAssetsDir, namespace)
		tlsConfig := yaml.MapSlice{
			{Key: "insecure_skip_verify", Value: tls.InsecureSkipVerify},
		}
		if tls.CAFile != "" {
			tlsConfig = append(tlsConfig, yaml.MapItem{Key: "ca_file", Value: tls.CAFile})
		}
		if tls.CA.Secret != nil {
			tlsConfig = append(tlsConfig, yaml.MapItem{Key: "ca_file", Value: pathPrefix + "_" + tls.CA.Secret.Name + "_" + tls.CA.Secret.Key})
		}
		if tls.CA.ConfigMap != nil {
			tlsConfig = append(tlsConfig, yaml.MapItem{Key: "ca_file", Value: pathPrefix + "_" + tls.CA.ConfigMap.Name + "_" + tls.CA.ConfigMap.Key})
		}
		if tls.CertFile != "" {
			tlsConfig = append(tlsConfig, yaml.MapItem{Key: "cert_file", Value: tls.CertFile})
		}
		if tls.Cert.Secret != nil {
			tlsConfig = append(tlsConfig, yaml.MapItem{Key: "cert_file", Value: pathPrefix + "_" + tls.Cert.Secret.Name + "_" + tls.Cert.Secret.Key})
		}
		if tls.Cert.ConfigMap != nil {
			tlsConfig = append(tlsConfig, yaml.MapItem{Key: "cert_file", Value: pathPrefix + "_" + tls.Cert.ConfigMap.Name + "_" + tls.Cert.ConfigMap.Key})
		}
		if tls.KeyFile != "" {
			tlsConfig = append(tlsConfig, yaml.MapItem{Key: "key_file", Value: tls.KeyFile})
		}
		if tls.KeySecret != nil {
			tlsConfig = append(tlsConfig, yaml.MapItem{Key: "key_file", Value: pathPrefix + "_" + tls.KeySecret.Name + "_" + tls.KeySecret.Key})
		}
		if tls.ServerName != "" {
			tlsConfig = append(tlsConfig, yaml.MapItem{Key: "server_name", Value: tls.ServerName})
		}
		cfg = append(cfg, yaml.MapItem{Key: "tls_config", Value: tlsConfig})
	}
	return cfg
}

func (cg *configGenerator) generateServiceMonitorConfig(
	version semver.Version,
	m *v1.ServiceMonitor,
	ep v1.Endpoint,
	i int,
	apiserverConfig *v1.APIServerConfig,
	basicAuthSecrets map[string]assets.BasicAuthCredentials,
	bearerTokens map[string]assets.BearerToken,
	overrideHonorLabels bool,
	overrideHonorTimestamps bool,
	ignoreNamespaceSelectors bool) yaml.MapSlice {

	hl := honorLabels(ep.HonorLabels, overrideHonorLabels)
	cfg := yaml.MapSlice{
		{
			Key:   "job_name",
			Value: fmt.Sprintf("%s/%s/%d", m.Namespace, m.Name, i),
		},
		{
			Key:   "honor_labels",
			Value: hl,
		},
	}
	if version.Major == 2 && version.Minor >= 9 {
		cfg = honorTimestamps(cfg, ep.HonorTimestamps, overrideHonorTimestamps)
	}

	if version.Major == 1 && version.Minor < 7 {
		if apiserverConfig != nil {
			cg.logger.Info("custom apiserver config is set but it will not take effect because prometheus version is < 1.7")
		}
		cfg = append(cfg, cg.generateK8SSDConfig(nil, nil, nil, kubernetesSDRoleEndpoint))
	} else {
		selectedNamespaces := getNamespacesFromNamespaceSelector(&m.Spec.NamespaceSelector, m.Namespace, ignoreNamespaceSelectors)
		cfg = append(cfg, cg.generateK8SSDConfig(selectedNamespaces, apiserverConfig, basicAuthSecrets, kubernetesSDRoleEndpoint))
	}

	if ep.Interval != "" {
		cfg = append(cfg, yaml.MapItem{Key: "scrape_interval", Value: ep.Interval})
	}
	if ep.ScrapeTimeout != "" {
		cfg = append(cfg, yaml.MapItem{Key: "scrape_timeout", Value: ep.ScrapeTimeout})
	}
	if ep.Path != "" {
		cfg = append(cfg, yaml.MapItem{Key: "metrics_path", Value: ep.Path})
	}
	if ep.ProxyURL != nil {
		cfg = append(cfg, yaml.MapItem{Key: "proxy_url", Value: ep.ProxyURL})
	}
	if ep.Params != nil {
		cfg = append(cfg, yaml.MapItem{Key: "params", Value: ep.Params})
	}
	if ep.Scheme != "" {
		cfg = append(cfg, yaml.MapItem{Key: "scheme", Value: ep.Scheme})
	}

	cfg = addTLStoYaml(cfg, m.Namespace, ep.TLSConfig)

	if ep.BearerTokenFile != "" {
		cfg = append(cfg, yaml.MapItem{Key: "bearer_token_file", Value: ep.BearerTokenFile})
	}

	if ep.BearerTokenSecret.Name != "" {
		if s, ok := bearerTokens[fmt.Sprintf("serviceMonitor/%s/%s/%d", m.Namespace, m.Name, i)]; ok {
			cfg = append(cfg, yaml.MapItem{Key: "bearer_token", Value: s})
		}
	}

	if ep.BasicAuth != nil {
		if s, ok := basicAuthSecrets[fmt.Sprintf("serviceMonitor/%s/%s/%d", m.Namespace, m.Name, i)]; ok {
			cfg = append(cfg, yaml.MapItem{
				Key: "basic_auth", Value: yaml.MapSlice{
					{Key: "username", Value: s.Username},
					{Key: "password", Value: s.Password},
				},
			})
		}
	}

	var relabelings []yaml.MapSlice

	// Filter targets by services selected by the monitor.

	// Exact label matches.
	var labelKeys []string
	for k := range m.Spec.Selector.MatchLabels {
		labelKeys = append(labelKeys, k)
	}
	sort.Strings(labelKeys)

	for _, k := range labelKeys {
		relabelings = append(relabelings, yaml.MapSlice{
			{Key: "action", Value: "keep"},
			{Key: "source_labels", Value: []string{"__meta_kubernetes_service_label_" + sanitizeLabelName(k)}},
			{Key: "regex", Value: m.Spec.Selector.MatchLabels[k]},
		})
	}
	// Set based label matching. We have to map the valid relations
	// `In`, `NotIn`, `Exists`, and `DoesNotExist`, into relabeling rules.
	for _, exp := range m.Spec.Selector.MatchExpressions {
		switch exp.Operator {
		case metav1.LabelSelectorOpIn:
			relabelings = append(relabelings, yaml.MapSlice{
				{Key: "action", Value: "keep"},
				{Key: "source_labels", Value: []string{"__meta_kubernetes_service_label_" + sanitizeLabelName(exp.Key)}},
				{Key: "regex", Value: strings.Join(exp.Values, "|")},
			})
		case metav1.LabelSelectorOpNotIn:
			relabelings = append(relabelings, yaml.MapSlice{
				{Key: "action", Value: "drop"},
				{Key: "source_labels", Value: []string{"__meta_kubernetes_service_label_" + sanitizeLabelName(exp.Key)}},
				{Key: "regex", Value: strings.Join(exp.Values, "|")},
			})
		case metav1.LabelSelectorOpExists:
			relabelings = append(relabelings, yaml.MapSlice{
				{Key: "action", Value: "keep"},
				{Key: "source_labels", Value: []string{"__meta_kubernetes_service_label_" + sanitizeLabelName(exp.Key)}},
				{Key: "regex", Value: ".+"},
			})
		case metav1.LabelSelectorOpDoesNotExist:
			relabelings = append(relabelings, yaml.MapSlice{
				{Key: "action", Value: "drop"},
				{Key: "source_labels", Value: []string{"__meta_kubernetes_service_label_" + sanitizeLabelName(exp.Key)}},
				{Key: "regex", Value: ".+"},
			})
		}
	}

	// Filter targets based on correct port for the endpoint.
	if ep.Port != "" {
		relabelings = append(relabelings, yaml.MapSlice{
			{Key: "action", Value: "keep"},
			{Key: "source_labels", Value: []string{"__meta_kubernetes_endpoint_port_name"}},
			{Key: "regex", Value: ep.Port},
		})
	} else if ep.TargetPort != nil {
		if ep.TargetPort.StrVal != "" {
			relabelings = append(relabelings, yaml.MapSlice{
				{Key: "action", Value: "keep"},
				{Key: "source_labels", Value: []string{"__meta_kubernetes_pod_container_port_name"}},
				{Key: "regex", Value: ep.TargetPort.String()},
			})
		} else if ep.TargetPort.IntVal != 0 {
			relabelings = append(relabelings, yaml.MapSlice{
				{Key: "action", Value: "keep"},
				{Key: "source_labels", Value: []string{"__meta_kubernetes_pod_container_port_number"}},
				{Key: "regex", Value: ep.TargetPort.String()},
			})
		}
	}

	// Relabel namespace and pod and service labels into proper labels.
	relabelings = append(relabelings, []yaml.MapSlice{
		{ // Relabel node labels for pre v2.3 meta labels
			{Key: "source_labels", Value: []string{"__meta_kubernetes_endpoint_address_target_kind", "__meta_kubernetes_endpoint_address_target_name"}},
			{Key: "separator", Value: ";"},
			{Key: "regex", Value: "Node;(.*)"},
			{Key: "replacement", Value: "${1}"},
			{Key: "target_label", Value: "node"},
		},
		{ // Relabel pod labels for >=v2.3 meta labels
			{Key: "source_labels", Value: []string{"__meta_kubernetes_endpoint_address_target_kind", "__meta_kubernetes_endpoint_address_target_name"}},
			{Key: "separator", Value: ";"},
			{Key: "regex", Value: "Pod;(.*)"},
			{Key: "replacement", Value: "${1}"},
			{Key: "target_label", Value: "pod"},
		},
		{
			{Key: "source_labels", Value: []string{"__meta_kubernetes_namespace"}},
			{Key: "target_label", Value: "namespace"},
		},
		{
			{Key: "source_labels", Value: []string{"__meta_kubernetes_service_name"}},
			{Key: "target_label", Value: "service"},
		},
		{
			{Key: "source_labels", Value: []string{"__meta_kubernetes_pod_name"}},
			{Key: "target_label", Value: "pod"},
		},
		{
			{Key: "source_labels", Value: []string{"__meta_kubernetes_pod_container_name"}},
			{Key: "target_label", Value: "container"},
		},
	}...)

	// Relabel targetLabels from Service onto target.
	for _, l := range m.Spec.TargetLabels {
		relabelings = append(relabelings, yaml.MapSlice{
			{Key: "source_labels", Value: []string{"__meta_kubernetes_service_label_" + sanitizeLabelName(l)}},
			{Key: "target_label", Value: sanitizeLabelName(l)},
			{Key: "regex", Value: "(.+)"},
			{Key: "replacement", Value: "${1}"},
		})
	}

	for _, l := range m.Spec.PodTargetLabels {
		relabelings = append(relabelings, yaml.MapSlice{
			{Key: "source_labels", Value: []string{"__meta_kubernetes_pod_label_" + sanitizeLabelName(l)}},
			{Key: "target_label", Value: sanitizeLabelName(l)},
			{Key: "regex", Value: "(.+)"},
			{Key: "replacement", Value: "${1}"},
		})
	}

	// By default, generate a safe job name from the service name.  We also keep
	// this around if a jobLabel is set in case the targets don't actually have a
	// value for it. A single service may potentially have multiple metrics
	// endpoints, therefore the endpoints labels is filled with the ports name or
	// as a fallback the port number.

	relabelings = append(relabelings, yaml.MapSlice{
		{Key: "source_labels", Value: []string{"__meta_kubernetes_service_name"}},
		{Key: "target_label", Value: "job"},
		{Key: "replacement", Value: "${1}"},
	})
	if m.Spec.JobLabel != "" {
		relabelings = append(relabelings, yaml.MapSlice{
			{Key: "source_labels", Value: []string{"__meta_kubernetes_service_label_" + sanitizeLabelName(m.Spec.JobLabel)}},
			{Key: "target_label", Value: "job"},
			{Key: "regex", Value: "(.+)"},
			{Key: "replacement", Value: "${1}"},
		})
	}

	if ep.Port != "" {
		relabelings = append(relabelings, yaml.MapSlice{
			{Key: "target_label", Value: "endpoint"},
			{Key: "replacement", Value: ep.Port},
		})
	} else if ep.TargetPort != nil && ep.TargetPort.String() != "" {
		relabelings = append(relabelings, yaml.MapSlice{
			{Key: "target_label", Value: "endpoint"},
			{Key: "replacement", Value: ep.TargetPort.String()},
		})
	}

	if ep.RelabelConfigs != nil {
		for _, c := range ep.RelabelConfigs {
			relabelings = append(relabelings, generateRelabelConfig(c))
		}
	}
	cfg = append(cfg, yaml.MapItem{Key: "relabel_configs", Value: relabelings})

	if ep.MetricRelabelConfigs != nil {
		var metricRelabelings []yaml.MapSlice
		for _, c := range ep.MetricRelabelConfigs {
			relabeling := generateRelabelConfig(c)

			metricRelabelings = append(metricRelabelings, relabeling)
		}
		cfg = append(cfg, yaml.MapItem{Key: "metric_relabel_configs", Value: metricRelabelings})
	}

	return cfg
}

// getNamespacesFromNamespaceSelector gets a list of namespaces to select based on
// the given namespace selector, the given default namespace, and whether to ignore namespace selectors
func getNamespacesFromNamespaceSelector(nsel *v1.NamespaceSelector, namespace string, ignoreNamespaceSelectors bool) []string {
	if ignoreNamespaceSelectors {
		return []string{namespace}
	} else if nsel.Any {
		return []string{}
	} else if len(nsel.MatchNames) == 0 {
		return []string{namespace}
	}
	return nsel.MatchNames
}

func (cg *configGenerator) generateK8SSDConfig(namespaces []string, apiserverConfig *v1.APIServerConfig, basicAuthSecrets map[string]assets.BasicAuthCredentials, role string) yaml.MapItem {
	k8sSDConfig := yaml.MapSlice{
		{
			Key:   "role",
			Value: role,
		},
	}

	if len(namespaces) != 0 {
		k8sSDConfig = append(k8sSDConfig, yaml.MapItem{
			Key: "namespaces",
			Value: yaml.MapSlice{
				{
					Key:   "names",
					Value: namespaces,
				},
			},
		})
	}

	if apiserverConfig != nil {
		k8sSDConfig = append(k8sSDConfig, yaml.MapItem{
			Key: "api_server", Value: apiserverConfig.Host,
		})

		if apiserverConfig.BasicAuth != nil && basicAuthSecrets != nil {
			if s, ok := basicAuthSecrets["apiserver"]; ok {
				k8sSDConfig = append(k8sSDConfig, yaml.MapItem{
					Key: "basic_auth", Value: yaml.MapSlice{
						{Key: "username", Value: s.Username},
						{Key: "password", Value: s.Password},
					},
				})
			}
		}

		if apiserverConfig.BearerToken != "" {
			k8sSDConfig = append(k8sSDConfig, yaml.MapItem{Key: "bearer_token", Value: apiserverConfig.BearerToken})
		}

		if apiserverConfig.BearerTokenFile != "" {
			k8sSDConfig = append(k8sSDConfig, yaml.MapItem{Key: "bearer_token_file", Value: apiserverConfig.BearerTokenFile})
		}

		// TODO: If we want to support secret refs for k8s service discovery tls
		// config as well, make sure to path the right namespace here.
		k8sSDConfig = addTLStoYaml(k8sSDConfig, "", apiserverConfig.TLSConfig)
	}

	return yaml.MapItem{
		Key: "kubernetes_sd_configs",
		Value: []yaml.MapSlice{
			k8sSDConfig,
		},
	}
}

func buildExternalLabels(p *v1.Prometheus) yaml.MapSlice {
	m := map[string]string{}

	// Use "prometheus" external label name by default if field is missing.
	// Do not add external label if field is set to empty string.
	prometheusExternalLabelName := "prometheus"
	if p.Spec.PrometheusExternalLabelName != nil {
		if *p.Spec.PrometheusExternalLabelName != "" {
			prometheusExternalLabelName = *p.Spec.PrometheusExternalLabelName
		} else {
			prometheusExternalLabelName = ""
		}
	}

	// Use defaultReplicaExternalLabelName constant by default if field is missing.
	// Do not add external label if field is set to empty string.
	replicaExternalLabelName := defaultReplicaExternalLabelName
	if p.Spec.ReplicaExternalLabelName != nil {
		if *p.Spec.ReplicaExternalLabelName != "" {
			replicaExternalLabelName = *p.Spec.ReplicaExternalLabelName
		} else {
			replicaExternalLabelName = ""
		}
	}

	if prometheusExternalLabelName != "" {
		m[prometheusExternalLabelName] = fmt.Sprintf("%s/%s", p.Namespace, p.Name)
	}

	if replicaExternalLabelName != "" {
		m[replicaExternalLabelName] = "$(POD_NAME)"
	}

	for n, v := range p.Spec.ExternalLabels {
		m[n] = v
	}
	return stringMapToMapSlice(m)
}

func stringMapToMapSlice(m map[string]string) yaml.MapSlice {
	res := yaml.MapSlice{}
	ks := make([]string, 0)

	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)

	for _, k := range ks {
		res = append(res, yaml.MapItem{Key: k, Value: m[k]})
	}

	return res
}

func generateRelabelConfig(c *v1.RelabelConfig) yaml.MapSlice {
	relabeling := yaml.MapSlice{}

	if len(c.SourceLabels) > 0 {
		relabeling = append(relabeling, yaml.MapItem{Key: "source_labels", Value: c.SourceLabels})
	}

	if c.Separator != "" {
		relabeling = append(relabeling, yaml.MapItem{Key: "separator", Value: c.Separator})
	}

	if c.TargetLabel != "" {
		relabeling = append(relabeling, yaml.MapItem{Key: "target_label", Value: c.TargetLabel})
	}

	if c.Regex != "" {
		relabeling = append(relabeling, yaml.MapItem{Key: "regex", Value: c.Regex})
	}

	if c.Modulus != uint64(0) {
		relabeling = append(relabeling, yaml.MapItem{Key: "modulus", Value: c.Modulus})
	}

	if c.Replacement != "" {
		relabeling = append(relabeling, yaml.MapItem{Key: "replacement", Value: c.Replacement})
	}

	if c.Action != "" {
		relabeling = append(relabeling, yaml.MapItem{Key: "action", Value: c.Action})
	}

	return relabeling
}
