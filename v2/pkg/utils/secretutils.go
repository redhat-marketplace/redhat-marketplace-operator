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

package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/goph/emperror"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	TokenFieldMissingOrEmpty error = errors.New("token field not found on secret")
	NoSecretsFound           error = errors.New("Could not find redhat-marketplace-pull-secret or ibm-entitlement-key")
)

type ReporterSecretFetcherBuilder struct {
	Ctx               context.Context
	K8sClient         client.Client
	DeployedNamespace string
}

type ControllerSecretFetcherBuilder struct {
	RHMPullSecretSI   *SecretInfo
	EntitlementKeySI  *SecretInfo
	Ctx               context.Context
	K8sClient         client.Client
	DeployedNamespace string
}

type SecretInfo struct {
	Name       string
	Secret     *v1.Secret
	StatusKey  string
	MessageKey string
	SecretKey  string
	MissingMsg string
}

type EntitlementKey struct {
	Auths map[string]Auth `json:"auths"`
}

type Auth struct {
	UserName string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
}

func ProvideSecretFetcherBuilderForReporter(client client.Client, ctx context.Context, deployedNamespace string) *ReporterSecretFetcherBuilder {
	return &ReporterSecretFetcherBuilder{
		Ctx:               ctx,
		K8sClient:         client,
		DeployedNamespace: deployedNamespace,
	}
}

func (rsf *ReporterSecretFetcherBuilder) ReturnSecret() (*SecretInfo, error) {
	pullSecret, pullSecretErr := rsf.GetPullSecret()
	if pullSecret != nil {
		return &SecretInfo{
			Name:       RHMPullSecretName,
			Secret:     pullSecret,
			StatusKey:  RHMPullSecretStatus,
			MessageKey: RHMPullSecretMessage,
			SecretKey:  RHMPullSecretKey,
			MissingMsg: RHMPullSecretMissing,
		}, nil
	}

	entitlementKeySecret, entitlementKeySecretErr := rsf.GetEntitlementKey()
	if pullSecret == nil && entitlementKeySecret != nil {
		return &SecretInfo{
			Name:       IBMEntitlementKeySecretName,
			Secret:     entitlementKeySecret,
			StatusKey:  IBMEntitlementKeyStatus,
			MessageKey: IBMEntitlementKeyMessage,
			SecretKey:  IBMEntitlementDataKey,
			MissingMsg: IBMEntitlementKeyPasswordMissing,
		}, nil
	}

	if entitlementKeySecretErr != nil && pullSecretErr != nil {
		return nil, NoSecretsFound
	}

	return nil, nil
}

func (rsf *ReporterSecretFetcherBuilder) GetEntitlementKey() (*v1.Secret, error) {
	ibmEntitlementKeySecret := &v1.Secret{}
	err := rsf.K8sClient.Get(context.TODO(),
		types.NamespacedName{Name: IBMEntitlementKeySecretName, Namespace: rsf.DeployedNamespace},
		ibmEntitlementKeySecret)
	if err != nil {
		return nil, err
	}

	return ibmEntitlementKeySecret, nil
}

func (rsf *ReporterSecretFetcherBuilder) GetPullSecret() (*v1.Secret, error) {
	rhmPullSecret := &v1.Secret{}
	err := rsf.K8sClient.Get(context.TODO(),
		types.NamespacedName{Name: RHMPullSecretName, Namespace: rsf.DeployedNamespace},
		rhmPullSecret)
	if err != nil {
		return nil, err
	}

	return rhmPullSecret, nil
}

func (rsf *ReporterSecretFetcherBuilder) ParseAndValidate(si *SecretInfo) (string, error) {
	jwtToken := ""
	if si.Name == IBMEntitlementKeySecretName {
		ek := &EntitlementKey{}
		err := json.Unmarshal([]byte(si.Secret.Data[si.SecretKey]), ek)
		if err != nil {
			return "", emperror.Wrap(err, "error unmarshalling entitlement key")
		}

		prodAuth, ok := ek.Auths[IBMEntitlementProdKey]
		if ok {
			if prodAuth.Password == "" {
				return "", fmt.Errorf("could not find jwt token on prod entitlement key %w", TokenFieldMissingOrEmpty)
			}
			jwtToken = prodAuth.Password
		}

		stageAuth, ok := ek.Auths[IBMEntitlementStageKey]
		if ok {
			if stageAuth.Password == "" {
				return "", fmt.Errorf("could not find jwt token on stage entitlement key %w", TokenFieldMissingOrEmpty)
			}
			jwtToken = stageAuth.Password
		}

	} else if si.Name == RHMPullSecretName {
		if _, ok := si.Secret.Data[si.SecretKey]; !ok {
			return "", fmt.Errorf("could not find jwt token on redhat-marketplace-pull-secret %w", TokenFieldMissingOrEmpty)
		}
		jwtToken = string(si.Secret.Data[si.SecretKey])
	}

	return jwtToken, nil
}

func ReturnSecret(client client.Client, request reconcile.Request) (*SecretInfo, error) {
	pullSecret, pullSecretErr := GetPullSecret(client, request)
	if pullSecret != nil {
		return &SecretInfo{
			Name:       RHMPullSecretName,
			Secret:     pullSecret,
			StatusKey:  RHMPullSecretStatus,
			MessageKey: RHMPullSecretMessage,
			SecretKey:  RHMPullSecretKey,
			MissingMsg: RHMPullSecretMissing,
		}, nil
	}

	entitlementKeySecret, entitlementKeySecretErr := GetEntitlementKey(client, request)
	if pullSecret == nil && entitlementKeySecret != nil {
		return &SecretInfo{
			Name:       IBMEntitlementKeySecretName,
			Secret:     entitlementKeySecret,
			StatusKey:  IBMEntitlementKeyStatus,
			MessageKey: IBMEntitlementKeyMessage,
			SecretKey:  IBMEntitlementDataKey,
			MissingMsg: IBMEntitlementKeyPasswordMissing,
		}, nil
	}

	if entitlementKeySecretErr != nil && pullSecretErr != nil {
		return nil, NoSecretsFound
	}

	return nil, nil
}

func GetEntitlementKey(client client.Client, request reconcile.Request) (*v1.Secret, error) {
	ibmEntitlementKeySecret := &v1.Secret{}
	err := client.Get(context.TODO(),
		types.NamespacedName{Name: IBMEntitlementKeySecretName, Namespace: request.Namespace},
		ibmEntitlementKeySecret)
	if err != nil {
		return nil, err
	}

	return ibmEntitlementKeySecret, nil
}

func GetPullSecret(client client.Client, request reconcile.Request) (*v1.Secret, error) {
	rhmPullSecret := &v1.Secret{}
	err := client.Get(context.TODO(),
		types.NamespacedName{Name: RHMPullSecretName, Namespace: request.Namespace},
		rhmPullSecret)
	if err != nil {
		return nil, err
	}

	return rhmPullSecret, nil
}

func ParseAndValidate(si *SecretInfo) (string, error) {
	jwtToken := ""
	if si.Name == IBMEntitlementKeySecretName {
		ek := &EntitlementKey{}
		err := json.Unmarshal([]byte(si.Secret.Data[si.SecretKey]), ek)
		if err != nil {
			return "", err
		}

		prodAuth, ok := ek.Auths[IBMEntitlementProdKey]
		if ok {
			if prodAuth.Password == "" {
				return "", fmt.Errorf("could not find jwt token on prod entitlement key %w", TokenFieldMissingOrEmpty)
			}
			jwtToken = prodAuth.Password
		}

		stageAuth, ok := ek.Auths[IBMEntitlementStageKey]
		if ok {
			if stageAuth.Password == "" {
				return "", fmt.Errorf("could not find jwt token on stage entitlement key %w", TokenFieldMissingOrEmpty)
			}
			jwtToken = stageAuth.Password
		}

	} else if si.Name == RHMPullSecretName {
		if _, ok := si.Secret.Data[si.SecretKey]; !ok {
			return "", fmt.Errorf("could not find jwt token on redhat-marketplace-pull-secret %w", TokenFieldMissingOrEmpty)
		}
		jwtToken = string(si.Secret.Data[si.SecretKey])
	}

	return jwtToken, nil
}
