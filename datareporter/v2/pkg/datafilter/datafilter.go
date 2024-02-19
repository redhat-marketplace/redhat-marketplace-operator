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
	"net/http"
	"sync"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/events"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/selector"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/transformer"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/uploader"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DataFilter struct {
	Selector     selector.DataFilterSelector
	Transformer  transformer.Transformer
	Destinations []*uploader.Uploader
}

// When DataReporterConfig reconciles, DataFilters must be locked & rebuilt, pausing any request processing
// The DataFilter may change, or it may be an underlying Secret/Configmap responsible for transforms & headers
// The HttpClient transport TLSConfig may change

type DataFilters struct {
	Log         logr.Logger
	k8sClient   client.Client
	httpClient  *http.Client
	dataFilters []DataFilter
	mu          sync.RWMutex
}

func NewDataFilters(
	log logr.Logger,
	k8sClient client.Client,
	httpClient *http.Client,
) *DataFilters {
	return &DataFilters{Log: log, k8sClient: k8sClient, httpClient: httpClient}
}

// Build
// Creates the underlying DataFilters from DataReporterConfig Spec
// Updates the httpClient transport
func (d *DataFilters) Build(drc *v1alpha1.DataReporterConfig) error {

	d.mu.RLock()
	defer d.mu.RUnlock()

	d.updateHttpClient(drc)

	err := d.updateDataFilters(drc)
	if err != nil {
		return err
	}

	return nil
}

func (d *DataFilters) FilterAndUpload(event events.Event) []int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var statusCodes []int
	for _, df := range d.dataFilters {
		if df.Selector.Matches(event) {

			var transformedJson, err = df.Transformer.Transform(event.RawMessage)
			if err == nil {
				event.RawMessage = transformedJson
			}

			// Upload to Destinations
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
			wg.Wait()
			close(statusCodeChan)

			statusCodes = make([]int, len(df.Destinations))
			for statusCode := range statusCodeChan {
				statusCodes = append(statusCodes, statusCode)
			}

			return statusCodes
		}
	}

	return statusCodes
}

func (d *DataFilters) getMapFromSecret(namespacedName types.NamespacedName) (map[string]string, error) {
	secretMap := make(map[string]string)

	if len(namespacedName.Name) != 0 {
		secret := corev1.Secret{}
		err := d.k8sClient.Get(context.TODO(), namespacedName, &secret)
		if err != nil {
			return secretMap, err
		}

		for k, v := range secret.Data {
			secretMap[k] = string(v)
		}
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

			// get transformer text
			transformerText, err := d.getMapFromSecret(
				types.NamespacedName{Name: dest.Transformer.ConfigMapKeyRef.Name, Namespace: drc.Namespace})
			if err != nil {
				return errors.Wrap(err, "could not get transformer text")
			}

			t, err := transformer.NewTransformer(dest.Transformer.TransformerType, transformerText[dest.Transformer.ConfigMapKeyRef.Key])
			if err != nil {
				return errors.Wrap(err, "could not initialize transformer")
			}

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

			u, err := uploader.NewUploader(nil, &config, &t)
			if err != nil {
				return err
			}

			destinations = append(destinations, u)
		}

		// get transformer text
		transformerText, err := d.getMapFromSecret(
			types.NamespacedName{Name: df.Transformer.ConfigMapKeyRef.Name, Namespace: drc.Namespace})
		if err != nil {
			return errors.Wrap(err, "could not get transformer text")
		}

		t, err := transformer.NewTransformer(df.Transformer.TransformerType, transformerText[df.Transformer.ConfigMapKeyRef.Key])
		if err != nil {
			return errors.Wrap(err, "could not initialize transformer")
		}

		newDataFilters = append(newDataFilters, DataFilter{Selector: sel, Transformer: t, Destinations: destinations})
	}

	d.dataFilters = newDataFilters

	return nil
}

func (d *DataFilters) updateHttpClient(drc *v1alpha1.DataReporterConfig) {

	newTLSClientConfig := &tls.Config{
		InsecureSkipVerify: drc.Spec.TLSConfig.InsecureSkipVerify,
	}
	if tpt, ok := d.httpClient.Transport.(*http.Transport); ok {
		tpt.TLSClientConfig = newTLSClientConfig
	}
}
