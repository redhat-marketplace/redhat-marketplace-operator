package uploaders

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/version"
	"golang.org/x/net/http/httpproxy"
	"golang.org/x/net/http2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/jsonpath"
)

type RedHatInsightsUploaderConfig struct {
	URL                 string   `json:"url"`
	Token               string   `json:"-"`
	OperatorVersion     string   `json:"operatorVersion"`
	ClusterID           string   `json:"clusterID"`
	AdditionalCertFiles []string `json:"additionalCertFiles,omitempty"`
	httpVersion         *int
}

type RedHatInsightsUploader struct {
	RedHatInsightsUploaderConfig
	client *http.Client
}

var _ Uploader = &RedHatInsightsUploader{}

func NewRedHatInsightsUploader(
	config *RedHatInsightsUploaderConfig,
) (Uploader, error) {
	tlsConfig, err := prometheus.GenerateCACertPool(config.AdditionalCertFiles...)

	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	config.httpVersion = ptr.Int(1)

	// default to 2 unless otherwise overridden
	if config.httpVersion == nil {
		config.httpVersion = ptr.Int(2)
	}

	proxyCfg := httpproxy.FromEnvironment()
	if proxyCfg.HTTPProxy != "" || proxyCfg.HTTPSProxy != "" {
		config.httpVersion = ptr.Int(1)
	}

	// Use the proper transport in the client
	switch *config.httpVersion {
	case 1:
		client.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
			Proxy:           http.ProxyFromEnvironment,
		}
	case 2:
		client.Transport = &http2.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	return &RedHatInsightsUploader{
		client:                       client,
		RedHatInsightsUploaderConfig: *config,
	}, nil
}

func provideProductionInsightsConfig(
	ctx context.Context,
	cc ClientCommandRunner,
	log logr.Logger,
) (*RedHatInsightsUploaderConfig, error) {
	secret := &corev1.Secret{}
	clusterVersion := &openshiftconfigv1.ClusterVersion{}
	result, _ := cc.Do(ctx,
		GetAction(types.NamespacedName{
			Name:      "pull-secret",
			Namespace: "openshift-config",
		}, secret),
		GetAction(types.NamespacedName{
			Name: "version",
		}, clusterVersion))

	if !result.Is(Continue) {
		return nil, result
	}

	dockerConfigBytes, ok := secret.Data[".dockerconfigjson"]

	if !ok {
		return nil, errors.WithStack(&ReportJobError{
			ErrorMessage: "failed to find .dockerconfigjson in secret openshift-config/pull-secret",
			Err:          errors.New("report job error"),
		})
	}

	var dockerObj interface{}
	err := json.Unmarshal(dockerConfigBytes, &dockerObj)

	if err != nil {
		return nil, errors.WithStack(&ReportJobError{
			ErrorMessage: "failed to unmarshal .dockerconfigjson from secret openshift-config/pull-secret",
			Err:          err,
		})
	}

	cloudAuthPath := jsonpath.New("cloudauthpath")
	err = cloudAuthPath.Parse(`{.auths.cloud\.openshift\.com.auth}`)

	if err != nil {
		return nil, errors.WithStack(&ReportJobError{
			ErrorMessage: "failed to parse auth token in .dockerconfigjson from secret openshift-config/pull-secret",
			Err:          err,
		})
	}

	buf := new(bytes.Buffer)
	err = cloudAuthPath.Execute(buf, dockerObj)

	if err != nil {
		return nil, errors.WithStack(&ReportJobError{
			ErrorMessage: "failed to template auth token in .dockerconfigjson from secret openshift-config/pull-secret",
			Err:          err,
		})
	}

	cloudToken := buf.String()

	return &RedHatInsightsUploaderConfig{
		URL:             "https://cloud.redhat.com",
		ClusterID:       string(clusterVersion.Spec.ClusterID), // get from cluster
		OperatorVersion: version.Version,
		Token:           cloudToken, // get from secret
	}, nil
}
