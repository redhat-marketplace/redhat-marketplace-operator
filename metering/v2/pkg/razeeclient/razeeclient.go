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

package razeeclient

import (
	"context"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"emperror.dev/errors"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Get the base URL & Org Key from the rhm-operator-secret
func GetRazeeDashKeys(client client.Client, namespace string) ([]byte, []byte, error) {
	var url []byte
	var key []byte

	rhmOperatorSecret := &corev1.Secret{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Name:      utils.RHM_OPERATOR_SECRET_NAME,
		Namespace: namespace,
	}, rhmOperatorSecret)
	if err != nil {
		return url, key, err
	}

	url, err = utils.ExtractCredKey(rhmOperatorSecret, corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: utils.RHM_OPERATOR_SECRET_NAME,
		},
		Key: utils.RAZEE_DASH_URL_FIELD,
	})
	if err != nil {
		return url, key, err
	}

	key, err = utils.ExtractCredKey(rhmOperatorSecret, corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: utils.RHM_OPERATOR_SECRET_NAME,
		},
		Key: utils.RAZEE_DASH_ORG_KEY_FIELD,
	})
	if err != nil {
		return url, key, err
	}

	return url, key, nil
}

func PostToRazeeDash(url string, body io.Reader, header http.Header) error {

	client := retryablehttp.NewClient()
	client.RetryWaitMin = 3000 * time.Millisecond
	client.RetryWaitMax = 5000 * time.Millisecond
	client.RetryMax = 5

	req, err := retryablehttp.NewRequest("POST", url, body)
	if err != nil {
		return errors.Wrap(err, "Error constructing POST request")
	}

	req.Header = header

	if os.Getenv("INSECURE_CLIENT") == "true" {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client.HTTPClient.Transport = tr
	}

	_, err = client.Do(req)
	if err != nil {
		return errors.Wrap(err, "POST error")
	}

	return nil
}

func GetNamespace() string {
	if ns, ok := os.LookupEnv("POD_NAMESPACE"); ok {
		return ns
	}

	// Fall back to the namespace associated with the service account token, if available
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}

	return "default"
}

func GetClusterID(client client.Client) (string, error) {
	instance := &openshiftconfigv1.ClusterVersion{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: "version"}, instance)
	if err != nil {
		return "", err
	}
	return string(instance.Spec.ClusterID), nil
}
