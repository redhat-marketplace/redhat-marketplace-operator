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

package marketplace

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/gotidy/ptr"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	DEFAULT_IMAGE_MARKETPLACE_AGENT = "marketplace-agent:latest"
	IBM_CATALOG_SOURCE_FLAG         = true
)

// blank assignment to verify that ReconcileMarketplaceConfig implements reconcile.Reconciler
var _ reconcile.Reconciler = &MarketplaceConfigReconciler{}

// MarketplaceConfigReconciler reconciles a MarketplaceConfig object
type MarketplaceConfigReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	Cfg    *config.OperatorConfig
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",namespace=system,resources=namespaces,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",namespace=system,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",namespace=system,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",namespace=system,resources=deployments/finalizers,verbs=get;list;watch;update;patch,resourceNames=redhat-marketplace-controller-manager
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=marketplaceconfigs;marketplaceconfigs/finalizers;marketplaceconfigs/status,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=razeedeployments,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=razeedeployments,verbs=update;patch;delete,resourceNames=rhm-marketplaceconfig-razeedeployment
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=meterbases,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=meterbases,verbs=update;patch;delete,resourceNames=rhm-marketplaceconfig-meterbase

// CatalogSource
// +kubebuilder:rbac:groups="operators.coreos.com",resources=catalogsources,verbs=create;get;list;watch
// +kubebuilder:rbac:groups="operators.coreos.com",resources=catalogsources,verbs=delete,resourceNames=ibm-operator-catalog;opencloud-operators

// Infrastructure Discovery
// +kubebuilder:rbac:groups="",namespace=system,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",namespace=system,resources=replicasets,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",namespace=system,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups="operators.coreos.com",namespace=system,resources=clusterserviceversions,verbs=get;list;watch
// +kubebuilder:rbac:groups="operators.coreos.com",namespace=system,resources=subscriptions,verbs=get;list;watch

// Reconcile reads that state of the cluster for a MarketplaceConfig object and makes changes based on the state read
// and what is in the MarketplaceConfig.Spec
func (r *MarketplaceConfigReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MarketplaceConfig")

	// Fetch the MarketplaceConfig instance
	marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
	if err := r.Client.Get(context.TODO(), request.NamespacedName, marketplaceConfig); err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("Resource not found. Ignoring since object must be deleted")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get MarketplaceConfig")
		return reconcile.Result{}, err
	}

	// check if license is accepted
	if !ptr.ToBool(marketplaceConfig.Spec.License.Accept) {
		if marketplaceConfig.Status.Conditions.GetCondition(status.ConditionType(marketplacev1alpha1.ConditionComplete)) != nil {
			// upgrade scenario from previous version without license acceptance section, update it as accepted
			reqLogger.Info("updating marketplaceconfig, setting license acceptance")
			if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				if err := r.Client.Get(context.TODO(), request.NamespacedName, marketplaceConfig); err != nil {
					return err
				}
				marketplaceConfig.Spec.License.Accept = ptr.Bool(true)
				return r.Client.Update(context.TODO(), marketplaceConfig)
			}); err != nil {
				return reconcile.Result{}, err
			}
		} else {
			err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				// license not accepted, update status
				if err := r.Client.Get(context.TODO(), request.NamespacedName, marketplaceConfig); err != nil {
					return err
				}
				if marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
					Type:    marketplacev1alpha1.ConditionNoLicense,
					Status:  corev1.ConditionTrue,
					Reason:  marketplacev1alpha1.ReasonLicenseNotAccepted,
					Message: "License has not been accepted in marketplaceconfig",
				}) {
					reqLogger.Info("updating marketplaceconfig status")
					return r.Client.Status().Update(context.TODO(), marketplaceConfig)
				}
				return nil
			})
			reqLogger.Info("License has not been accepted in marketplaceconfig. You have to accept license to continue with initialization")
			return reconcile.Result{}, err
		}
	} else {
		// License Accepted, clear status
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if marketplaceConfig.Status.Conditions.RemoveCondition(status.ConditionType(marketplacev1alpha1.ConditionNoLicense)) {
				reqLogger.Info("updating marketplaceconfig status")
				return r.Client.Status().Update(context.TODO(), marketplaceConfig)
			}
			return nil
		}); err != nil {
			return reconcile.Result{}, err
		}
	}

	secretFetcher := utils.ProvideSecretFetcherBuilder(r.Client, context.TODO(), request.Namespace)

	// This could ideally be in a namespace reconciler
	if result, err := r.updateDeployedNamespaceLabels(marketplaceConfig); err != nil {
		return result, err
	}

	// Ensure the Spec is complete, set derived values
	if result, err := r.initializeMarketplaceConfigSpec(request, secretFetcher); err != nil {
		return result, err
	}

	// Install is starting, set Status
	if result, err := r.updateMarketplaceConfigStatusStarting(request); err != nil {
		return result, err
	}

	// Create or update MeterBase, set derived values
	if result, err := r.createOrUpdateMeterBase(request); err != nil {
		return result, err
	}

	// handle meterbase settings for meter definition catalog server
	if result, err := r.handleMeterDefinitionCatalogServerConfigs(request); err != nil {
		return result, err
	}

	// Create or update RazeeDeployment, set derived values
	if result, err := r.createOrUpdateRazeeRazeeDeployment(request); err != nil {
		return result, err
	}

	// Create CatalogSource

	for _, catalogSrcName := range [2]string{utils.IBM_CATALOGSRC_NAME, utils.OPENCLOUD_CATALOGSRC_NAME} {
		if result, err := r.createCatalogSource(marketplaceConfig, catalogSrcName); err != nil {
			return result, err
		}
	}

	// Install is complete, set Status
	if result, err := r.updateMarketplaceConfigStatusFinished(request); err != nil {
		return result, err
	}

	reqLogger.Info("reconciling finished")
	return reconcile.Result{}, nil
}

func (r *MarketplaceConfigReconciler) handleMeterDefinitionCatalogServerConfigs(
	request reconcile.Request,
) (reconcile.Result, error) {

	reqLogger := r.Log.WithValues("func", "handleMeterDefinitionCatalogServerConfigs", "Request.Namespace", request.Namespace, "Request.Name", request.Name)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		meterBase := &marketplacev1alpha1.MeterBase{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.METERBASE_NAME, Namespace: request.Namespace}, meterBase); err != nil {
			reqLogger.Error(err, "failed to get meterbase")
			return err
		}

		marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
			reqLogger.Error(err, "failed to get marketplaceconfig")
			return err
		}

		meterBaseCopy := meterBase.DeepCopy()

		if ptr.ToBool(marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer) {
			// if meterbase doesn't have MeterdefinitionCatalogServerConfig on MeterBase.Spec.
			// Set the struct and set all flags to true
			if meterBase.Spec.MeterdefinitionCatalogServerConfig == nil {
				reqLogger.Info("enabling MeterDefinitionCatalogServerConfig values")
				meterBase.Spec.MeterdefinitionCatalogServerConfig = &common.MeterDefinitionCatalogServerConfig{
					SyncCommunityMeterDefinitions:      true,
					SyncSystemMeterDefinitions:         false, //always making this false for now
					DeployMeterDefinitionCatalogServer: !ptr.ToBool(marketplaceConfig.Spec.IsDisconnected) && ptr.ToBool(marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer),
				}
			} else {
				// foundMeterBase.Spec.MeterdefinitionCatalogServerConfig already exists
				// just allow for toggling the deployment of the file server - leave individual sync flags alone
				if !meterBase.Spec.MeterdefinitionCatalogServerConfig.DeployMeterDefinitionCatalogServer {
					reqLogger.Info("enabling MeterDefinitionCatalogServerConfig values")
					meterBase.Spec.MeterdefinitionCatalogServerConfig = &common.MeterDefinitionCatalogServerConfig{
						SyncCommunityMeterDefinitions:      meterBase.Spec.MeterdefinitionCatalogServerConfig.SyncCommunityMeterDefinitions,
						SyncSystemMeterDefinitions:         meterBase.Spec.MeterdefinitionCatalogServerConfig.SyncSystemMeterDefinitions,
						DeployMeterDefinitionCatalogServer: !ptr.ToBool(marketplaceConfig.Spec.IsDisconnected) && ptr.ToBool(marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer),
					}
				}
			}
		}

		// meterdef catalog server disabled, set all flags to false. This will remove file server resources and all community & system meterdefs
		if !ptr.ToBool(marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer) && meterBase.Spec.MeterdefinitionCatalogServerConfig != nil {
			reqLogger.Info("disabling MeterDefinitionCatalogServerConfig values")

			meterBase.Spec.MeterdefinitionCatalogServerConfig = &common.MeterDefinitionCatalogServerConfig{
				//TODO: probably not necessary but setting to false here just to be safe
				SyncCommunityMeterDefinitions:      false,
				SyncSystemMeterDefinitions:         false,
				DeployMeterDefinitionCatalogServer: false,
			}
		}

		if !reflect.DeepEqual(meterBaseCopy.Spec, meterBase.Spec) {
			reqLogger.Info("updating meterbase")
			return r.Client.Update(context.TODO(), meterBase)
		}

		return nil
	})

	return reconcile.Result{}, err
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *MarketplaceConfigReconciler) SetupWithManager(mgr manager.Manager) error {
	// Create a new controller
	ownerHandler := handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &marketplacev1alpha1.MarketplaceConfig{}, handler.OnlyControllerOwner())

	namespacePredicate := predicates.NamespacePredicate(r.Cfg.DeployedNamespace)

	// ClusterRegistrationReconciler handles MarketplaceConfig deletion
	return ctrl.NewControllerManagedBy(mgr).
		For(&marketplacev1alpha1.MarketplaceConfig{},
			builder.WithPredicates(predicate.Funcs{
				CreateFunc:  func(e event.CreateEvent) bool { return true },
				UpdateFunc:  func(e event.UpdateEvent) bool { return true },
				DeleteFunc:  func(event.DeleteEvent) bool { return false },
				GenericFunc: func(e event.GenericEvent) bool { return false },
			})).
		WithEventFilter(namespacePredicate).
		Watches(&marketplacev1alpha1.RazeeDeployment{}, ownerHandler, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&marketplacev1alpha1.MeterBase{}, ownerHandler).
		Watches(&marketplacev1alpha1.RazeeDeployment{}, ownerHandler, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&marketplacev1alpha1.MeterBase{}, ownerHandler).
		Complete(r)
}

func (r *MarketplaceConfigReconciler) updateDeployedNamespaceLabels(marketplaceConfig *marketplacev1alpha1.MarketplaceConfig) (reconcile.Result, error) {
	// Add License Server tag to deployed namespace
	reqLogger := r.Log.WithValues("func", "updateDeployedNamespaceLabels", "Request.Namespace", marketplaceConfig.Namespace, "Request.Name", marketplaceConfig.Name)
	deployedNamespace := &corev1.Namespace{}

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: r.Cfg.DeployedNamespace}, deployedNamespace); err != nil {
			return err
		}

		if deployedNamespace.Labels == nil {
			deployedNamespace.Labels = make(map[string]string)
		}

		if v, ok := deployedNamespace.Labels[utils.LicenseServerTag]; !ok || v != "true" {
			deployedNamespace.Labels[utils.LicenseServerTag] = "true"

			reqLogger.Info("updating namespace")
			return r.Client.Update(context.TODO(), deployedNamespace)
		}

		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *MarketplaceConfigReconciler) initializeMarketplaceConfigSpec(
	request reconcile.Request,
	secretFetcher *utils.SecretFetcherBuilder,
) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("func", "initializeMarketplaceConfigSpec", "Request.Namespace", request.Namespace, "Request.Name", request.Name)

	marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
		reqLogger.Error(err, "failed to get marketplaceconfig")
		return reconcile.Result{}, err
	}

	// Initialize MarketplaceConfigSpec
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
			reqLogger.Error(err, "failed to get marketplaceconfig")
			return err
		}

		marketplaceConfigCopy := marketplaceConfig.DeepCopy()

		// IS_DISCONNECTED flag takes precedence, default IsDisconnected to false
		if r.Cfg.IsDisconnected {
			marketplaceConfig.Spec.IsDisconnected = ptr.Bool(true)
		} else if marketplaceConfig.Spec.IsDisconnected == nil {
			marketplaceConfig.Spec.IsDisconnected = ptr.Bool(false)
		}

		if marketplaceConfig.Spec.Features == nil {
			marketplaceConfig.Spec.Features = &common.Features{}
		}

		if marketplaceConfig.Spec.Features.Registration == nil {
			marketplaceConfig.Spec.Features.Registration = ptr.Bool(true)
		}

		// Removed Features
		marketplaceConfig.Spec.Features.Deployment = ptr.Bool(false)
		marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer = ptr.Bool(false)

		// Initialize Catalog flag
		if marketplaceConfig.Spec.InstallIBMCatalogSource == nil {
			marketplaceConfig.Spec.InstallIBMCatalogSource = &r.Cfg.Features.IBMCatalog
		}

		// Removing EnabledMetering field so setting them all to nil
		// this will no longer do anything
		if marketplaceConfig.Spec.EnableMetering != nil {
			marketplaceConfig.Spec.EnableMetering = nil
		}

		// Set Cluster DisplayName
		if !ptr.ToBool(marketplaceConfig.Spec.IsDisconnected) {
			si, err := secretFetcher.ReturnSecret()
			if err == nil { // check for missing secret in during status update
				reqLogger.Info("found secret", "secret", si.Secret.GetName())

				if si.Secret != nil {
					if clusterDisplayName, ok := si.Secret.Data[utils.ClusterDisplayNameKey]; ok {
						count := utf8.RuneCountInString(string(clusterDisplayName))
						clusterName := strings.Trim(string(clusterDisplayName), "\n")

						if marketplaceConfig.Spec.ClusterName != clusterName {
							if count <= 256 {
								marketplaceConfig.Spec.ClusterName = clusterName
								reqLogger.Info("setting ClusterName", "name", clusterName)
							} else {
								err := errors.New("CLUSTER_DISPLAY_NAME exceeds 256 chars")
								reqLogger.Error(err, "name", clusterDisplayName)
							}
						}
					}
				}
			}
		}

		// Set cluster UUID
		clusterVersion := &openshiftconfigv1.ClusterVersion{}
		if err := r.Client.Get(context.Background(), client.ObjectKey{Name: "version"}, clusterVersion); err != nil {
			if !k8serrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
				reqLogger.Error(err, "Failed to retrieve clusterversion resource")
				return err
			}
			if marketplaceConfig.Spec.ClusterUUID == "" {
				marketplaceConfig.Spec.ClusterUUID = uuid.New().String()
			}
		} else {
			marketplaceConfig.Spec.ClusterUUID = string(clusterVersion.Spec.ClusterID)
		}

		// Set the controller deployment as the controller-ref, since it owns the finalizer
		dep := &appsv1.Deployment{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{
			Name:      utils.RHM_METERING_DEPLOYMENT_NAME,
			Namespace: r.Cfg.DeployedNamespace,
		}, dep)
		if err != nil {
			return err
		}

		marketplaceConfig.ObjectMeta.SetOwnerReferences(nil)
		if err = controllerutil.SetControllerReference(dep, marketplaceConfig, r.Scheme); err != nil {
			return err
		}

		// Update MarketplaceConfig
		if !reflect.DeepEqual(*marketplaceConfigCopy, *marketplaceConfig) {
			reqLogger.Info("updating marketplaceconfig")
			return r.Client.Update(context.TODO(), marketplaceConfig)
		}
		return nil
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	// Set MarketplaceConfig Status
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
			reqLogger.Error(err, "failed to get marketplaceconfig")
			return err
		}

		updated := false
		if ptr.ToBool(marketplaceConfig.Spec.IsDisconnected) {
			updated = updated || marketplaceConfig.Status.Conditions.RemoveCondition(marketplacev1alpha1.ConditionSecretError)
			updated = updated || marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionIsDisconnected,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonInternetDisconnected,
				Message: "Detected disconnected environment",
			})
		} else {
			updated = updated || marketplaceConfig.Status.Conditions.RemoveCondition(marketplacev1alpha1.ConditionIsDisconnected)

			_, err := secretFetcher.ReturnSecret()
			if err != nil {
				updated = updated || marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
					Type:    marketplacev1alpha1.ConditionSecretError,
					Status:  corev1.ConditionTrue,
					Reason:  marketplacev1alpha1.ReasonNoSecret,
					Message: "no redhat-marketplace-pull-secret or ibm-entitlement-key secret found, secret is required in a connected environment",
				})

				if updated {
					reqLogger.Info("updating marketplaceconfig status")
					if err := r.Client.Status().Update(context.TODO(), marketplaceConfig); err != nil {
						return err
					}
				}

				return errors.New("no redhat-marketplace-pull-secret or ibm-entitlement-key secret found, secret is required in a connected environment")
			} else {
				updated = updated || marketplaceConfig.Status.Conditions.RemoveCondition(marketplacev1alpha1.ConditionSecretError)
			}
		}

		if updated {
			reqLogger.Info("updating marketplaceconfig status")
			return r.Client.Status().Update(context.TODO(), marketplaceConfig)
		}
		return nil
	})

	return reconcile.Result{}, err
}

func (r *MarketplaceConfigReconciler) createOrUpdateMeterBase(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("func", "createOrUpdateMeterBase", "Request.Namespace", request.Namespace, "Request.Name", request.Name)

	marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
		reqLogger.Error(err, "failed to get marketplaceconfig")
		return reconcile.Result{}, err
	}

	meterBase := &marketplacev1alpha1.MeterBase{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.METERBASE_NAME, Namespace: request.Namespace}, meterBase)
	if k8serrors.IsNotFound(err) {
		meterBase = utils.BuildMeterBaseCr(
			request.Namespace,
			!ptr.ToBool(marketplaceConfig.Spec.IsDisconnected) && ptr.ToBool(marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer),
		)

		if err = controllerutil.SetControllerReference(marketplaceConfig, meterBase, r.Scheme); err != nil {
			reqLogger.Error(err, "Failed to set controller ref")
			return reconcile.Result{}, err
		}

		reqLogger.Info("creating meterbase")
		if err := r.Client.Create(context.TODO(), meterBase); err != nil {
			reqLogger.Error(err, "Failed to create a new MeterBase CR.")
			return reconcile.Result{}, err
		}

		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
			if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
				reqLogger.Error(err, "failed to get marketplaceconfig")
				return err
			}

			updated := marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonMeterBaseInstalled,
				Message: "Meter base installed.",
			})

			if updated {
				reqLogger.Info("updating marketplaceconfig status")
				return r.Client.Status().Update(context.TODO(), marketplaceConfig)
			}

			return nil
		}); err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get meterbase")
		return reconcile.Result{}, err
	}

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		meterBase := &marketplacev1alpha1.MeterBase{}
		err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.METERBASE_NAME, Namespace: request.Namespace}, meterBase)
		if err != nil {
			reqLogger.Error(err, "failed to get meterbase")
			return err
		}

		marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
			reqLogger.Error(err, "failed to get marketplaceconfig")
			return err
		}

		// Update MarketplaceConfig Status
		updated := false

		if marketplaceConfig.Status.MeterBaseSubConditions == nil {
			updated = true
			marketplaceConfig.Status.MeterBaseSubConditions = status.Conditions{}
		}

		if meterBase.Status.Conditions != nil {
			if !utils.ConditionsEqual(
				meterBase.Status.Conditions,
				marketplaceConfig.Status.MeterBaseSubConditions) {
				marketplaceConfig.Status.MeterBaseSubConditions = meterBase.Status.Conditions
				updated = updated || true
			}
		}

		if updated {
			reqLogger.Info("updating marketplaceconfig status")
			return r.Client.Status().Update(context.TODO(), marketplaceConfig)
		}

		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *MarketplaceConfigReconciler) createOrUpdateRazeeRazeeDeployment(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("func", "createOrUpdateRazeeRazeeDeployment", "Request.Namespace", request.Namespace, "Request.Name", request.Name)

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
			reqLogger.Error(err, "failed to get marketplaceconfig")
			return err
		}

		//Check if RazeeDeployment exists, if not create one
		razeeDeployment := &marketplacev1alpha1.RazeeDeployment{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.RAZEE_NAME, Namespace: request.Namespace}, razeeDeployment)
		if err != nil && k8serrors.IsNotFound(err) {
			razeeDeployment := utils.BuildRazeeCr(marketplaceConfig.Namespace, marketplaceConfig.Spec.ClusterUUID,
				marketplaceConfig.Spec.DeploySecretName, marketplaceConfig.Spec.Features, marketplaceConfig.Spec.InstallIBMCatalogSource)

			// Sets the owner for razeeDeployment
			if err = controllerutil.SetControllerReference(marketplaceConfig, razeeDeployment, r.Scheme); err != nil {
				reqLogger.Error(err, "Failed to create a new RazeeDeployment CR.")
				return err
			}

			// include a display name if set
			if marketplaceConfig.Spec.ClusterName != "" {
				reqLogger.Info("setting cluster name override on razee cr")
				razeeDeployment.Spec.ClusterDisplayName = marketplaceConfig.Spec.ClusterName
			}
			// Disable razee in disconnected environment or if RHM/Software Central account does not exist
			cond := marketplaceConfig.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionRHMAccountExists)
			rhmAccountExists := cond != nil && cond.IsTrue()
			razeeDeployment.Spec.Enabled = !ptr.ToBool(marketplaceConfig.Spec.IsDisconnected) && rhmAccountExists

			reqLogger.Info("creating razee cr")
			err = r.Client.Create(context.TODO(), razeeDeployment)

			if err != nil {
				reqLogger.Error(err, "Failed to create a new RazeeDeployment CR.")
				return err
			}

			updated := marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonRazeeInstalled,
				Message: "RazeeDeployment installed.",
			})

			if updated {
				reqLogger.Info("updating marketplaceconfig status")
				return r.Client.Status().Update(context.TODO(), marketplaceConfig)
			}
		} else if err != nil {
			reqLogger.Error(err, "Failed to get RazeeDeployment CR")
			return err
		}

		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	// Keep razeeDeployment spec updated, use retry to avoid conflict with razeedeployment controller: object has been modified
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		razeeDeployment := &marketplacev1alpha1.RazeeDeployment{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.RAZEE_NAME, Namespace: request.Namespace}, razeeDeployment); err != nil {
			reqLogger.Error(err, "failed to get razeedeployment")
			return err
		}

		marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
			reqLogger.Error(err, "failed to get marketplaceconfig")
			return err
		}
		cond := marketplaceConfig.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionRHMAccountExists)
		rhmAccountExists := cond != nil && cond.IsTrue()

		razeeDeploymentCopy := razeeDeployment.DeepCopy()

		// Disable razee in disconnected environment or RHM/Software Central account does not exist
		razeeDeployment.Spec.Enabled = !ptr.ToBool(marketplaceConfig.Spec.IsDisconnected) && rhmAccountExists
		razeeDeployment.Spec.ClusterUUID = marketplaceConfig.Spec.ClusterUUID
		razeeDeployment.Spec.DeploySecretName = marketplaceConfig.Spec.DeploySecretName
		razeeDeployment.Spec.Features = marketplaceConfig.Spec.Features.DeepCopy()
		razeeDeployment.Spec.InstallIBMCatalogSource = marketplaceConfig.Spec.InstallIBMCatalogSource

		if marketplaceConfig.Spec.ClusterName != "" {
			razeeDeployment.Spec.ClusterDisplayName = marketplaceConfig.Spec.ClusterName
		}

		if !reflect.DeepEqual(razeeDeploymentCopy.Spec, razeeDeployment.Spec) {
			reqLogger.Info("updating razeedeployment")
			return r.Client.Update(context.TODO(), razeeDeployment)
		}

		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	// Update MarketplaceConfig with Razee watch label
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
			reqLogger.Error(err, "failed to get marketplaceconfig")
			return err
		}

		if marketplaceConfig.Labels == nil {
			marketplaceConfig.Labels = make(map[string]string)
		}

		if marketplaceConfig.Labels[utils.RazeeWatchResource] != utils.RazeeWatchLevelDetail {
			marketplaceConfig.Labels[utils.RazeeWatchResource] = utils.RazeeWatchLevelDetail
			reqLogger.Info("updating marketplaceconfig")
			return r.Client.Update(context.TODO(), marketplaceConfig)
		}

		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	// Set MarketplaceConfig Conditions
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		razeeDeployment := &marketplacev1alpha1.RazeeDeployment{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.RAZEE_NAME, Namespace: request.Namespace}, razeeDeployment); err != nil {
			reqLogger.Error(err, "failed to get razeedeployment")
			return err
		}

		marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
			reqLogger.Error(err, "failed to get marketplaceconfig")
			return err
		}

		// Update MarketplaceConfig Status
		updated := false

		if marketplaceConfig.Status.RazeeSubConditions == nil {
			updated = true
			marketplaceConfig.Status.RazeeSubConditions = status.Conditions{}
		}

		for _, condition := range razeeDeployment.Status.Conditions {
			updated = updated || marketplaceConfig.Status.RazeeSubConditions.SetCondition(condition)
		}

		if razeeDeployment.Status.Conditions != nil {
			if !utils.ConditionsEqual(
				razeeDeployment.Status.Conditions,
				marketplaceConfig.Status.RazeeSubConditions) {
				marketplaceConfig.Status.RazeeSubConditions = razeeDeployment.Status.Conditions
				updated = updated || true
			}
		}

		// Update MarketplaceConfig
		if updated {
			reqLogger.Info("updating marketplaceconfig status")
			return r.Client.Status().Update(context.TODO(), marketplaceConfig)
		}

		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *MarketplaceConfigReconciler) updateMarketplaceConfigStatusStarting(
	request reconcile.Request,
) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("func", "updateMarketplaceConfigStatusStarting", "Request.Namespace", request.Namespace, "Request.Name", request.Name)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
			reqLogger.Error(err, "failed to get marketplaceconfig")
			return err
		}

		if marketplaceConfig.Status.Conditions.IsUnknownFor(marketplacev1alpha1.ConditionInstalling) {
			ok := marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonStartInstall,
				Message: "Installing starting",
			})

			if ok {
				reqLogger.Info("updating marketplaceconfig status")
				return r.Client.Status().Update(context.TODO(), marketplaceConfig)
			}
		}

		return nil
	})

	return reconcile.Result{}, err
}

func (r *MarketplaceConfigReconciler) updateMarketplaceConfigStatusFinished(
	request reconcile.Request,
) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("func", "updateMarketplaceConfigStatusFinished", "Request.Namespace", request.Namespace, "Request.Name", request.Name)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
			reqLogger.Error(err, "failed to get marketplaceconfig")
			return err
		}

		updated := false

		updated = updated || marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionInstalling,
			Status:  corev1.ConditionFalse,
			Reason:  marketplacev1alpha1.ReasonInstallFinished,
			Message: "Finished Installing necessary components",
		})

		updated = updated || marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionComplete,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonInstallFinished,
			Message: "Finished Installing necessary components",
		})

		if updated {
			//Updating Marketplace Config with Cluster Registration status
			reqLogger.Info("updating marketplaceconfig status")
			return r.Client.Status().Update(context.TODO(), marketplaceConfig)
		}

		return nil
	})

	return reconcile.Result{}, err
}

// Begin installation or deletion of Catalog Source
func (r *MarketplaceConfigReconciler) createCatalogSource(instance *marketplacev1alpha1.MarketplaceConfig, catalogName string) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("func", "createCatalogSource", "Request.Namespace", instance.Namespace, "Request.Name", instance.Name)

	return reconcile.Result{}, retry.RetryOnConflict(retry.DefaultBackoff, func() error {

		catalogSrc := &operatorsv1alpha1.CatalogSource{}
		catalogSrcNamespacedName := types.NamespacedName{
			Name:      catalogName,
			Namespace: utils.OPERATOR_MKTPLACE_NS}

		// If InstallIBMCatalogSource is true: install Catalog Source
		// if InstallIBMCatalogSource is false: do not install Catalog Source, and delete existing one (if it exists)
		if ptr.ToBool(instance.Spec.InstallIBMCatalogSource) {
			// If the Catalog Source does not exist, create one
			if err := r.Client.Get(context.TODO(), catalogSrcNamespacedName, catalogSrc); err != nil && k8serrors.IsNotFound(err) {
				// Create catalog source
				var newCatalogSrc *operatorsv1alpha1.CatalogSource
				if utils.IBM_CATALOGSRC_NAME == catalogName {
					newCatalogSrc = utils.BuildNewIBMCatalogSrc()
				} else { // utils.OPENCLOUD_CATALOGSRC_NAME
					newCatalogSrc = utils.BuildNewOpencloudCatalogSrc()
				}

				reqLogger.Info("Creating catalog source")
				if err := r.Client.Create(context.TODO(), newCatalogSrc); err != nil {
					reqLogger.Error(err, "Failed to create a CatalogSource.", "CatalogSource.Namespace ", newCatalogSrc.Namespace, "CatalogSource.Name", newCatalogSrc.Name)
					return err
				}

				ok := instance.Status.Conditions.SetCondition(status.Condition{
					Type:    marketplacev1alpha1.ConditionInstalling,
					Status:  corev1.ConditionTrue,
					Reason:  marketplacev1alpha1.ReasonCatalogSourceInstall,
					Message: catalogName + " catalog source installed.",
				})

				if ok {
					return r.Client.Status().Update(context.TODO(), instance)
				}

				return nil
			} else if err != nil {
				// Could not get catalog source
				reqLogger.Error(err, "Failed to get CatalogSource", "CatalogSource.Namespace ", catalogSrcNamespacedName.Namespace, "CatalogSource.Name", catalogSrcNamespacedName.Name)
				return err
			}
		} else {
			// Delete catalog source, if it contains our label
			if err := r.Client.Get(context.TODO(), catalogSrcNamespacedName, catalogSrc); err != nil {
				if catalogSrc.Labels[utils.OperatorTag] == utils.OperatorTagValue {
					if err := r.Client.Delete(context.TODO(), catalogSrc); err != nil {
						reqLogger.Info("Failed to delete the existing CatalogSource.", "CatalogSource.Namespace ", catalogSrc.Namespace, "CatalogSource.Name", catalogSrc.Name)
						return err
					}

					ok := instance.Status.Conditions.SetCondition(status.Condition{
						Type:    marketplacev1alpha1.ConditionInstalling,
						Status:  corev1.ConditionTrue,
						Reason:  marketplacev1alpha1.ReasonCatalogSourceDelete,
						Message: catalogName + " catalog source deleted.",
					})

					if ok {
						return r.Client.Status().Update(context.TODO(), instance)
					}
				}
			}
		}

		return nil
	})
}
