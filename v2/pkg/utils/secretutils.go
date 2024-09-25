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
	"strings"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/goph/emperror"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Parser            *jwt.Parser
}

type SecretInfo struct {
	Type   string
	Secret *v1.Secret
	Env    string
	Token  string
	Claims *MarketplaceClaims
}

type EntitlementKey struct {
	Auths map[string]Auth `json:"auths"`
}

type Auth struct {
	UserName string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
}

type MarketplaceClaims struct {
	AccountID string `json:"rhmAccountId"`
	Password  string `json:"password,omitempty"`
	APIKey    string `json:"iam_apikey,omitempty"`
	Env       string `json:"env,omitempty"`
	jwt.RegisteredClaims
}

func ProvideSecretFetcherBuilder(client client.Client, ctx context.Context, deployedNamespace string) *SecretFetcherBuilder {
	return &SecretFetcherBuilder{
		Ctx:               ctx,
		K8sClient:         client,
		DeployedNamespace: deployedNamespace,
		Parser:            new(jwt.Parser),
	}
}

// Returns Secret with priority on redhat-marketplace-pull-secret
// Parse and populate SecretInfo
func (sf *SecretFetcherBuilder) ReturnSecret() (*SecretInfo, error) {

	pullSecret, pullSecretErr := sf.GetPullSecret()
	if pullSecretErr == nil {
		return sf.Parse(pullSecret)
	} else if !k8serrors.IsNotFound(pullSecretErr) {
		return nil, pullSecretErr
	}

	entitlementKeySecret, entitlementKeySecretErr := sf.GetEntitlementKey()
	if entitlementKeySecretErr == nil {
		return sf.Parse(entitlementKeySecret)
	} else if !k8serrors.IsNotFound(entitlementKeySecretErr) {
		return nil, entitlementKeySecretErr
	}

	return nil, NoSecretsFound
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

// Delete Secrets, ignore IsNotFound
func (sf *SecretFetcherBuilder) DeleteSecret() error {
	if err := sf.K8sClient.Delete(
		sf.Ctx,
		&v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: RHMPullSecretName, Namespace: sf.DeployedNamespace}},
	); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if err := sf.K8sClient.Delete(
		sf.Ctx,
		&v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: IBMEntitlementKeySecretName, Namespace: sf.DeployedNamespace}},
	); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	return nil
}

// Parse v1.Secret and return SecretInfo
// Will handle case if ibm-entitlement-key is written into redhat-marketplace-pull-secret
func (sf *SecretFetcherBuilder) Parse(secret *v1.Secret) (*SecretInfo, error) {
	si := &SecretInfo{Secret: secret}

	if key, ok := si.Secret.Data[IBMEntitlementDataKey]; ok { // ibm-entitlement-key stored as .dockerconfigjson with jwt auths
		ek := &EntitlementKey{}
		err := json.Unmarshal(key, ek)
		if err != nil {
			return si, emperror.WrapWith(err, "error unmarshalling", "secret", si.Secret.GetName(), "key", IBMEntitlementDataKey)
		}

		stageAuth, ok := ek.Auths[IBMEntitlementStageKey]
		if ok {
			if stageAuth.Password == "" {

				return si, emperror.WrapWith(TokenFieldMissingOrEmpty, "secret", si.Secret.GetName(), "key", IBMEntitlementDataKey, "key", IBMEntitlementStageKey, "key", "password")
			}
			si.Token = stageAuth.Password
			si.Env = StageEnv
		}

		prodAuth, ok := ek.Auths[IBMEntitlementProdKey]
		if ok {
			if prodAuth.Password == "" {
				return si, emperror.WrapWith(TokenFieldMissingOrEmpty, "secret", si.Secret.GetName(), "key", IBMEntitlementDataKey, "key", IBMEntitlementProdKey, "key", "password")
			}
			si.Token = prodAuth.Password
			si.Env = ProdEnv
		}

		si.Type = IBMEntitlementKeySecretName

	} else if key, ok := si.Secret.Data[RHMPullSecretKey]; ok { // jwt string set at key PULL_SECRET
		if err := sf.validateDecode(string(key)); err != nil {
			return si, emperror.WrapWith(err, "unable to decode jwt", "secret", si.Secret.GetName(), "key", RHMPullSecretKey)
		}

		si.Token = string(key)

		token, _, err := sf.Parser.ParseUnverified(si.Token, &MarketplaceClaims{})
		if err != nil {
			return si, err
		}

		si.Claims, ok = token.Claims.(*MarketplaceClaims)
		if !ok {
			return si, emperror.WrapWith(errors.New("invalid claims"), "claims is not type *MarketplaceClaims", "secret", si.Secret.GetName(), "key", RHMPullSecretKey)
		}

		if si.Claims.AccountID == "" { // this was probably an ibm-entitlement-key incorrectly set as a redhat-marketplace-pull-secret
			si.Type = IBMEntitlementKeySecretName
			si.Env = ProdEnv // assumed
		} else { // redhat-marketplace-pull-secret
			if strings.ToLower(si.Claims.Env) == StageEnv {
				si.Env = StageEnv
			} else {
				si.Env = ProdEnv
			}
			si.Type = RHMPullSecretName
		}

	} else {
		return si, emperror.WrapWith(errors.New("invalid secret"), "no jwt found", "secret", si.Secret.GetName())
	}

	return si, nil
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

// Check if all JWT parts can be decoded, and none have been malformed by a typo
func (sf *SecretFetcherBuilder) validateDecode(token string) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return emperror.Wrap(jwt.ErrTokenMalformed, "token contains an invalid number of segments")
	}
	if _, err := sf.Parser.DecodeSegment(parts[0]); err != nil {
		return emperror.Wrap(err, "could not base64 decode header")
	}
	if _, err := sf.Parser.DecodeSegment(parts[1]); err != nil {
		return emperror.Wrap(err, "could not base64 decode claim")
	}
	if _, err := sf.Parser.DecodeSegment(parts[2]); err != nil {
		return emperror.Wrap(err, "could not base64 decode signature")
	}
	return nil
}
