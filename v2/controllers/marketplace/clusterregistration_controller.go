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
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/gotidy/ptr"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/marketplace"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// blank assignment to verify that ReconcileClusterRegistration implements reconcile.Reconciler
var _ reconcile.Reconciler = &ClusterRegistrationReconciler{}

// ClusterRegistrationReconciler reconciles a Registration object
type ClusterRegistrationReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger

	Cfg *config.OperatorConfig
}

type SecretInfo struct {
	TypeOf     string
	Secret     *v1.Secret
	StatusKey  string
	MessageKey string
	SecretKey  string
	MissingMsg string
}

// +kubebuilder:rbac:groups="",namespace=system,resources=secrets,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",namespace=system,resources=secrets;secrets/finalizers,resourceNames=redhat-marketplace-pull-secret;ibm-entitlement-key;rhm-operator-secret,verbs=update;patch
// +kubebuilder:rbac:groups="apps",namespace=system,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",namespace=system,resources=deployments/finalizers,verbs=get;list;watch;update;patch,resourceNames=ibm-metrics-operator-controller-manager
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=marketplaceconfigs;marketplaceconfigs/finalizers;marketplaceconfigs/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="config.openshift.io",resources=clusterversions,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",namespace=system,resources=deployments;deployments/finalizers,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",namespace=system,resources=deployments/finalizers,verbs=get;list;watch;update;patch,resourceNames=ibm-metrics-operator-controller-manager

// Reconcile fetches the cluster registration state with MarketplaceClient
// Updates MarketplaceConfig with account and cluster registration status
// Related reconcilers should read the state of MarketplaceConfig, and avoid using MarketplaceClient
func (r *ClusterRegistrationReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ClusterRegistration")

	secretFetcher := utils.ProvideSecretFetcherBuilder(r.Client, context.TODO(), request.Namespace)
	si, err := secretFetcher.ReturnSecret()
	if err == utils.NoSecretsFound {
		// marketplaceconfig controller will provide status in this case
		reqLogger.Info("no redhat-marketplace-pull-secret or ibm-entitlement-key secret found, secret is required in a connected environment")
		// if there is no secret, do not finalize MaketplaceConfig
		return reconcile.Result{}, r.removeMarketplaceConfigFinalizer(request)
	} else if err != nil {
		return reconcile.Result{}, err
	}

	reqLogger.Info("found secret", "secret", si.Secret.GetName())

	// Warn about incorrect secret, though this case is handled
	if si.Type == utils.IBMEntitlementKeySecretName && si.Secret.GetName() == utils.RHMPullSecretName {
		reqLogger.Info(fmt.Sprintf("warning: %v jwt written into secret/%v data.%v. %v should be writen into secret/%v as %v.",
			utils.IBMEntitlementKeySecretName,
			utils.RHMPullSecretName,
			utils.RHMPullSecretKey,
			utils.IBMEntitlementKeySecretName,
			utils.IBMEntitlementKeySecretName,
			utils.IBMEntitlementDataKey))
	}

	mclient, err := marketplace.NewMarketplaceClientBuilder(r.Cfg).NewMarketplaceClient(si.Token, si.Claims)
	if err != nil {
		reqLogger.Error(err, "failed to build marketplaceclient")
		return reconcile.Result{}, nil
	}

	marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Namespace: request.Namespace,
		Name:      utils.MARKETPLACECONFIG_NAME,
	}, marketplaceConfig)

	if err == nil && marketplaceConfig.GetDeletionTimestamp() != nil {
		// On marketplaceconfig Deletion, attempt unregister, and remove finalizers
		// Garbage Collection should delete remaining owned resources
		if !ptr.ToBool(marketplaceConfig.Spec.IsDisconnected) {
			// Attempt to unregister, do not block reconciler
			// underlying marketplaceClient is not retryablehttp, so attempt retries
			err := retry.OnError(retry.DefaultBackoff, func(_ error) bool {
				return true
			}, func() error {
				marketplaceClientAccount := &marketplace.MarketplaceClientAccount{
					AccountId:   marketplaceConfig.Spec.RhmAccountID,
					ClusterUuid: marketplaceConfig.Spec.ClusterUUID,
				}
				reqLogger.Info("unregister", "marketplace client account", marketplaceClientAccount)
				registrationStatusOutput, err := mclient.UnRegister(marketplaceClientAccount)
				if err != nil {
					reqLogger.Error(err, "unregister failed")
					return err
				}
				reqLogger.Info("unregister", "RegistrationStatus", registrationStatusOutput.RegistrationStatus)
				return err
			})
			if err != nil {
				reqLogger.Error(err, "error requesting unregistration")
			}
			reqLogger.Info("cluster unregistered")
		}

		if err := r.removeMarketplaceConfigFinalizer(request); err != nil {
			return reconcile.Result{}, err
		}

		reqLogger.Info("marketplaceconfig delete is complete.")
		return reconcile.Result{}, nil

	} else if k8serrors.IsNotFound(err) && si.Secret.GetDeletionTimestamp() == nil {
		// Create a default MarketplaceConfig if a pull-secret is created, connected environment quickstart
		marketplaceConfig.Name = utils.MARKETPLACECONFIG_NAME
		marketplaceConfig.Namespace = r.Cfg.DeployedNamespace

		// IS_DISCONNECTED flag takes precedence, default IsDisconnected to false
		if r.Cfg.IsDisconnected {
			marketplaceConfig.Spec.IsDisconnected = ptr.Bool(true)
		} else if marketplaceConfig.Spec.IsDisconnected == nil {
			marketplaceConfig.Spec.IsDisconnected = ptr.Bool(false)
		}

		// Set required ClusterID
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

		marketplaceConfig.Spec.ClusterUUID = string(clusterID)

		// Set the controller deployment as the controller-ref, since it owns the finalizer
		dep := &appsv1.Deployment{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{
			Name:      utils.RHM_METERING_DEPLOYMENT_NAME,
			Namespace: r.Cfg.DeployedNamespace,
		}, dep)
		if err != nil {
			return reconcile.Result{}, err
		}
		if err = controllerutil.SetControllerReference(dep, marketplaceConfig, r.Scheme); err != nil {
			return reconcile.Result{}, err
		}

		if err = r.Client.Create(context.TODO(), marketplaceConfig); err != nil {
			return reconcile.Result{}, err
		}
	} else if si.Secret.GetDeletionTimestamp() != nil {
		// return if in deletion
		return reconcile.Result{}, err
	}

	// Set MarketplaceConfig as the Owner of the pull secret
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		secretFetcher.GetPullSecret()
		return secretFetcher.AddOwnerRefToAll(marketplaceConfig, r.Scheme)
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	// check if license is accepted before registering cluster
	if !ptr.ToBool(marketplaceConfig.Spec.IsDisconnected) && !ptr.ToBool(marketplaceConfig.Spec.License.Accept) {
		reqLogger.Info("License has not been accepted in marketplaceconfig. You have to accept license to continue")
		return reconcile.Result{}, nil
	}

	var newOptSecretObj *v1.Secret
	var rhmAccountExists *bool // nil=unknown, true, false
	var registrationStatusConditions status.Conditions
	if !ptr.ToBool(marketplaceConfig.Spec.IsDisconnected) {
		//only check registration status, compare pull secret from COS if we are not in a disconnected environment

		if err != nil {
			reqLogger.Error(err, "failed to build marketplaceclient")
			return reconcile.Result{}, nil
		}

		if si.Type == utils.IBMEntitlementKeySecretName { // IEK may or may not have RHM/SWC account, and account may be created at any time
			cond := marketplaceConfig.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionRHMAccountExists)
			if cond == nil || cond.IsFalse() { // Check if we don't already know the condition, or is false
				exists, err := mclient.RhmAccountExists() // false on error
				if err != nil {                           // API request error
					reqLogger.Error(err, "failed to check if rhm account exists")
				} else {
					rhmAccountExists = ptr.Bool(exists)
				}
			} else if cond != nil && cond.IsTrue() { // already found account exists
				rhmAccountExists = ptr.Bool(true)
			}
		} else if si.Type == utils.RHMPullSecretName {
			rhmAccountExists = ptr.Bool(true)
		}

		// only check registration and get the secret if RHM account exists
		if ptr.ToBool(rhmAccountExists) {
			//
			// Secret
			//
			//Calling POST endpoint to pull the secret definition
			newOptSecretObj, err = mclient.GetMarketplaceSecret()
			if err != nil {
				reqLogger.Error(err, "failed to get operator secret")
			} else {
				// Secret comes back with namespace "redhat-marketplace-operator", set to request namespace
				newOptSecretObj.Namespace = request.Namespace

				//Fetch the Secret with name rhm-operator-secret
				secretKeyname := types.NamespacedName{
					Name:      newOptSecretObj.Name,
					Namespace: newOptSecretObj.Namespace,
				}

				reqLogger.Info("retrieving operator secret", "name", secretKeyname.Name, "namespace", secretKeyname.Namespace)

				err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
					optSecret := &v1.Secret{}
					err = r.Client.Get(context.TODO(), secretKeyname, optSecret)
					if err != nil {
						if !k8serrors.IsNotFound(err) {
							reqLogger.Error(err, "error getting operator secret")
							return err
						}

						controllerutil.SetOwnerReference(si.Secret, newOptSecretObj, r.Scheme)
						reqLogger.Info("secret not found, creating")
						err = r.Client.Create(context.TODO(), newOptSecretObj)
						if err != nil {
							reqLogger.Error(err, "Failed to create operator object")
							return err
						}
					} else {
						reqLogger.Info("Comparing old and new rhm-operator-secret")

						if !reflect.DeepEqual(newOptSecretObj.Data, optSecret.Data) {
							reqLogger.Info("rhm-operator-secret are different copy")
							optSecret.Data = newOptSecretObj.Data
							controllerutil.SetOwnerReference(si.Secret, newOptSecretObj, r.Scheme)
							reqLogger.Info("updating rhm-operator-secret")
							return r.Client.Update(context.TODO(), optSecret)
						}
					}

					return nil
				})
				if err != nil {
					return reconcile.Result{}, err
				}
			}

			//
			// Registration
			//

			registrationStatusOutput, err := mclient.RegistrationStatus(&marketplace.MarketplaceClientAccount{
				AccountId:   marketplaceConfig.Spec.RhmAccountID,
				ClusterUuid: marketplaceConfig.Spec.ClusterUUID,
			})
			if err != nil {
				reqLogger.Error(err, "registration status failed")
			} else {
				registrationStatusConditions = registrationStatusOutput.TransformConfigStatus()
			}
		}
	}

	// Update the AccountID & annotations field in MarketplaceConfig
	// Update the Registration Status
	reqLogger.Info("updating marketplaceconfig")
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err = r.Client.Get(context.TODO(), types.NamespacedName{
			Namespace: request.Namespace,
			Name:      utils.MARKETPLACECONFIG_NAME,
		}, marketplaceConfig); err != nil {
			return err
		}

		updated := false
		// Add Finalizer
		updated = updated || controllerutil.AddFinalizer(marketplaceConfig, utils.CONTROLLER_FINALIZER)

		if si.Type == utils.IBMEntitlementKeySecretName && newOptSecretObj != nil {
			if marketplaceConfig.Spec.RhmAccountID != string(newOptSecretObj.Data[utils.BUCKET_NAME_FIELD]) {
				marketplaceConfig.Spec.RhmAccountID = string(newOptSecretObj.Data[utils.BUCKET_NAME_FIELD])
				updated = true
			}
		}
		if si.Type == utils.RHMPullSecretName {
			if marketplaceConfig.Spec.RhmAccountID != si.Claims.AccountID {
				marketplaceConfig.Spec.RhmAccountID = si.Claims.AccountID
				updated = true
			}
		}

		// If the annotation is not set, or (unlikely) does not match tokenClaims.Env
		env, ok := marketplaceConfig.GetAnnotations()["marketplace.redhat.com/environment"]
		if ok {
			if env != si.Env {
				ok = false
			}
		}
		if !ok {
			marketplaceConfig.SetAnnotations(map[string]string{"marketplace.redhat.com/environment": si.Env})
			updated = true
		}

		if updated {
			return r.Client.Update(context.TODO(), marketplaceConfig)
		}

		return nil
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	// Update the Registration Status
	reqLogger.Info("updating marketplaceconfig status")
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err = r.Client.Get(context.TODO(), types.NamespacedName{
			Namespace: request.Namespace,
			Name:      utils.MARKETPLACECONFIG_NAME,
		}, marketplaceConfig); err != nil {
			return err
		}

		updated := false

		if rhmAccountExists != nil {
			if ptr.ToBool(rhmAccountExists) {
				updated = marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
					Type:    marketplacev1alpha1.ConditionRHMAccountExists,
					Status:  v1.ConditionTrue,
					Reason:  marketplacev1alpha1.ReasonRHMAccountExists,
					Message: "RHM/Software Central account exists",
				}) || updated
			} else {
				updated = marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
					Type:    marketplacev1alpha1.ConditionRHMAccountExists,
					Status:  v1.ConditionFalse,
					Reason:  marketplacev1alpha1.ReasonRHMAccountNotExist,
					Message: "RHM/Software Central account does not exist",
				}) || updated
			}
		}

		for _, cond := range registrationStatusConditions {
			updated = updated || marketplaceConfig.Status.Conditions.SetCondition(cond)
		}

		if updated {
			return r.Client.Status().Update(context.TODO(), marketplaceConfig)
		}

		return nil
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	reqLogger.Info("ClusterRegistrationController reconcile finished")
	// Requeue to resolve RHMAccountExists for ibm-entitlement-key users who later register for RHM
	return reconcile.Result{RequeueAfter: time.Hour * 1}, nil
}

func (r *ClusterRegistrationReconciler) removeMarketplaceConfigFinalizer(request reconcile.Request) error {
	marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), types.NamespacedName{
			Namespace: request.Namespace,
			Name:      utils.MARKETPLACECONFIG_NAME,
		}, marketplaceConfig); k8serrors.IsNotFound(err) {
			return nil
		} else if err != nil {
			return err
		}
		if controllerutil.RemoveFinalizer(marketplaceConfig, utils.CONTROLLER_FINALIZER) {
			return r.Client.Update(context.TODO(), marketplaceConfig)
		}
		return nil
	})
}

func (r *ClusterRegistrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Secret{}, builder.WithPredicates(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					switch e.Object.GetName() {
					case utils.RHMPullSecretName:
						return true
					case utils.IBMEntitlementKeySecretName:
						return true
					default:
						return false
					}
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					switch e.ObjectNew.GetName() {
					case utils.RHMPullSecretName:
						return true
					case utils.IBMEntitlementKeySecretName:
						return true
					case utils.RHM_OPERATOR_SECRET_NAME:
						return true
					default:
						return false
					}
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					return e.Object.GetName() == utils.RHM_OPERATOR_SECRET_NAME
				},
				GenericFunc: func(e event.GenericEvent) bool {
					switch e.Object.GetName() {
					case utils.RHMPullSecretName:
						return true
					case utils.IBMEntitlementKeySecretName:
						return true
					case utils.RHM_OPERATOR_SECRET_NAME:
						return true
					default:
						return false
					}
				},
			},
		)).
		// This controller is responsible for keeping the AccountID & Env annotation updated on MarketplaceConfig (connected)
		// Since the values are dervived from the pull secret
		// It also must reconcile due a change from IsDisconnected to connected mode
		Watches(
			&marketplacev1alpha1.MarketplaceConfig{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
				return []reconcile.Request{
					{NamespacedName: types.NamespacedName{
						Name:      utils.RHMPullSecretName,
						Namespace: a.GetNamespace(),
					}},
				}
			}),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool { return true },
				UpdateFunc: func(e event.UpdateEvent) bool {

					marketplaceConfigNew, newOk := e.ObjectNew.(*marketplacev1alpha1.MarketplaceConfig)
					marketplaceConfigOld, oldOk := e.ObjectOld.(*marketplacev1alpha1.MarketplaceConfig)

					if newOk && oldOk {
						// Change in disconnected mode
						if !ptr.ToBool(marketplaceConfigNew.Spec.IsDisconnected) && ptr.ToBool(marketplaceConfigOld.Spec.IsDisconnected) {
							return true
						}
						// Change in license accept
						if ptr.ToBool(marketplaceConfigNew.Spec.License.Accept) && !ptr.ToBool(marketplaceConfigOld.Spec.License.Accept) {
							return true
						}
						// Change in ClusterID
						if marketplaceConfigNew.Spec.ClusterUUID != marketplaceConfigOld.Spec.ClusterUUID {
							return true
						}
						// Change in AccountID
						if marketplaceConfigNew.Spec.RhmAccountID != marketplaceConfigOld.Spec.RhmAccountID {
							return true
						}
						// Change in annotations
						if !reflect.DeepEqual(marketplaceConfigNew.GetAnnotations(), marketplaceConfigOld.GetAnnotations()) {
							return true
						}
					}

					// Deletion / Finalizer
					if marketplaceConfigNew.GetDeletionTimestamp() != nil {
						return true
					}

					return false
				},
				DeleteFunc:  func(event.DeleteEvent) bool { return false },
				GenericFunc: func(e event.GenericEvent) bool { return false },
			},
			)).
		Complete(r)
}
