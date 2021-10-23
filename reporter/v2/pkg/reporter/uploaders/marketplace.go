package reporters

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/version"
	"golang.org/x/net/http/httpproxy"
	"golang.org/x/net/http2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/jsonpath"
)

type MarketplaceUploaderConfig struct {
	URL                 string   `json:"url"`
	Token               string   `json:"-"`
	OperatorVersion     string   `json:"operatorVersion"`
	ClusterID           string   `json:"clusterID"`
	AdditionalCertFiles []string `json:"additionalCertFiles,omitempty"`
	httpVersion         *int
}

type MarketplaceUploader struct {
	MarketplaceUploaderConfig
	client *http.Client
}

var _ Uploader = &MarketplaceUploader{}

func NewMarketplaceUploader(
	config *MarketplaceUploaderConfig,
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

	return &MarketplaceUploader{
		client:                    client,
		MarketplaceUploaderConfig: *config,
	}, nil
}

func provideProductionConfig(
	ctx context.Context,
	cc ClientCommandRunner,
	log logr.Logger,
) (*MarketplaceUploaderConfig, error) {
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

	return &MarketplaceUploaderConfig{
		URL:             "https://cloud.redhat.com",
		ClusterID:       string(clusterVersion.Spec.ClusterID), // get from cluster
		OperatorVersion: version.Version,
		Token:           cloudToken, // get from secret
	}, nil
}
