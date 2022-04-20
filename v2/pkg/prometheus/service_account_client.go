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
	"sync"

	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type ServiceAccountClient struct {
	tokenRequestObj *authv1.TokenRequest
	client          typedv1.ServiceAccountInterface
	log             logr.Logger

	tokens map[string]*Token

	sync.Mutex
}

type Token struct {
	AuthToken           *string
	ExpirationTimestamp metav1.Time
}

func NewServiceAccountClient(
	namespace string,
	kubernetesInterface kubernetes.Interface,
	log logr.Logger,
) *ServiceAccountClient {
	return &ServiceAccountClient{
		client: kubernetesInterface.CoreV1().ServiceAccounts(namespace),
		log:    log.WithName("ServiceAccountClient"),
	}
}

func (s *ServiceAccountClient) GetToken(targetServiceAccountName string, audience string, expireSecs int64) (string, error) {
	s.Lock()
	defer s.Unlock()

	now := metav1.Now().UTC()

	token, ok := s.tokens[s.cacheKey(targetServiceAccountName, audience)]

	if !ok || (ok && now.UTC().After(token.ExpirationTimestamp.Time)) {
		s.log.Info("auth token from service account found")

		return s.getToken(targetServiceAccountName, audience, expireSecs)
	}

	return *token.AuthToken, nil
}

func (s *ServiceAccountClient) cacheKey(targetServiceAccount, audience string) string {
	return targetServiceAccount + ":" + audience
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

func (s *ServiceAccountClient) getToken(targetServiceAccount, audience string, expireSecs int64) (string, error) {
	opts := metav1.CreateOptions{}
	tr := s.newTokenRequest(audience, expireSecs)

	tr, err := s.client.CreateToken(context.TODO(), targetServiceAccount, tr, opts)
	if err != nil {
		return "", err
	}

	token := &Token{
		AuthToken:           ptr.String(tr.Status.Token),
		ExpirationTimestamp: tr.Status.ExpirationTimestamp,
	}

	s.tokens[s.cacheKey(targetServiceAccount, audience)] = token
	return *token.AuthToken, nil
}
