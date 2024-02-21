// Copyright 2024 IBM Corp.
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

package datafilter

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"sync"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/events"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/selector"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/transformer"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/uploader"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sapiflag "k8s.io/component-base/cli/flag"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultManifestType = "dataReporter"
)

type DataFilter struct {
	Selector     selector.DataFilterSelector
	Transformer  transformer.Transformer
	Destinations []*uploader.Uploader
	ManifestType string
}

// When DataReporterConfig reconciles, DataFilters must be locked & rebuilt, pausing any request processing
// The DataFilter may change, or it may be an underlying Secret/Configmap responsible for transforms & headers
// The HttpClient transport TLSConfig may change

type DataFilters struct {
	Log              logr.Logger
	k8sClient        client.Client
	httpClient       *http.Client
	dataFilters      []DataFilter
	eventEngine      *events.EventEngine
	eventConfig      *events.Config
	apiHandlerConfig *v1alpha1.ApiHandlerConfig
	mu               sync.RWMutex
}

func NewDataFilters(
	log logr.Logger,
	k8sClient client.Client,
	httpClient *http.Client,
	eventEngine *events.EventEngine,
	eventConfig *events.Config,
	apiHandlerConfig *v1alpha1.ApiHandlerConfig,
) *DataFilters {
	return &DataFilters{
		Log:              log,
		k8sClient:        k8sClient,
		httpClient:       httpClient,
		eventEngine:      eventEngine,
		eventConfig:      eventConfig,
		apiHandlerConfig: apiHandlerConfig,
	}
}

// Build
// Creates the underlying DataFilters from DataReporterConfig Spec
// Updates the httpClient transport
func (d *DataFilters) Build(drc *v1alpha1.DataReporterConfig) error {

	d.mu.RLock()
	defer d.mu.RUnlock()

	// Update API Handler
	if drc.Spec.ConfirmDelivery != nil {
		d.apiHandlerConfig.ConfirmDelivery = drc.Spec.ConfirmDelivery
	}

	if err := d.updateHttpClient(drc); err != nil {
		return errors.Wrap(err, "failed to update http client")
	}

	if err := d.updateDataFilters(drc); err != nil {
		return errors.Wrap(err, "failed to update datafilters")
	}

	return nil
}

func (d *DataFilters) FilterAndUpload(event events.Event) []int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var statusCodes []int
	for _, df := range d.dataFilters {
		if df.Selector.Matches(event) { // Send to the destinations of the first matched filter and return
			// Transform and Upload for DataService
			// Set the ManifestType
			if len(df.ManifestType) == 0 {
				event.ManifestType = defaultManifestType
			} else {
				event.ManifestType = df.ManifestType
			}

			dsEvent := event
			var transformedJson, err = df.Transformer.Transform(dsEvent.RawMessage)
			if err == nil {
				dsEvent.RawMessage = transformedJson
			}

			// Send to DataService. If not deliverable, return bad code and skip altDestinations
			if ptr.ToBool(d.apiHandlerConfig.ConfirmDelivery) {
				if err := d.eventEngine.ProcessorSender.ReportOne(dsEvent); err != nil {
					statusCodes = append(statusCodes, http.StatusBadGateway)
					return statusCodes
				}
			} else {
				d.eventEngine.EventChan <- dsEvent
				statusCodes = append(statusCodes, http.StatusOK)
			}

			// Transform and Upload for Destinations
			statusCodeChan := make(chan int)
			var wg sync.WaitGroup
			for _, dest := range df.Destinations {
				wg.Add(1)
				go func(statusCodeChan chan int, dest *uploader.Uploader) {
					defer wg.Done()
					statusCode, err := dest.TransformAndUpload(event.RawMessage)
					if err != nil {
						d.Log.Error(err, "failed to filter and upload", "statusCode", statusCode)
					}
					statusCodeChan <- statusCode
				}(statusCodeChan, dest)
			}
			go func() {
				wg.Wait()
				close(statusCodeChan)
			}()

			for statusCode := range statusCodeChan {
				statusCodes = append(statusCodes, statusCode)
			}

			return statusCodes
		}
	}

	// No matches, default send as-is to DataService
	event.ManifestType = defaultManifestType

	if ptr.ToBool(d.apiHandlerConfig.ConfirmDelivery) {
		if err := d.eventEngine.ProcessorSender.ReportOne(event); err != nil {
			statusCodes = append(statusCodes, http.StatusBadGateway)
		}
	} else {
		d.eventEngine.EventChan <- event
		statusCodes = append(statusCodes, http.StatusOK)
	}

	return statusCodes
}

func (d *DataFilters) getMapFromSecret(namespacedName types.NamespacedName) (map[string]string, error) {
	secretMap := make(map[string]string)

	secret := &corev1.Secret{}
	if err := d.k8sClient.Get(context.TODO(), namespacedName, secret); err != nil {
		return secretMap, err
	}

	for k, v := range secret.Data {
		secretMap[k] = string(v)
	}

	return secretMap, nil
}

func (d *DataFilters) getMapFromConfigMap(namespacedName types.NamespacedName) (map[string]string, error) {
	dataMap := make(map[string]string)

	if len(namespacedName.Name) != 0 {
		configMap := corev1.ConfigMap{}
		err := d.k8sClient.Get(context.TODO(), namespacedName, &configMap)
		if err != nil {
			return dataMap, err
		}

		for k, v := range configMap.Data {
			dataMap[k] = string(v)
		}
	}

	return dataMap, nil
}

func (d *DataFilters) updateDataFilters(drc *v1alpha1.DataReporterConfig) error {
	var newDataFilters []DataFilter

	for _, df := range drc.Spec.DataFilters {

		sel, err := selector.NewDataFilterSelector(df.Selector)
		if err != nil {
			return err
		}

		var destinations []*uploader.Uploader
		for _, dest := range df.AltDestinations {

			destHeader := make(map[string]string)
			if len(dest.Header.Secret.Name) != 0 {
				destHeader, err = d.getMapFromSecret(
					types.NamespacedName{Name: dest.Header.Secret.Name, Namespace: drc.Namespace})
				if err != nil {
					return errors.Wrap(err, "could not get destination header secret")
				}
			}

			authHeader := make(map[string]string)
			if len(dest.Authorization.Header.Secret.Name) != 0 {
				authHeader, err = d.getMapFromSecret(
					types.NamespacedName{Name: dest.Authorization.Header.Secret.Name, Namespace: drc.Namespace})
				if err != nil {
					return errors.Wrap(err, "could not get authorization header secret")
				}
			}

			var authBodyData []byte
			if dest.Authorization.BodyData.SecretKeyRef != nil {
				authBodyData, err = d.getSecretData(
					types.NamespacedName{Name: dest.Authorization.BodyData.SecretKeyRef.Name, Namespace: drc.Namespace},
					dest.Authorization.BodyData.SecretKeyRef.Key)
				if err != nil {
					return errors.Wrap(err, "could not get authorization body data")
				}
			}

			tConfig, err := d.getConfigMapData(
				types.NamespacedName{Name: dest.Transformer.ConfigMapKeyRef.Name, Namespace: drc.Namespace},
				dest.Transformer.ConfigMapKeyRef.Key)
			if err != nil {
				return errors.WrapWithDetails(err, "could not get transformer config",
					"configmap", df.Transformer.ConfigMapKeyRef.Name,
					"key", df.Transformer.ConfigMapKeyRef.Key,
				)
			}

			t, err := transformer.NewTransformer(df.Transformer.TransformerType, tConfig)
			if err != nil {
				return errors.Wrap(err, "could not initialize transformer")
			}

			config := uploader.Config{
				Log:                  d.Log,
				DestURL:              dest.URL,
				DestHeader:           destHeader,
				DestURLSuffixExpr:    dest.URLSuffixExpr,
				AuthURL:              dest.Authorization.URL,
				AuthHeader:           authHeader,
				AuthDestHeader:       dest.Authorization.AuthDestHeader,
				AuthDestHeaderPrefix: dest.Authorization.AuthDestHeaderPrefix,
				AuthTokenExpr:        dest.Authorization.TokenExpr,
				AuthBodyData:         authBodyData,
			}

			u, err := uploader.NewUploader(d.httpClient, &config, &t)
			if err != nil {
				return err
			}

			destinations = append(destinations, u)
		}

		tConfig, err := d.getConfigMapData(
			types.NamespacedName{Name: df.Transformer.ConfigMapKeyRef.Name, Namespace: drc.Namespace},
			df.Transformer.ConfigMapKeyRef.Key)
		if err != nil {
			return errors.WrapWithDetails(err, "could not get transformer config",
				"configmap", df.Transformer.ConfigMapKeyRef.Name,
				"key", df.Transformer.ConfigMapKeyRef.Key,
			)
		}

		t, err := transformer.NewTransformer(df.Transformer.TransformerType, tConfig)
		if err != nil {
			return errors.Wrap(err, "could not initialize transformer")
		}

		newDataFilters = append(newDataFilters, DataFilter{Selector: sel, Transformer: t, Destinations: destinations})
	}

	d.dataFilters = newDataFilters

	return nil
}

// Failure to get a new valid TLS Config will result in retaining the current TLS Config for the http transport
func (d *DataFilters) updateHttpClient(drc *v1alpha1.DataReporterConfig) error {
	// The retryablehttp StandardClient masquerading as a http.client
	// should have a Transport of retryablehttp.RoundTripper
	if rt, ok := d.httpClient.Transport.(*retryablehttp.RoundTripper); ok {
		// The underlying HTTPClient has the TLS Config
		if tpt, ok := rt.Client.HTTPClient.Transport.(*http.Transport); ok {
			tlsConfig, err := d.getTLSConfig(drc)
			if err != nil {
				return errors.Wrap(err, "error creating tls config")
			}
			tpt.TLSClientConfig = tlsConfig
			return nil
		} else {
			return errors.New("httpclient transport is not of type http.Transport")
		}
	} else {
		// Log only, as this is the case with httpTest
		d.Log.Error(errors.New("httpclient transport is not of type retryablehttp.roundtripper"), "expected httpclient transport type retryablehttp.roundtripper")
		return nil
	}
}

func (d *DataFilters) getTLSConfig(drc *v1alpha1.DataReporterConfig) (*tls.Config, error) {

	// nil, default configuration is used
	if drc.Spec.TLSConfig == nil {
		return nil, nil
	}

	tlsConfig := &tls.Config{}

	// InsecureSkipVerify
	if drc.Spec.TLSConfig.InsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.ClientAuth = 0
		return tlsConfig, nil
	}

	// CA Certs
	caCertPool, err := x509.SystemCertPool()
	if err != nil {
		return tlsConfig, errors.Wrap(err, "failed to get system cert pool")
	}

	for _, sks := range drc.Spec.TLSConfig.CACerts {
		certData, err := d.getSecretData(types.NamespacedName{Name: sks.Name, Namespace: drc.Namespace}, sks.Key)
		if err != nil {
			return tlsConfig, errors.Wrap(err, "error getting cacert data")
		}
		if !caCertPool.AppendCertsFromPEM(certData) {
			return tlsConfig, errors.NewWithDetails("cacert secret data could not be appended to cert pool", "secret", sks.Name, "key", sks.Key)
		}
	}
	tlsConfig.RootCAs = caCertPool

	// Client Certificates
	certificates := []tls.Certificate{}
	for _, pair := range drc.Spec.TLSConfig.Certificates {
		// Must have a pair
		if pair.ClientCert.SecretKeyRef == nil || pair.ClientKey.SecretKeyRef == nil {
			return tlsConfig, errors.New("missing key/cert pair for tlsconfig certificates")
		}
		// Key
		keyData, err := d.getSecretData(types.NamespacedName{Name: pair.ClientKey.SecretKeyRef.Name, Namespace: drc.Namespace}, pair.ClientKey.SecretKeyRef.Key)
		if err != nil {
			return tlsConfig, errors.Wrap(err, "error getting client key data")
		}
		// Cert
		certData, err := d.getSecretData(types.NamespacedName{Name: pair.ClientCert.SecretKeyRef.Name, Namespace: drc.Namespace}, pair.ClientCert.SecretKeyRef.Key)
		if err != nil {
			return tlsConfig, errors.Wrap(err, "error getting client cert data")
		}
		// Pair
		cert, err := tls.X509KeyPair(certData, keyData)
		if err != nil {
			return tlsConfig, errors.Wrap(err, "error creating cert from client x509 key pair")
		}
		// Append
		certificates = append(certificates, cert)
	}
	tlsConfig.Certificates = certificates

	// MinVersion
	if len(drc.Spec.TLSConfig.MinVersion) != 0 {
		minVersion, err := k8sapiflag.TLSVersion(drc.Spec.TLSConfig.MinVersion)
		if err != nil {
			return tlsConfig, errors.WrapWithDetails(err, "failed to parse tls config minversion", "minversion", drc.Spec.TLSConfig.MinVersion)
		}
		tlsConfig.MinVersion = minVersion
	}

	// CipherSuites
	if len(drc.Spec.TLSConfig.CipherSuites) != 0 {
		cipherSuites, err := k8sapiflag.TLSCipherSuites(drc.Spec.TLSConfig.CipherSuites)
		if err != nil {
			return tlsConfig, errors.Wrap(err, "failed to convert tls config cipher suite name to id")
		}
		tlsConfig.CipherSuites = cipherSuites
	}

	return tlsConfig, nil
}

func (d *DataFilters) getSecretData(nsn types.NamespacedName, key string) ([]byte, error) {
	var data []byte
	secret := &corev1.Secret{}
	if err := d.k8sClient.Get(context.TODO(), nsn, secret); err != nil {
		return data, errors.Wrap(err, "error getting secret")
	}
	data, ok := secret.Data[key]
	if !ok {
		return data, errors.NewWithDetails("key not found for secret", "secret", nsn.Name, "key", key)
	}
	return data, nil
}

func (d *DataFilters) getConfigMapData(nsn types.NamespacedName, key string) (string, error) {
	var data string
	cm := &corev1.ConfigMap{}
	if err := d.k8sClient.Get(context.TODO(), nsn, cm); err != nil {
		return data, errors.Wrap(err, "error getting configmap")
	}
	data, ok := cm.Data[key]
	if !ok {
		return data, errors.NewWithDetails("key not found for configmap", "configmap", nsn.Name, "key", key)
	}
	return data, nil
}
