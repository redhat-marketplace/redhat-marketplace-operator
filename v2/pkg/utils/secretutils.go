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
	"strings"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/goph/emperror"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	TokenFieldMissingOrEmpty error = errors.New("token field not found on secret")
	NoSecretsFound           error = errors.New("Could not find redhat-marketplace-pull-secret or ibm-entitlement-key")
)

type SecretFetcherBuilder struct {
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
	Env        string
}

type EntitlementKey struct {
	Auths map[string]Auth `json:"auths"`
}

type Auth struct {
	UserName string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
}

func ProvideSecretFetcherBuilder(client client.Client, ctx context.Context, deployedNamespace string) *SecretFetcherBuilder {
	return &SecretFetcherBuilder{
		Ctx:               ctx,
		K8sClient:         client,
		DeployedNamespace: deployedNamespace,
	}
}

func (sf *SecretFetcherBuilder) ReturnSecret() (*SecretInfo, error) {
	pullSecret, pullSecretErr := sf.GetPullSecret()
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

	entitlementKeySecret, entitlementKeySecretErr := sf.GetEntitlementKey()
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

func (sf *SecretFetcherBuilder) GetEntitlementKey() (*v1.Secret, error) {
	ibmEntitlementKeySecret := &v1.Secret{}
	err := sf.K8sClient.Get(sf.Ctx,
		types.NamespacedName{Name: IBMEntitlementKeySecretName, Namespace: sf.DeployedNamespace},
		ibmEntitlementKeySecret)
	if err != nil {
		return nil, err
	}

	return ibmEntitlementKeySecret, nil
}

func (sf *SecretFetcherBuilder) GetPullSecret() (*v1.Secret, error) {
	rhmPullSecret := &v1.Secret{}
	err := sf.K8sClient.Get(sf.Ctx,
		types.NamespacedName{Name: RHMPullSecretName, Namespace: sf.DeployedNamespace},
		rhmPullSecret)
	if err != nil {
		return nil, err
	}

	return rhmPullSecret, nil
}

func (sf *SecretFetcherBuilder) ParseAndValidate(si *SecretInfo) (string, error) {
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
			si.Env = ProdEnv
		}

		stageAuth, ok := ek.Auths[IBMEntitlementStageKey]
		if ok {
			if stageAuth.Password == "" {
				return "", fmt.Errorf("could not find jwt token on stage entitlement key %w", TokenFieldMissingOrEmpty)
			}
			jwtToken = stageAuth.Password
			si.Env = StageEnv
		}

	} else if si.Name == RHMPullSecretName {
		if _, ok := si.Secret.Data[si.SecretKey]; !ok {
			return "", fmt.Errorf("could not find jwt token on redhat-marketplace-pull-secret %w", TokenFieldMissingOrEmpty)
		}
		jwtToken = string(si.Secret.Data[si.SecretKey])
	}

	// Validate jwt token segments and decode
	parts := strings.Split(jwtToken, ".")
	if len(parts) != 3 {
		return jwtToken, emperror.Wrap(jwt.ErrTokenMalformed, "token contains an invalid number of segments")
	}

	parser := new(jwt.Parser)

	if _, err := parser.DecodeSegment(parts[0]); err != nil {
		return jwtToken, emperror.Wrap(err, "could not base64 decode header")
	}

	if _, err := parser.DecodeSegment(parts[1]); err != nil {
		return jwtToken, emperror.Wrap(err, "could not base64 decode claim")
	}

	if _, err := parser.DecodeSegment(parts[2]); err != nil {
		return jwtToken, emperror.Wrap(err, "could not base64 decode signature")
	}

	return jwtToken, nil
}

// will set the owner ref on both the redhat-marketplace-pull-secret and the ibm-entitlement-key so that both get cleaned up if we delete marketplace config
func (sf *SecretFetcherBuilder) AddOwnerRefToAll(marketplaceConfig *marketplacev1alpha1.MarketplaceConfig, scheme *runtime.Scheme) error {
	pullSecret, _ := sf.GetPullSecret()
	if pullSecret != nil {
		err := sf.addOwnerRef(marketplaceConfig, pullSecret, scheme)
		if err != nil {
			return err
		}
	}

	entitlementKeySecret, _ := sf.GetEntitlementKey()
	if entitlementKeySecret != nil {
		err := sf.addOwnerRef(marketplaceConfig, entitlementKeySecret, scheme)
		if err != nil {
			return err
		}
	}

	return nil
}

func (sf *SecretFetcherBuilder) addOwnerRef(marketplaceConfig *marketplacev1alpha1.MarketplaceConfig, secret *v1.Secret, scheme *runtime.Scheme) error {
	ownerFound := false
	for _, owner := range secret.ObjectMeta.OwnerReferences {
		if owner.Name == secret.Name &&
			owner.Kind == secret.Kind &&
			owner.APIVersion == secret.APIVersion {
			ownerFound = true
		}
	}

	if err := controllerutil.SetOwnerReference(
		marketplaceConfig,
		secret,
		scheme); !ownerFound && err == nil {
		sf.K8sClient.Update(sf.Ctx, secret)
		if err != nil {
			return err
		}
	}

	return nil
}
