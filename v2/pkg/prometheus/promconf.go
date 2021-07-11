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
	"context"
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus-operator/prometheus-operator/pkg/assets"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func LoadBasicAuthSecrets(
	client client.Client,
	mons map[string]*monitoringv1.ServiceMonitor,
	remoteReads []monitoringv1.RemoteReadSpec,
	remoteWrites []monitoringv1.RemoteWriteSpec,
	apiserverConfig *monitoringv1.APIServerConfig,
	SecretsInPromNS *corev1.SecretList,
) (map[string]assets.BasicAuthCredentials, error) {

	secrets := map[string]assets.BasicAuthCredentials{}
	nsSecretCache := make(map[string]*corev1.Secret)
	for _, mon := range mons {
		for i, ep := range mon.Spec.Endpoints {
			if ep.BasicAuth != nil {
				credentials, err := loadBasicAuthSecretFromAPI(ep.BasicAuth, client, mon.Namespace, nsSecretCache)
				if err != nil {
					return nil, fmt.Errorf("could not generate basicAuth for servicemonitor %s. %s", mon.Name, err)
				}
				secrets[fmt.Sprintf("serviceMonitor/%s/%s/%d", mon.Namespace, mon.Name, i)] = credentials
			}

		}
	}

	for i, remote := range remoteReads {
		if remote.BasicAuth != nil {
			credentials, err := loadBasicAuthSecret(remote.BasicAuth, SecretsInPromNS)
			if err != nil {
				return nil, fmt.Errorf("could not generate basicAuth for remote_read config %d. %s", i, err)
			}
			secrets[fmt.Sprintf("remoteRead/%d", i)] = credentials
		}
	}

	for i, remote := range remoteWrites {
		if remote.BasicAuth != nil {
			credentials, err := loadBasicAuthSecret(remote.BasicAuth, SecretsInPromNS)
			if err != nil {
				return nil, fmt.Errorf("could not generate basicAuth for remote_write config %d. %s", i, err)
			}
			secrets[fmt.Sprintf("remoteWrite/%d", i)] = credentials
		}
	}

	// load apiserver basic auth secret
	if apiserverConfig != nil && apiserverConfig.BasicAuth != nil {
		credentials, err := loadBasicAuthSecret(apiserverConfig.BasicAuth, SecretsInPromNS)
		if err != nil {
			return nil, fmt.Errorf("could not generate basicAuth for apiserver config. %s", err)
		}
		secrets["apiserver"] = credentials
	}

	return secrets, nil

}

func loadBasicAuthSecretFromAPI(basicAuth *monitoringv1.BasicAuth, c client.Client, ns string, cache map[string]*corev1.Secret) (assets.BasicAuthCredentials, error) {
	var username string
	var password string
	var err error

	if username, err = getCredFromSecret(c, basicAuth.Username, "username", ns, ns+"/"+basicAuth.Username.Name, cache); err != nil {
		return assets.BasicAuthCredentials{}, err
	}

	if password, err = getCredFromSecret(c, basicAuth.Password, "password", ns, ns+"/"+basicAuth.Password.Name, cache); err != nil {
		return assets.BasicAuthCredentials{}, err
	}

	return assets.BasicAuthCredentials{Username: username, Password: password}, nil
}

func loadBasicAuthSecret(basicAuth *monitoringv1.BasicAuth, s *corev1.SecretList) (assets.BasicAuthCredentials, error) {
	var username string
	var password string
	var err error

	for _, secret := range s.Items {

		if secret.Name == basicAuth.Username.Name {
			if username, err = extractCredKey(&secret, basicAuth.Username, "username"); err != nil {
				return assets.BasicAuthCredentials{}, err
			}
		}

		if secret.Name == basicAuth.Password.Name {
			if password, err = extractCredKey(&secret, basicAuth.Password, "password"); err != nil {
				return assets.BasicAuthCredentials{}, err
			}

		}
		if username != "" && password != "" {
			break
		}
	}

	if username == "" && password == "" {
		return assets.BasicAuthCredentials{}, fmt.Errorf("basic auth username and password secret not found")
	}

	return assets.BasicAuthCredentials{Username: username, Password: password}, nil
}

func extractCredKey(secret *corev1.Secret, sel corev1.SecretKeySelector, cred string) (string, error) {
	if s, ok := secret.Data[sel.Key]; ok {
		return string(s), nil
	}
	return "", fmt.Errorf("secret %s key %q in secret %q not found", cred, sel.Key, sel.Name)
}

func getCredFromSecret(c client.Client, sel corev1.SecretKeySelector, namespace string, cred string, cacheKey string, cache map[string]*corev1.Secret) (_ string, err error) {
	var ok bool
	var s *corev1.Secret

	if s, ok = cache[cacheKey]; !ok {
		s = &corev1.Secret{}
		if err = c.Get(context.TODO(), types.NamespacedName{Name: sel.Name, Namespace: namespace}, s); err != nil {
			return "", fmt.Errorf("unable to fetch %s secret %s/%s: %s", cred, namespace, sel.Name, err)
		}
		cache[cacheKey] = s
	}
	return extractCredKey(s, sel, cred)
}

func LoadBearerTokensFromSecrets(client client.Client, mons map[string]*monitoringv1.ServiceMonitor) (map[string]assets.BearerToken, error) {
	tokens := map[string]assets.BearerToken{}
	nsSecretCache := make(map[string]*corev1.Secret)

	for _, mon := range mons {
		for i, ep := range mon.Spec.Endpoints {
			if ep.BearerTokenSecret.Name == "" {
				continue
			}

			token, err := getCredFromSecret(
				client,
				ep.BearerTokenSecret,
				mon.GetNamespace(),
				"bearertoken",
				mon.Namespace+"/"+ep.BearerTokenSecret.Name,
				nsSecretCache,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to extract endpoint bearertoken for servicemonitor %v from secret %v in namespace %v. Error: %v",
					mon.Name, ep.BearerTokenSecret.Name, mon.Namespace, err,
				)
			}

			tokens[fmt.Sprintf("serviceMonitor/%s/%s/%d", mon.Namespace, mon.Name, i)] = assets.BearerToken(token)
		}
	}

	return tokens, nil
}
