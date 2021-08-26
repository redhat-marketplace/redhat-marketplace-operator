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
	"sync"

	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type ServiceAccountClient struct {
	KubernetesInterface kubernetes.Interface
	Token               *Token
	TokenRequestObj     *authv1.TokenRequest
	Client              typedv1.ServiceAccountInterface
	sync.Mutex
}

type Token struct {
	AuthToken           *string
	ExpirationTimestamp metav1.Time
}

func (s *ServiceAccountClient) NewServiceAccountToken(targetServiceAccountName string, audience string, expireSecs int64, reqLogger logr.Logger) (string, error) {
	s.Lock()
	defer s.Unlock()

	now := metav1.Now().UTC()
	opts := metav1.CreateOptions{}
	tr := s.newTokenRequest(audience, expireSecs)

	if s.Token == nil {
		reqLogger.Info("auth token from service account found")

		return s.getToken(targetServiceAccountName, s.Client, tr, opts)
	}

	if now.UTC().After(s.Token.ExpirationTimestamp.Time) {

		reqLogger.Info("service account token is expired")

		return s.getToken(targetServiceAccountName, s.Client, tr, opts)
	}

	return s.getToken(targetServiceAccountName, s.Client, tr, opts)
}

func NewServiceAccountClient(namespace string, kubernetesInterface kubernetes.Interface) *ServiceAccountClient {
	if kubernetesInterface == nil {
		fmt.Println("KUBERNETES INTERFACE IS NIL")
	}
	return &ServiceAccountClient{
		Client: kubernetesInterface.CoreV1().ServiceAccounts(namespace),
	}
}

func (s *ServiceAccountClient) newTokenRequest(audience string, expireSeconds int64) *authv1.TokenRequest {
	if len(audience) != 0 {
		return &authv1.TokenRequest{
			Spec: authv1.TokenRequestSpec{
				Audiences:         []string{audience},
				ExpirationSeconds: ptr.Int64(expireSeconds),
			},
		}
	} else {
		return &authv1.TokenRequest{
			Spec: authv1.TokenRequestSpec{
				ExpirationSeconds: ptr.Int64(expireSeconds),
			},
		}
	}
}

func (s *ServiceAccountClient) getToken(targetServiceAccount string, client typedv1.ServiceAccountInterface, tr *authv1.TokenRequest, opts metav1.CreateOptions) (string, error) {
	tr, err := client.CreateToken(context.TODO(), targetServiceAccount, tr, opts)
	if err != nil {
		return "", err
	}

	s.Token = &Token{
		AuthToken:           ptr.String(tr.Status.Token),
		ExpirationTimestamp: tr.Status.ExpirationTimestamp,
	}

	token := tr.Status.Token
	return token, nil
}
