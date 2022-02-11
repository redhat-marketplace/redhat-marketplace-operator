// Copyright 2021 IBM Corp.
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

package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	emperrors "emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/marketplace"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// blank assignment to verify that ReconcileClusterRegistration implements reconcile.Reconciler
var _ reconcile.Reconciler = &ClusterRegistrationReconciler{}

var (
	TokenFieldMissingOrEmpty error = errors.New("token field not found on secret")
)

// ClusterRegistrationReconciler reconciles a Registration object
type ClusterRegistrationReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger

	cfg *config.OperatorConfig
}

type SecretInfo struct {
	TypeOf     string
	Secret     *v1.Secret
	StatusKey  string
	MessageKey string
	SecretKey  string
	MissingMsg string
}

// +kubebuilder:rbac:groups="",resources=secret,verbs=get;list;watch
// +kubebuilder:rbac:groups="",namespace=system,resources=secret,verbs=create
// +kubebuilder:rbac:groups="",namespace=system,resources=secret,resourceNames=redhat-marketplace-pull-secret,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=marketplaceconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=marketplaceconfigs,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="config.openshift.io",resources=clusterversions,verbs=get;list;watch

// Reconcile reads that state of the cluster for a ClusterRegistration object and makes changes based on the state read
// and what is in the ClusterRegistration.Spec
func (r *ClusterRegistrationReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ClusterRegistration")

	si, err := ReturnSecret(r.Client, request, reqLogger)
	if err != nil {
		return reconcile.Result{}, err
	}

	annotations := si.Secret.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	jwtToken, err := ParseAndValidate(si)
	if err != nil {
		reqLogger.Error(err, "error validating secret")
		if errors.Is(err, TokenFieldMissingOrEmpty) {
			return r.updateSecretWithMessage(si, annotations, reqLogger)
		}

		return reconcile.Result{}, err
	}

	if jwtToken == "" {
		err := errors.New("jwt token is empty")
		reqLogger.Error(err, "couldn't find secret field")
		return reconcile.Result{}, err
	}

	tokenClaims, err := marketplace.GetJWTTokenClaim(jwtToken)
	if err != nil {
		reqLogger.Error(err, "Token is missing account id")
		annotations[si.StatusKey] = "error"
		annotations[si.MessageKey] = "Account id is not available in provided token, please generate token from RH Marketplace again"
		si.Secret.SetAnnotations(annotations)
		if err := r.Client.Update(context.TODO(), si.Secret); err != nil {
			reqLogger.Error(err, "Failed to patch secret with Endpoint status")
		}
		reqLogger.Info("Secret updated with account id missing error")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Marketplace Token Claims set")

	newMarketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Namespace: request.Namespace,
		Name:      "marketplaceconfig",
	}, newMarketplaceConfig)

	if err != nil && k8serrors.IsNotFound(err) {
		newMarketplaceConfig = nil
	}

	if newMarketplaceConfig != nil {
		err = r.addOwnerRefToAll(r.Client, newMarketplaceConfig, request, reqLogger)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	//only check registration status, compare pull secret from COS if we are not in a disconnected environment
	if newMarketplaceConfig != nil && !r.cfg.IsDisconnected {
		mclient, err := marketplace.NewMarketplaceClientBuilder(r.cfg).
			NewMarketplaceClient(jwtToken, tokenClaims)

		if err != nil {
			reqLogger.Error(err, "failed to build marketplaceclient")
			return reconcile.Result{}, nil
		}

		reqLogger.Info("MarketPlace config object found, check status if its installed or not")
		//Setting MarketplaceClientAccount

		marketplaceClientAccount := &marketplace.MarketplaceClientAccount{
			AccountId:   newMarketplaceConfig.Spec.RhmAccountID,
			ClusterUuid: newMarketplaceConfig.Spec.ClusterUUID,
		}

		// Marketplace config object found
		reqLogger.Info("Pulling MarketPlace config object status")
		registrationStatusOutput, _ := mclient.RegistrationStatus(marketplaceClientAccount)

		if registrationStatusOutput.RegistrationStatus == marketplace.RegistrationStatusInstalled {
			reqLogger.Info("MarketPlace config object is already registered for account")

			//Update secret with status
			if annotations[si.SecretKey] != "success" {
				reqLogger.Info("Updating secret with success status")
				annotations[si.StatusKey] = "success"
				annotations[si.MessageKey] = "rhm-operator-secret generated successfully"
				si.Secret.SetAnnotations(annotations)
				if err := r.Client.Update(context.TODO(), si.Secret); err != nil {
					reqLogger.Error(err, "Failed to patch secret with Endpoint status")
					return reconcile.Result{}, err
				}
				reqLogger.Info("Secret updated with status on success")
			}
		}

		reqLogger.Info("token found", "from secret", si.TypeOf)
		//Calling POST endpoint to pull the secret definition
		newOptSecretObj, err := mclient.GetMarketplaceSecret()
		if err != nil {
			reqLogger.Error(err, "failed to get marketplace secret")
			annotations[si.StatusKey] = "error"
			annotations[si.MessageKey] = err.Error()
			si.Secret.SetAnnotations(annotations)
			if err := r.Client.Update(context.TODO(), si.Secret); err != nil {
				reqLogger.Error(err, "Failed to patch secret with Endpoint status")
			}
			return reconcile.Result{}, err
		}
		newOptSecretObj.SetNamespace(request.Namespace)

		//Fetch the Secret with name redhat-Operator-secret
		secretKeyname := types.NamespacedName{
			Name:      newOptSecretObj.Name,
			Namespace: newOptSecretObj.Namespace,
		}

		reqLogger.Info("retrieving secret", "name", secretKeyname.Name, "namespace", secretKeyname.Namespace)

		optSecret := &v1.Secret{}
		err = r.Client.Get(context.TODO(), secretKeyname, optSecret)
		if err != nil {
			if !k8serrors.IsNotFound(err) {
				reqLogger.Error(err, "bad error getting secret")
				return reconcile.Result{}, err
			}

			reqLogger.Info("secret not found, creating")
			err = r.Client.Create(context.TODO(), newOptSecretObj)
			if err != nil {
				reqLogger.Error(err, "Failed to Create Secret Object")
				return reconcile.Result{}, err
			}
		} else {
			reqLogger.Info("Comparing old and new rhm-operator-secret")

			if !reflect.DeepEqual(newOptSecretObj.Data, optSecret.Data) {
				reqLogger.Info("rhm-operator-secret are different copy")
				optSecret.Data = newOptSecretObj.Data

				err := r.Client.Update(context.TODO(), optSecret)
				if err != nil {
					reqLogger.Error(err, "could not update rhm-operator-secret with new object", "Resource", utils.RHMOperatorSecretName)
					return reconcile.Result{}, err
				}
			}
		}
	}

	//Update secret with status
	if annotations[si.SecretKey] != "success" {
		reqLogger.Info("Updating secret with success status")
		annotations[si.StatusKey] = "success"
		annotations[si.MessageKey] = "rhm-operator-secret generated successfully"
		si.Secret.SetAnnotations(annotations)
		if err := r.Client.Update(context.TODO(), si.Secret); err != nil {
			reqLogger.Error(err, "Failed to patch secret with Endpoint status")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Secret updated with status on success")
	}

	//Create Markeplace Config object
	reqLogger.Info("finding clusterversion resource")
	clusterVersion := &openshiftconfigv1.ClusterVersion{}
	err = r.Client.Get(context.Background(), client.ObjectKey{
		Name: "version",
	}, clusterVersion)

	if err != nil {
		if !k8serrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			reqLogger.Error(err, "Failed to retrieve clusterversion resource")
			return reconcile.Result{}, err
		}
		clusterVersion = nil
	}

	var clusterID string
	if clusterVersion != nil {
		clusterID = string(clusterVersion.Spec.ClusterID)
		reqLogger.Info("Clusterversion object found with clusterID", "clusterID", clusterID)
	} else {
		clusterID = uuid.New().String()
		reqLogger.Info("Clusterversion object not found, generating clusterID", "clusterID", clusterID)
	}

	newMarketplaceConfig = &marketplacev1alpha1.MarketplaceConfig{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Namespace: request.Namespace,
		Name:      "marketplaceconfig",
	}, newMarketplaceConfig)

	annotations = map[string]string{
		"marketplace.redhat.com/environment": tokenClaims.Env,
	}

	if err != nil {
		if k8serrors.IsNotFound(err) {
			newMarketplaceConfig.ObjectMeta.Name = "marketplaceconfig"
			newMarketplaceConfig.ObjectMeta.Namespace = request.Namespace
			newMarketplaceConfig.Spec.ClusterUUID = string(clusterID)
			newMarketplaceConfig.Spec.RhmAccountID = tokenClaims.AccountID
			newMarketplaceConfig.Annotations = annotations

			// Create Marketplace Config object with ClusterID
			reqLogger.Info("Marketplace Config creating")
			err = r.Client.Create(context.TODO(), newMarketplaceConfig)
			if err != nil {
				reqLogger.Error(err, "Failed to Create Marketplace Config Object")
				return reconcile.Result{}, err
			}

			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.Error(err, "failed to get marketplaceconfig")
		return reconcile.Result{}, err
	}

	owners := newMarketplaceConfig.GetOwnerReferences()

	if newMarketplaceConfig.Spec.ClusterUUID != string(clusterID) ||
		newMarketplaceConfig.Spec.RhmAccountID != tokenClaims.AccountID ||
		!reflect.DeepEqual(newMarketplaceConfig.GetOwnerReferences(), owners) ||
		!reflect.DeepEqual(newMarketplaceConfig.Annotations, annotations) {
		newMarketplaceConfig.Spec.ClusterUUID = string(clusterID)
		newMarketplaceConfig.Spec.RhmAccountID = tokenClaims.AccountID
		newMarketplaceConfig.Annotations = annotations

		err = r.Client.Update(context.TODO(), newMarketplaceConfig)
		if err != nil {
			reqLogger.Error(err, "Failed to update Marketplace Config Object")
			return reconcile.Result{}, err
		}
	}

	reqLogger.Info("reconcile finished. Marketplace Config Created")
	return reconcile.Result{}, nil
}

func (r *ClusterRegistrationReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (m *ClusterRegistrationReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	m.cfg = cfg
	return nil
}

func ReturnSecret(client client.Client, request reconcile.Request, reqLogger logr.Logger) (*SecretInfo, error) {
	pullSecret, pullSecretErr := GetPullSecret(client, request)
	if pullSecretErr != nil && k8serrors.IsNotFound(pullSecretErr) {
		reqLogger.Info("could not find secret", "secret", utils.RHMPullSecretName)
	}
	if pullSecret != nil {
		reqLogger.Info("found secret", "secret type", utils.RHMPullSecretName)
		return &SecretInfo{
			TypeOf:     utils.RHMPullSecretName,
			Secret:     pullSecret,
			StatusKey:  utils.RHMPullSecretStatus,
			MessageKey: utils.RHMPullSecretMessage,
			SecretKey:  utils.RHMPullSecretKey,
			MissingMsg: utils.RHMPUllSecretMissing,
		}, nil
	}

	entitlementKeySecret, entitlementKeySecretErr := GetEntitlementKey(client, request)
	if entitlementKeySecretErr != nil && k8serrors.IsNotFound(entitlementKeySecretErr) {
		reqLogger.Info("could not find secret", "secret", utils.IBMEntitlementKeySecretName)
	}
	if pullSecret == nil && entitlementKeySecret != nil {
		reqLogger.Info("found secret", "secret type", utils.IBMEntitlementKeySecretName)
		return &SecretInfo{
			TypeOf:     utils.IBMEntitlementKeySecretName,
			Secret:     entitlementKeySecret,
			StatusKey:  utils.IBMEntitlementKeyStatus,
			MessageKey: utils.IBMEntitlementKeyMessage,
			SecretKey:  utils.IBMEntitlementDataKey,
			MissingMsg: utils.IBMEntitlementKeyPasswordMissing,
		}, nil
	}

	if entitlementKeySecretErr != nil && pullSecretErr != nil {
		return nil, emperrors.New(fmt.Sprintf("could not find %s or %s", utils.RHMPullSecretName, utils.IBMEntitlementKeySecretName))
	}

	return nil, nil
}

func GetEntitlementKey(client client.Client, request reconcile.Request) (*v1.Secret, error) {
	ibmEntitlementKeySecret := &v1.Secret{}
	err := client.Get(context.TODO(),
		types.NamespacedName{Name: utils.IBMEntitlementKeySecretName, Namespace: request.Namespace},
		ibmEntitlementKeySecret)
	if err != nil {
		return nil, err
	}

	return ibmEntitlementKeySecret, nil
}

func GetPullSecret(client client.Client, request reconcile.Request) (*v1.Secret, error) {
	rhmPullSecret := &v1.Secret{}
	err := client.Get(context.TODO(),
		types.NamespacedName{Name: utils.RHMPullSecretName, Namespace: request.Namespace},
		rhmPullSecret)
	if err != nil {
		return nil, err
	}

	return rhmPullSecret, nil
}

func ParseAndValidate(si *SecretInfo) (string, error) {
	jwtToken := ""
	if si.TypeOf == utils.IBMEntitlementKeySecretName {
		ek := &marketplace.EntitlementKey{}
		err := json.Unmarshal([]byte(si.Secret.Data[si.SecretKey]), ek)
		if err != nil {
			return "", err
		}

		prodAuth, ok := ek.Auths[utils.IBMEntitlementProdKey]
		if ok {
			if prodAuth.Password == "" {
				return "", fmt.Errorf("could not find jwt token on prod entitlement key %w", TokenFieldMissingOrEmpty)
			}
			jwtToken = prodAuth.Password
		}

		stageAuth, ok := ek.Auths[utils.IBMEntitlementStageKey]
		if ok {
			if stageAuth.Password == "" {
				return "", fmt.Errorf("could not find jwt token on stage entitlement key %w", TokenFieldMissingOrEmpty)
			}
			jwtToken = stageAuth.Password
		}

	} else if si.TypeOf == utils.RHMPullSecretName {
		if _, ok := si.Secret.Data[si.SecretKey]; !ok {
			return "", fmt.Errorf("could not find jwt token on redhat-marketplace-pull-secret %w", TokenFieldMissingOrEmpty)
		}
		jwtToken = string(si.Secret.Data[si.SecretKey])
	}

	return jwtToken, nil
}

func (r *ClusterRegistrationReconciler) updateSecretWithMessage(si *SecretInfo, annotations map[string]string, reqLogger logr.Logger) (reconcile.Result, error) {
	reqLogger.Info("Missing token field in secret")
	annotations[si.StatusKey] = "error"
	annotations[si.MessageKey] = si.MissingMsg
	si.Secret.SetAnnotations(annotations)
	err := r.Client.Update(context.TODO(), si.Secret)
	if err != nil {
		reqLogger.Error(err, "Failed to patch secret with Endpoint status")
	}
	reqLogger.Info("Secret updated with status on failiure")
	return reconcile.Result{}, err
}

// will set the owner ref on both the redhat-marketplace-pull-secret and the ibm-entitlement-key so that both get cleaned up if we delete marketplace config
// TODO: @dan using the client and scheme on the ClusterRegistrationReconciler struct and passing in the client, let me know you want to only using params
func (m *ClusterRegistrationReconciler) addOwnerRefToAll(client client.Client, marketplaceConfig *marketplacev1alpha1.MarketplaceConfig, request reconcile.Request, reqLogger logr.Logger) error {
	pullSecret, _ := GetPullSecret(client, request)
	if pullSecret != nil {
		err := m.addOwnerRef(marketplaceConfig, pullSecret, reqLogger)
		if err != nil {
			return err
		}
	}

	entitlementKeySecret, _ := GetEntitlementKey(client, request)
	if entitlementKeySecret != nil {
		err := m.addOwnerRef(marketplaceConfig, entitlementKeySecret, reqLogger)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *ClusterRegistrationReconciler) addOwnerRef(marketplaceConfig *marketplacev1alpha1.MarketplaceConfig, secret *v1.Secret, reqLogger logr.Logger) error {
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
		m.Scheme); !ownerFound && err == nil {
		m.Client.Update(context.TODO(), secret)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ClusterRegistrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	namespacePredicate := predicates.NamespacePredicate(r.cfg.DeployedNamespace)
	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(namespacePredicate).
		For(&v1.Secret{}, builder.WithPredicates(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					secret, ok := e.Object.(*v1.Secret)
					if !ok {
						return false
					}
					secretName := secret.ObjectMeta.Name
					if _, ok := secret.Data[utils.RHMPullSecretKey]; ok && secretName == utils.RHMPullSecretName {
						return true
					}

					if _, ok := secret.Data[utils.IBMEntitlementDataKey]; ok && secretName == utils.IBMEntitlementKeySecretName {
						return true
					}
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					secret, ok := e.ObjectNew.(*v1.Secret)
					secretName := secret.ObjectMeta.Name
					if !ok {
						return false
					}

					if secretName == utils.RHMPullSecretName {
						if _, ok := secret.Data[utils.RHMPullSecretKey]; ok && e.ObjectOld != e.ObjectNew {
							return true
						}
					}

					if secretName == utils.IBMEntitlementKeySecretName {
						if _, ok := secret.Data[utils.IBMEntitlementDataKey]; ok && e.ObjectOld != e.ObjectNew {
							return true
						}
					}

					return false
				},
				DeleteFunc: func(event.DeleteEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					secret, ok := e.Object.(*v1.Secret)
					if !ok {
						return false
					}
					secretName := secret.ObjectMeta.Name
					if _, ok := secret.Data[utils.RHMPullSecretKey]; ok && secretName == utils.RHMPullSecretName {
						return true
					}

					if _, ok := secret.Data[utils.IBMEntitlementDataKey]; ok && secretName == utils.IBMEntitlementKeySecretName {
						return true
					}
					return false
				},
			},
		)).
		Complete(r)
}
