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
	"fmt"
	"os"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/marketplace"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	DEFAULT_IMAGE_MARKETPLACE_AGENT = "marketplace-agent:latest"
	IBM_CATALOG_SOURCE_FLAG         = true
)

var (
	//log                      = logf.Log.WithName("controller_marketplaceconfig")
	generateMetricsFlag = false
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
	cc     ClientCommandRunner
	cfg    *config.OperatorConfig
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=secret,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=marketplaceconfigs;marketplaceconfigs/finalizers;marketplaceconfigs/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=marketplaceconfigs;marketplaceconfigs/finalizers;marketplaceconfigs/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=razeedeployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=razeedeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterbases,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=meterbases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="operators.coreos.com",resources=catalogsources,verbs=create;get;list;watch
// +kubebuilder:rbac:groups="operators.coreos.com",resources=catalogsources,verbs=delete,resourceNames=ibm-operator-catalog

// Reconcile reads that state of the cluster for a MarketplaceConfig object and makes changes based on the state read
// and what is in the MarketplaceConfig.Spec
func (r *MarketplaceConfigReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MarketplaceConfig")

	// Fetch the MarketplaceConfig instance
	marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, marketplaceConfig)
	if err != nil {
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

	// run the finalizers
	// Check for deletion and run cleanup
	isMarketplaceConfigMarkedToBeDeleted := marketplaceConfig.GetDeletionTimestamp() != nil
	if isMarketplaceConfigMarkedToBeDeleted {
		// Cleanup. Unregister. Garbage Collection should delete remaining owned resources
		secret := &v1.Secret{}
		err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.RHMPullSecretName, Namespace: request.Namespace}, secret)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				reqLogger.Error(err, "Secret not found. Skipping unregister")
			} else if err != nil {
				reqLogger.Error(err, "Failed to get Secret")
				return reconcile.Result{}, err
			} else {
				// Attempt Unregister
				pullSecret, ok := secret.Data[utils.RHMPullSecretKey]
				if !ok {
					reqLogger.Error(err, "Secret did not contain pull secret key. Skipping unregister")
				} else {
					token := string(pullSecret)
					tokenClaims, err := marketplace.GetJWTTokenClaim(token)
					if err != nil {
						reqLogger.Error(err, "error parsing token")
						return reconcile.Result{}, err
					}

					marketplaceClient, err := marketplace.NewMarketplaceClientBuilder(r.cfg).NewMarketplaceClient(token, tokenClaims)
					if err != nil {
						reqLogger.Error(err, "error constructing marketplace client")
						return reconcile.Result{}, err
					}

					err = r.unregister(marketplaceConfig, marketplaceClient, request, reqLogger)
					if err != nil {
						return reconcile.Result{}, err
					}
				}
			}
		}

		// Remove Finalizer
		controllerutil.RemoveFinalizer(marketplaceConfig, utils.CONTROLLER_FINALIZER)
		err := r.Client.Update(context.TODO(), marketplaceConfig)
		if err != nil {
			reqLogger.Error(err, "Failed to update MarketplaceConfig")
			return reconcile.Result{}, err
		}

		reqLogger.Info("Delete is complete.")
		return reconcile.Result{}, nil
	}

	// Add Finalizer
	if !controllerutil.ContainsFinalizer(marketplaceConfig, utils.CONTROLLER_FINALIZER) {
		controllerutil.AddFinalizer(marketplaceConfig, utils.CONTROLLER_FINALIZER)
		err = r.Client.Update(context.TODO(), marketplaceConfig)
		if err != nil {
			reqLogger.Error(err, "Failed to update MarketplaceConfig")
			return reconcile.Result{}, err
		}
	}
	// Set default namespaces for workload monitoring
	if marketplaceConfig.Spec.NamespaceLabelSelector == nil {
		marketplaceConfig.Spec.NamespaceLabelSelector = &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "openshift.io/cluster-monitoring",
					Operator: "DoesNotExist",
				},
			},
		}

		err = r.Client.Update(context.TODO(), marketplaceConfig)
		if err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{Requeue: true}, nil
	}

	//Initialize enabled features if not set
	if marketplaceConfig.Spec.Features == nil {
		marketplaceConfig.Spec.Features = &common.Features{
			Deployment:                         ptr.Bool(true),
			Registration:                       ptr.Bool(true),
			EnableMeterDefinitionCatalogServer: ptr.Bool(false),
		}
	} else {
		var updateMarketplaceConfig bool
		if marketplaceConfig.Spec.Features.Deployment == nil {
			updateMarketplaceConfig = true
			marketplaceConfig.Spec.Features.Deployment = ptr.Bool(true)
		}
		if marketplaceConfig.Spec.Features.Registration == nil {
			updateMarketplaceConfig = true
			marketplaceConfig.Spec.Features.Registration = ptr.Bool(true)
		}
		if marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer == nil {
			reqLogger.Info("updating marketplaceConfig.Spec.Features.MeterDefinitionCatalogServer")
			updateMarketplaceConfig = true
			marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer = ptr.Bool(false)
		}

		if updateMarketplaceConfig {
			err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				return r.Client.Update(context.TODO(), marketplaceConfig)
			})
			if err != nil {
				reqLogger.Error(err, "failed to update marketplaceconfig")
				return reconcile.Result{}, err
			}

			return reconcile.Result{Requeue: true}, nil
		}
	}

	// Removing EnabledMetering field so setting them all to nil
	// this will no longer do anything
	if marketplaceConfig.Spec.EnableMetering != nil {
		marketplaceConfig.Spec.EnableMetering = nil
	}

	// if the operator is running in a disconnected environment just update the marketplaceconfig status and apply meterbase cr
	if r.cfg.IsDisconnected {
		if *marketplaceConfig.Spec.Features.Deployment ||
			*marketplaceConfig.Spec.Features.Registration {
			err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				err = r.Client.Get(context.TODO(), client.ObjectKeyFromObject(marketplaceConfig), marketplaceConfig)
				if err != nil {
					return err
				}

				//Initialize enabled features if not set
				if marketplaceConfig.Spec.Features == nil {
					marketplaceConfig.Spec.Features = &common.Features{}
				}

				marketplaceConfig.Spec.Features.Deployment = ptr.Bool(false)
				marketplaceConfig.Spec.Features.Registration = ptr.Bool(false)
				marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer = ptr.Bool(false)

				return r.Client.Update(context.TODO(), marketplaceConfig)
			})

			if err != nil {
				reqLogger.Error(err, "Failed to update marketplaceconfig.")
				return reconcile.Result{}, err
			}

			return reconcile.Result{Requeue: true}, nil
		}

		if marketplaceConfig.Status.Conditions.IsUnknownFor(marketplacev1alpha1.ConditionInstalling) {
			ok := marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionIsDisconnected,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonInternetDisconnected,
				Message: "Detected disconnected environment",
			})
			if ok {
				err = r.Client.Status().Update(context.TODO(), marketplaceConfig)

				if err != nil {
					reqLogger.Error(err, "Failed to update marketplaceconfig status.")
					return reconcile.Result{}, err
				}

				return reconcile.Result{Requeue: true}, nil
			}
		}

		foundMeterBase := &marketplacev1alpha1.MeterBase{}

		err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.METERBASE_NAME, Namespace: marketplaceConfig.Namespace}, foundMeterBase)
		if k8serrors.IsNotFound(err) {
			newMeterBaseCr := utils.BuildMeterBaseCr(
				marketplaceConfig.Namespace,
				*marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer,
			)

			if err = controllerutil.SetControllerReference(marketplaceConfig, newMeterBaseCr, r.Scheme); err != nil {
				reqLogger.Error(err, "Failed to set controller ref")
				return reconcile.Result{}, err
			}

			reqLogger.Info("creating meterbase")
			err = r.Client.Create(context.TODO(), newMeterBaseCr)
			if err != nil {
				reqLogger.Error(err, "Failed to create a new MeterBase CR.")
				return reconcile.Result{}, err
			}

			ok := marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonMeterBaseInstalled,
				Message: "Meter base installed.",
			})

			if ok {
				err = r.Client.Status().Update(context.TODO(), marketplaceConfig)

				if err != nil {
					reqLogger.Error(err, "failed to update status")
					return reconcile.Result{}, err
				}
			}

			return reconcile.Result{Requeue: true}, nil
		} else if err != nil {
			reqLogger.Error(err, "Failed to get MeterBase CR")
			return reconcile.Result{}, err
		}

		reqLogger.Info("found meterbase")

		var updated bool

		if marketplaceConfig.Status.MeterBaseSubConditions == nil {
			marketplaceConfig.Status.MeterBaseSubConditions = status.Conditions{}
		}

		// update meter definition catalog config
		result, err := func() (reconcile.Result, error) {
			catalogServerEnabled := true

			if marketplaceConfig.Spec.Features != nil &&
				marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer != nil {
				catalogServerEnabled = *marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer
			}

			update := utils.UpdateMeterDefinitionCatalogConfig(foundMeterBase, utils.NewMeterDefinitionCatalogConfig(
				catalogServerEnabled,
			))

			if !update {
				return reconcile.Result{}, nil
			}

			err = r.Client.Update(context.TODO(), foundMeterBase)
			if err != nil {
				reqLogger.Error(err, "Failed to update meterbase")
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		}()

		if result.Requeue || err != nil {
			return result, err
		}

		if foundMeterBase != nil && foundMeterBase.Status.Conditions != nil {
			if !utils.ConditionsEqual(
				foundMeterBase.Status.Conditions,
				marketplaceConfig.Status.MeterBaseSubConditions) {
				marketplaceConfig.Status.MeterBaseSubConditions = foundMeterBase.Status.Conditions
				updated = updated || true
			}
		}

		if updated {
			err = r.Client.Status().Update(context.TODO(), marketplaceConfig)
			if err != nil {
				reqLogger.Error(err, "Failed to update status")
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.Info("finished install for disconnected environments")
	}

	deployedNamespace := &corev1.Namespace{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: r.cfg.DeployedNamespace}, deployedNamespace)
	if err != nil {
		reqLogger.Error(err, "err getting deployed ns")
	}

	if deployedNamespace.Labels == nil {
		deployedNamespace.Labels = make(map[string]string)
	}

	if v, ok := deployedNamespace.Labels[utils.LicenseServerTag]; !ok || v != "true" {
		deployedNamespace.Labels[utils.LicenseServerTag] = "true"

		err = r.Client.Update(context.TODO(), deployedNamespace)
		if err != nil {
			reqLogger.Error(err, "Failed to update deployed namespace with license server tag")
			return reconcile.Result{}, err
		}

		return reconcile.Result{Requeue: true}, nil
	}

	//Fetch the redhat-marketplace-pull-secret or ibm-entitlement-key
	si, err := utils.ReturnSecret(r.Client, request)
	if err != nil {
		return reconcile.Result{}, err
	}

	reqLogger.Info("found secret", "secret", si.Name)

	if marketplaceConfig.Labels == nil {
		marketplaceConfig.Labels = make(map[string]string)
	}

	var updateInstanceSpec bool

	if si.Secret != nil {
		if clusterDisplayName, ok := si.Secret.Data[utils.ClusterDisplayNameKey]; ok {
			count := utf8.RuneCountInString(string(clusterDisplayName))
			clusterName := strings.Trim(string(clusterDisplayName), "\n")

			if marketplaceConfig.Spec.ClusterName != clusterName {
				if count <= 256 {
					marketplaceConfig.Spec.ClusterName = clusterName
					updateInstanceSpec = true
					reqLogger.Info("setting ClusterName", "name", clusterName)
				} else {
					err := errors.New("CLUSTER_DISPLAY_NAME exceeds 256 chars")
					reqLogger.Error(err, "name", clusterDisplayName)
				}
			}
		}
	}

	if v, ok := marketplaceConfig.Labels[utils.RazeeWatchResource]; !ok || v != utils.RazeeWatchLevelDetail {
		updateInstanceSpec = true
		marketplaceConfig.Labels[utils.RazeeWatchResource] = utils.RazeeWatchLevelDetail
	}

	if updateInstanceSpec {
		err = r.Client.Update(context.TODO(), marketplaceConfig)

		if err != nil {
			reqLogger.Error(err, "Failed to update the marketplace config")
			return reconcile.Result{}, err
		}

		return reconcile.Result{Requeue: true}, nil
	}

	if marketplaceConfig.Status.Conditions.IsUnknownFor(marketplacev1alpha1.ConditionInstalling) {
		ok := marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionInstalling,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonStartInstall,
			Message: "Installing starting",
		})

		if ok {
			err = r.Client.Status().Update(context.TODO(), marketplaceConfig)

			if err != nil {
				reqLogger.Error(err, "Failed to create a new RazeeDeployment CR.")
				return reconcile.Result{}, err
			}

			return reconcile.Result{Requeue: true}, nil
		}
	}

	var foundRazee *marketplacev1alpha1.RazeeDeployment

	//Check if RazeeDeployment exists, if not create one
	foundRazee = &marketplacev1alpha1.RazeeDeployment{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.RAZEE_NAME, Namespace: marketplaceConfig.Namespace}, foundRazee)
	if err != nil && k8serrors.IsNotFound(err) {
		newRazeeCrd := utils.BuildRazeeCr(marketplaceConfig.Namespace, marketplaceConfig.Spec.ClusterUUID, marketplaceConfig.Spec.DeploySecretName, marketplaceConfig.Spec.Features)

		// Sets the owner for foundRazee
		if err = controllerutil.SetControllerReference(marketplaceConfig, newRazeeCrd, r.Scheme); err != nil {
			reqLogger.Error(err, "Failed to create a new RazeeDeployment CR.")
			return reconcile.Result{}, err
		}

		// include a display name if set
		if marketplaceConfig.Spec.ClusterName != "" {
			reqLogger.Info("setting cluster name override on razee cr")
			newRazeeCrd.Spec.ClusterDisplayName = marketplaceConfig.Spec.ClusterName
		}

		reqLogger.Info("creating razee cr")
		err = r.Client.Create(context.TODO(), newRazeeCrd)

		if err != nil {
			reqLogger.Error(err, "Failed to create a new RazeeDeployment CR.")
			return reconcile.Result{}, err
		}

		ok := marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionInstalling,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonRazeeInstalled,
			Message: "RazeeDeployment installed.",
		})

		if ok {
			err = r.Client.Status().Update(context.TODO(), marketplaceConfig)

			if err != nil {
				reqLogger.Error(err, "failed to update status")
				return reconcile.Result{}, err
			}
		}

		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get RazeeDeployment CR")
		return reconcile.Result{}, err
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.RAZEE_NAME, Namespace: marketplaceConfig.Namespace}, foundRazee)
		if err != nil {
			return err
		}

		updatedRazee := foundRazee.DeepCopy()
		updatedRazee.Spec.ClusterUUID = marketplaceConfig.Spec.ClusterUUID
		updatedRazee.Spec.DeploySecretName = marketplaceConfig.Spec.DeploySecretName
		updatedRazee.Spec.Features = marketplaceConfig.Spec.Features.DeepCopy()

		if marketplaceConfig.Spec.ClusterName != "" {
			if !reflect.DeepEqual(marketplaceConfig.Spec.ClusterName, foundRazee.Spec.ClusterDisplayName) {
				updatedRazee.Spec.ClusterDisplayName = marketplaceConfig.Spec.ClusterName
			}
		}

		if !reflect.DeepEqual(foundRazee, updatedRazee) {
			reqLogger.Info("updating razee cr")
			err = r.Client.Update(context.TODO(), updatedRazee)

			if err != nil {
				return err
			}

			ok := marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonRazeeInstalled,
				Message: "RazeeDeployment updated.",
			})

			if ok {
				_ = r.Client.Status().Update(context.TODO(), marketplaceConfig)
			}

			return err
		}

		return nil
	})

	if err != nil {
		reqLogger.Error(err, "failed to update razee")
		return reconcile.Result{}, err
	}

	foundMeterBase := &marketplacev1alpha1.MeterBase{}

	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.METERBASE_NAME, Namespace: marketplaceConfig.Namespace}, foundMeterBase)
	if k8serrors.IsNotFound(err) {
		reqLogger.Info("catalog server enabled", "enabled", marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer)
		newMeterBaseCr := utils.BuildMeterBaseCr(
			marketplaceConfig.Namespace,
			*marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer,
		)

		if err = controllerutil.SetControllerReference(marketplaceConfig, newMeterBaseCr, r.Scheme); err != nil {
			reqLogger.Error(err, "Failed to set controller ref")
			return reconcile.Result{}, err
		}

		reqLogger.Info("creating meterbase")
		err = r.Client.Create(context.TODO(), newMeterBaseCr)
		if err != nil {
			reqLogger.Error(err, "Failed to create a new MeterBase CR.")
			return reconcile.Result{}, err
		}

		ok := marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionInstalling,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonMeterBaseInstalled,
			Message: "Meter base installed.",
		})

		if ok {
			err = r.Client.Status().Update(context.TODO(), marketplaceConfig)

			if err != nil {
				reqLogger.Error(err, "failed to update status")
				return reconcile.Result{}, err
			}
		}

		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get MeterBase CR")
		return reconcile.Result{}, err
	}
	reqLogger.Info("found meterbase")

	if *marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer {
		// meterbase doesn't have MeterdefinitionCatalogServerConfig on MeterBase.Spec.
		// Set the stuct and set all flags to true
		if foundMeterBase.Spec.MeterdefinitionCatalogServerConfig == nil {
			reqLogger.Info("enabling MeterDefinitionCatalogServerConfig values")
			foundMeterBase.Spec.MeterdefinitionCatalogServerConfig = &common.MeterDefinitionCatalogServerConfig{
				//TODO: are we setting this to false in production ?
				SyncCommunityMeterDefinitions:      false,
				SyncSystemMeterDefinitions:         false,
				DeployMeterDefinitionCatalogServer: false,
			}

			reqLogger.Info("setting MeterdefinitionCatalog features")

			err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				return r.Client.Update(context.TODO(), foundMeterBase)
			})
			if err != nil {
				reqLogger.Error(err, "failed to update meterbase")
				return reconcile.Result{}, err
			}
		}

		// foundMeterBase.Spec.MeterdefinitionCatalogServerConfig already exists
		// just allow for toggling the deployment of the file server - leave individual sync flags alone
		if foundMeterBase.Spec.MeterdefinitionCatalogServerConfig != nil && !foundMeterBase.Spec.MeterdefinitionCatalogServerConfig.DeployMeterDefinitionCatalogServer {
			reqLogger.Info("enabling MeterDefinitionCatalogServerConfig values")
			foundMeterBase.Spec.MeterdefinitionCatalogServerConfig = &common.MeterDefinitionCatalogServerConfig{
				DeployMeterDefinitionCatalogServer: false,
			}

			reqLogger.Info("setting MeterdefinitionCatalog features")

			err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				return r.Client.Update(context.TODO(), foundMeterBase)
			})
			if err != nil {
				reqLogger.Error(err, "failed to update meterbase")
				return reconcile.Result{}, err
			}
		}
	}

	// meterdef catalog server disabled, set all flags to false. This will remove file server resources and all community & system meterdefs
	if !*marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer && foundMeterBase.Spec.MeterdefinitionCatalogServerConfig != nil {
		reqLogger.Info("disabling MeterDefinitionCatalogServerConfig values")

		foundMeterBase.Spec.MeterdefinitionCatalogServerConfig = &common.MeterDefinitionCatalogServerConfig{
			//TODO: probably not necessary but setting to false here just to be safe
			SyncCommunityMeterDefinitions:      false,
			SyncSystemMeterDefinitions:         false,
			DeployMeterDefinitionCatalogServer: false,
		}

		reqLogger.Info("disabling MeterdefinitionCatalog features")

		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			return r.Client.Update(context.TODO(), foundMeterBase)
		})
		if err != nil {
			reqLogger.Error(err, "failed to update meterbase")
			return reconcile.Result{}, err
		}
	}

	for _, catalogSrcName := range [2]string{utils.IBM_CATALOGSRC_NAME, utils.OPENCLOUD_CATALOGSRC_NAME} {
		requeueFlag, err := r.createCatalogSource(request, marketplaceConfig, catalogSrcName)
		if requeueFlag && err == nil {
			return reconcile.Result{Requeue: true}, nil
		}
	}

	var updated bool

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

	if marketplaceConfig.Status.RazeeSubConditions == nil {
		marketplaceConfig.Status.RazeeSubConditions = status.Conditions{}
	}

	if foundRazee != nil && foundRazee.Status.Conditions != nil {
		if !utils.ConditionsEqual(
			foundRazee.Status.Conditions,
			marketplaceConfig.Status.RazeeSubConditions) {
			marketplaceConfig.Status.RazeeSubConditions = foundRazee.Status.Conditions
			updated = updated || true
		}
	}

	if marketplaceConfig.Status.MeterBaseSubConditions == nil {
		marketplaceConfig.Status.MeterBaseSubConditions = status.Conditions{}
	}

	if foundMeterBase != nil && foundMeterBase.Status.Conditions != nil {
		if !utils.ConditionsEqual(
			foundMeterBase.Status.Conditions,
			marketplaceConfig.Status.MeterBaseSubConditions) {
			marketplaceConfig.Status.MeterBaseSubConditions = foundMeterBase.Status.Conditions
			updated = updated || true
		}
	}

	reqLogger.Info("Finding Cluster registration status")

	requeueResult, requeue, err := func() (reconcile.Result, bool, error) {
		if si.Secret == nil {
			return reconcile.Result{}, false, nil
		}

		token, err := utils.ParseAndValidate(si)
		if err != nil {
			reqLogger.Error(err, "error validating secret")
			return reconcile.Result{}, false, err
		}
		tokenClaims, err := marketplace.GetJWTTokenClaim(token)
		if err != nil {
			reqLogger.Error(err, "error parsing token")
			return reconcile.Result{Requeue: true}, false, nil
		}

		reqLogger.Info("attempting to update registration")
		marketplaceClient, err := marketplace.NewMarketplaceClientBuilder(r.cfg).
			NewMarketplaceClient(token, tokenClaims)

		if err != nil {
			reqLogger.Error(err, "error constructing marketplace client")
			return reconcile.Result{Requeue: true}, true, err
		}

		marketplaceClientAccount := &marketplace.MarketplaceClientAccount{
			AccountId:   marketplaceConfig.Spec.RhmAccountID,
			ClusterUuid: marketplaceConfig.Spec.ClusterUUID,
		}

		registrationStatusOutput, err := marketplaceClient.RegistrationStatus(marketplaceClientAccount)
		if err != nil {
			reqLogger.Error(err, "registration status failed")
			return reconcile.Result{Requeue: true}, true, err
		}

		reqLogger.Info("attempting to update registration", "status", registrationStatusOutput.RegistrationStatus)

		statusConditions := registrationStatusOutput.TransformConfigStatus()

		for _, cond := range statusConditions {
			updated = updated || marketplaceConfig.Status.Conditions.SetCondition(cond)
		}

		return reconcile.Result{}, false, nil
	}()

	if requeue || err != nil {
		return requeueResult, err
	}

	if updated {
		//Updating Marketplace Config with Cluster Registration status
		err = r.Client.Status().Update(context.TODO(), marketplaceConfig)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	reqLogger.Info("reconciling finished")
	return reconcile.Result{}, nil
}

// labelsForMarketplaceConfig returs the labels for selecting the resources
// belonging to the given marketplaceConfig custom resource name
func labelsForMarketplaceConfig(name string) map[string]string {
	return map[string]string{"app": "marketplaceconfig", "marketplaceconfig_cr": name}
}

// Begin installation or deletion of Catalog Source
func (r *MarketplaceConfigReconciler) createCatalogSource(request reconcile.Request, marketplaceConfig *marketplacev1alpha1.MarketplaceConfig, catalogName string) (bool, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name, "CatalogSource.Name", catalogName)

	// Get installation setting for Catalog Source (checks MarketplaceConfig.Spec if it doesn't exist, use flag)
	installCatalogSrcP := marketplaceConfig.Spec.InstallIBMCatalogSource
	var installCatalogSrc bool

	if installCatalogSrcP == nil {
		reqLogger.Info("MarketplaceConfig.Spec.InstallIBMCatalogSource not found. Using flag.")
		installCatalogSrc = r.cfg.Features.IBMCatalog

		marketplaceConfig.Spec.InstallIBMCatalogSource = &installCatalogSrc
		r.Client.Update(context.TODO(), marketplaceConfig)
		return true, nil
	} else {
		reqLogger.Info("MarketplaceConfig.Spec.InstallIBMCatalogSource found")
		installCatalogSrc = *installCatalogSrcP
	}

	// Check if the Catalog Source exists.
	catalogSrc := &operatorsv1alpha1.CatalogSource{}
	catalogSrcNamespacedName := types.NamespacedName{
		Name:      catalogName,
		Namespace: utils.OPERATOR_MKTPLACE_NS}
	err := r.Client.Get(context.TODO(), catalogSrcNamespacedName, catalogSrc)

	// If installCatalogSrc is true: install Catalog Source
	// if installCatalogSrc is false: do not install Catalog Source, and delete existing one (if it exists)
	reqLogger.Info("Checking Install Catalog Src", "InstallCatalogSource: ", installCatalogSrc)
	if installCatalogSrc {
		// If the Catalog Source does not exist, create one
		if err != nil && k8serrors.IsNotFound(err) {
			// Create catalog source
			var newCatalogSrc *operatorsv1alpha1.CatalogSource
			if utils.IBM_CATALOGSRC_NAME == catalogName {
				newCatalogSrc = utils.BuildNewIBMCatalogSrc()
			} else { // utils.OPENCLOUD_CATALOGSRC_NAME
				newCatalogSrc = utils.BuildNewOpencloudCatalogSrc()
			}

			reqLogger.Info("Creating catalog source")
			err = r.Client.Create(context.TODO(), newCatalogSrc)
			if err != nil {
				reqLogger.Error(err, "Failed to create a CatalogSource.", "CatalogSource.Namespace ", newCatalogSrc.Namespace, "CatalogSource.Name", newCatalogSrc.Name)
				return false, err
			}

			ok := marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonCatalogSourceInstall,
				Message: catalogName + " catalog source installed.",
			})

			if ok {
				err = r.Client.Status().Update(context.TODO(), marketplaceConfig)

				if err != nil {
					reqLogger.Error(err, "failed to update status")
					return false, err
				}
			}

			// catalog source created successfully - return and requeue
			return true, nil
		} else if err != nil {
			// Could not get catalog source
			reqLogger.Error(err, "Failed to get CatalogSource", "CatalogSource.Namespace ", catalogSrcNamespacedName.Namespace, "CatalogSource.Name", catalogSrcNamespacedName.Name)
			return false, err
		}

		reqLogger.Info("Found CatalogSource", "CatalogSource.Namespace ", catalogSrcNamespacedName.Namespace, "CatalogSource.Name", catalogSrcNamespacedName.Name)
	} else {
		// If catalog source exists, delete it.
		if err == nil {
			// Delete catalog source.
			reqLogger.Info("Deleting catalog source")
			catalogSrc.Name = catalogSrcNamespacedName.Name
			catalogSrc.Namespace = catalogSrcNamespacedName.Namespace
			err = r.Client.Delete(context.TODO(), catalogSrc, client.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil {
				reqLogger.Info("Failed to delete the existing CatalogSource.", "CatalogSource.Namespace ", catalogSrc.Namespace, "CatalogSource.Name", catalogSrc.Name)
				return false, err
			}

			ok := marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonCatalogSourceDelete,
				Message: catalogName + " catalog source deleted.",
			})

			if ok {
				err = r.Client.Status().Update(context.TODO(), marketplaceConfig)
				if err != nil {
					reqLogger.Error(err, "failed to update status")
					return false, err
				}
			}

			// catalog source deleted successfully - return and requeue
			return true, nil
		} else if err != nil && !k8serrors.IsNotFound(err) {
			// Could not get catalog source
			reqLogger.Error(err, "Failed to get CatalogSource", "CatalogSource.Namespace ", catalogSrcNamespacedName.Namespace, "CatalogSource.Name", catalogSrcNamespacedName.Name)
			return false, err
		}

		reqLogger.Info(catalogName + " catalog Source does not exist.")
	}
	return false, nil
}

func (r *MarketplaceConfigReconciler) unregister(marketplaceConfig *marketplacev1alpha1.MarketplaceConfig, marketplaceClient *marketplace.MarketplaceClient, request reconcile.Request, reqLogger logr.Logger) error {
	reqLogger.Info("attempting to un-register")

	marketplaceClientAccount := &marketplace.MarketplaceClientAccount{
		AccountId:   marketplaceConfig.Spec.RhmAccountID,
		ClusterUuid: marketplaceConfig.Spec.ClusterUUID,
	}

	reqLogger.Info("unregister", "marketplace client account", marketplaceClientAccount)

	registrationStatusOutput, err := marketplaceClient.UnRegister(marketplaceClientAccount)
	if err != nil {
		reqLogger.Error(err, "unregister failed")
		return err
	}

	reqLogger.Info("unregister", "RegistrationStatus", registrationStatusOutput.RegistrationStatus)

	return err
}

func (r *MarketplaceConfigReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (r *MarketplaceConfigReconciler) InjectCommandRunner(ccp ClientCommandRunner) error {
	r.cc = ccp
	return nil
}

func (m *MarketplaceConfigReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	m.cfg = cfg
	return nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *MarketplaceConfigReconciler) SetupWithManager(mgr manager.Manager) error {
	// Create a new controller
	ownerHandler := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MarketplaceConfig{},
	}

	namespacePredicate := predicates.NamespacePredicate(r.cfg.DeployedNamespace)

	return ctrl.NewControllerManagedBy(mgr).
		For(&marketplacev1alpha1.MarketplaceConfig{}).
		WithEventFilter(namespacePredicate).
		Watches(&source.Kind{Type: &marketplacev1alpha1.RazeeDeployment{}}, ownerHandler).
		Watches(&source.Kind{Type: &marketplacev1alpha1.MeterBase{}}, ownerHandler).
		Watches(&source.Kind{Type: &marketplacev1alpha1.RazeeDeployment{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &marketplacev1alpha1.MarketplaceConfig{},
		}).
		Watches(&source.Kind{Type: &marketplacev1alpha1.MeterBase{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &marketplacev1alpha1.MarketplaceConfig{},
		}).
		Complete(r)
}

// getOperatorGroup returns the associated OLM OperatorGroup
func getOperatorGroup() (string, error) {
	// OperatorGroupEnvVar is the constant for env variable OPERATOR_GROUP
	// which is annotated as olm.operatorGroup
	var operatorGroupEnvVar = "OPERATOR_GROUP"

	og, found := os.LookupEnv(operatorGroupEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", operatorGroupEnvVar)
	}
	return og, nil
}
