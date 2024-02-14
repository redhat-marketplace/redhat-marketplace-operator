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
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/events"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/selector"
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
	Destinations []*uploader.Uploader
	ManifestType string
}

// When DataReporterConfig reconciles, DataFilters must be locked & rebuilt, pausing any request processing
// The DataFilter may change, or it may be an underlying Secret/Configmap responsible for transforms & headers
// The HttpClient transport TLSConfig may change

type DataFilters struct {
	Log         logr.Logger
	k8sClient   client.Client
	httpClient  *http.Client
	dataFilters []DataFilter
	eventEngine *events.EventEngine
	eventConfig *events.Config
	mu          sync.RWMutex
}

func NewDataFilters(
	log logr.Logger,
	k8sClient client.Client,
	httpClient *http.Client,
	eventEngine *events.EventEngine,
	eventConfig *events.Config,
) *DataFilters {
	return &DataFilters{
		Log:         log,
		k8sClient:   k8sClient,
		httpClient:  httpClient,
		eventEngine: eventEngine,
		eventConfig: eventConfig,
	}
}

// Build
// Creates the underlying DataFilters from DataReporterConfig Spec
// Updates the httpClient transport
func (d *DataFilters) Build(drc *v1alpha1.DataReporterConfig) error {

	d.mu.RLock()
	defer d.mu.RUnlock()

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

			statusCodeChan := make(chan int)

			// Transform and Upload for DataService

			// Set the ManifestType
			if len(df.ManifestType) == 0 {
				event.ManifestType = defaultManifestType
			} else {
				event.ManifestType = df.ManifestType
			}

			// TODO: Transform

			// TODO: Do we need to add ConfirmDelivery option for DataService (1 event: 1 report)

			// Send event to DataService
			d.eventEngine.EventChan <- event

			// Transform and Upload for Destinations
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
			wg.Wait()
			close(statusCodeChan)

			statusCodes = make([]int, len(df.Destinations))
			for statusCode := range statusCodeChan {
				statusCodes = append(statusCodes, statusCode)
			}
			return statusCodes
		}
	}

	// No matches, default send as-is to DataService
	event.ManifestType = defaultManifestType
	d.eventEngine.EventChan <- event
	statusCodes = append(statusCodes, http.StatusOK)

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

func (d *DataFilters) updateDataFilters(drc *v1alpha1.DataReporterConfig) error {
	var newDataFilters []DataFilter

	for _, df := range drc.Spec.DataFilters {

		sel, err := selector.NewDataFilterSelector(df.Selector)
		if err != nil {
			return err
		}

		var destinations []*uploader.Uploader
		for _, dest := range df.AltDestinations {

			destHeader, err := d.getMapFromSecret(
				types.NamespacedName{Name: dest.HeaderSecret.SecretRef.Name, Namespace: drc.Namespace})
			if err != nil {
				return errors.Wrap(err, "could not get destination header secret")
			}

			authHeader, err := d.getMapFromSecret(
				types.NamespacedName{Name: dest.Authorization.HeaderSecret.SecretRef.Name, Namespace: drc.Namespace})
			if err != nil {
				return errors.Wrap(err, "could not get authorization header secret")
			}

			// TODO Add Transformer

			config := uploader.Config{
				DestURL:              dest.URL,
				DestHeader:           destHeader,
				DestURLSuffixExpr:    dest.URLSuffixExpr,
				AuthURL:              dest.Authorization.URL,
				AuthHeader:           authHeader,
				AuthDestHeader:       dest.Authorization.AuthDestHeader,
				AuthDestHeaderPrefix: dest.Authorization.AuthDestHeaderPrefix,
				AuthTokenExpr:        dest.Authorization.TokenExpr,
			}

			u, err := uploader.NewUploader(nil, &config)
			if err != nil {
				return err
			}

			destinations = append(destinations, u)
		}

		newDataFilters = append(newDataFilters, DataFilter{Selector: sel, Destinations: destinations})
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
		return errors.New("httpclient transport is not of type retryablehttp.roundtripper")
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
		if pair.ClientCert == nil || pair.ClientKey == nil {
			return tlsConfig, errors.New("missing key/cert pair for tlsconfig certificates")
		}
		// Key
		keyData, err := d.getSecretData(types.NamespacedName{Name: pair.ClientKey.Name, Namespace: drc.Namespace}, pair.ClientKey.Key)
		if err != nil {
			return tlsConfig, errors.Wrap(err, "error getting client key data")
		}
		// Cert
		certData, err := d.getSecretData(types.NamespacedName{Name: pair.ClientCert.Name, Namespace: drc.Namespace}, pair.ClientCert.Key)
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
