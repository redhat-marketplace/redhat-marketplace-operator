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
	"github.com/google/uuid"
	"github.com/gotidy/ptr"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
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

	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
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

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",namespace=system,resources=namespaces,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",namespace=system,resources=secret,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",namespace=system,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",namespace=system,resources=deployments/finalizers,verbs=get;list;watch;update;patch,resourceNames=redhat-marketplace-controller-manager
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=marketplaceconfigs;marketplaceconfigs/finalizers;marketplaceconfigs/status,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=razeedeployments,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=razeedeployments,verbs=update;patch;delete,resourceNames=rhm-marketplaceconfig-razeedeployment
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=meterbases,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=meterbases,verbs=update;patch;delete,resourceNames=rhm-marketplaceconfig-meterbase
// +kubebuilder:rbac:groups="operators.coreos.com",resources=catalogsources,verbs=create;get;list;watch
// +kubebuilder:rbac:groups="operators.coreos.com",resources=catalogsources,verbs=delete,resourceNames=ibm-operator-catalog;opencloud-operators

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

	secretFetcher := utils.ProvideSecretFetcherBuilder(r.Client, context.TODO(), request.Namespace)

	// run the finalizers
	// Check for deletion and run cleanup
	isMarketplaceConfigMarkedToBeDeleted := marketplaceConfig.GetDeletionTimestamp() != nil
	if isMarketplaceConfigMarkedToBeDeleted {
		// Cleanup. Unregister. Garbage Collection should delete remaining owned resources

		si, err := secretFetcher.ReturnSecret()
		if err != nil {
			if errors.Is(err, utils.NoSecretsFound) {
				reqLogger.Error(err, "Secret not found. Skipping unregister")
			} else {
				reqLogger.Error(err, "Failed to get secret")
				return reconcile.Result{}, err
			}
		} else {
			//Attempt to unregister
			token, err := secretFetcher.ParseAndValidate(si)
			if err != nil {
				reqLogger.Error(err, "error validating secret skipping unregister")
			} else {
				//Continue with unregister
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

		// Remove Finalizer
		controllerutil.RemoveFinalizer(marketplaceConfig, utils.CONTROLLER_FINALIZER)
		err = r.Client.Update(context.TODO(), marketplaceConfig)
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
		if err := r.Client.Update(context.TODO(), marketplaceConfig); err != nil {
			reqLogger.Error(err, "Failed to update MarketplaceConfig")
			return reconcile.Result{}, err
		}
	}

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

	// This has no watch/update, only create/delete
	for _, catalogSrcName := range [2]string{utils.IBM_CATALOGSRC_NAME, utils.OPENCLOUD_CATALOGSRC_NAME} {
		if result, err := r.createCatalogSource(request, catalogSrcName); err != nil {
			return result, err
		}
	}

	// Install is complete, set Status
	if result, err := r.updateMarketplaceConfigStatusFinished(request); err != nil {
		return result, err
	}

	// Update registration status
	if result, err := r.findRegistrationStatus(request, secretFetcher); err != nil {
		return result, err
	}

	reqLogger.Info("reconciling finished")
	return reconcile.Result{}, nil
}

// labelsForMarketplaceConfig returs the labels for selecting the resources
// belonging to the given marketplaceConfig custom resource name
func labelsForMarketplaceConfig(name string) map[string]string {
	return map[string]string{"app": "marketplaceconfig", "marketplaceconfig_cr": name}
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

		if *marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer {
			// if meterbase doesn't have MeterdefinitionCatalogServerConfig on MeterBase.Spec.
			// Set the struct and set all flags to true
			if meterBase.Spec.MeterdefinitionCatalogServerConfig == nil {
				reqLogger.Info("enabling MeterDefinitionCatalogServerConfig values")
				meterBase.Spec.MeterdefinitionCatalogServerConfig = &common.MeterDefinitionCatalogServerConfig{
					SyncCommunityMeterDefinitions:      true,
					SyncSystemMeterDefinitions:         false, //always making this false for now
					DeployMeterDefinitionCatalogServer: !*marketplaceConfig.Spec.IsDisconnected && *marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer,
				}
			} else {
				// foundMeterBase.Spec.MeterdefinitionCatalogServerConfig already exists
				// just allow for toggling the deployment of the file server - leave individual sync flags alone
				if !meterBase.Spec.MeterdefinitionCatalogServerConfig.DeployMeterDefinitionCatalogServer {
					reqLogger.Info("enabling MeterDefinitionCatalogServerConfig values")
					meterBase.Spec.MeterdefinitionCatalogServerConfig = &common.MeterDefinitionCatalogServerConfig{
						SyncCommunityMeterDefinitions:      meterBase.Spec.MeterdefinitionCatalogServerConfig.SyncCommunityMeterDefinitions,
						SyncSystemMeterDefinitions:         meterBase.Spec.MeterdefinitionCatalogServerConfig.SyncSystemMeterDefinitions,
						DeployMeterDefinitionCatalogServer: !*marketplaceConfig.Spec.IsDisconnected && *marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer,
					}
				}
			}
		}

		// meterdef catalog server disabled, set all flags to false. This will remove file server resources and all community & system meterdefs
		if !*marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer && meterBase.Spec.MeterdefinitionCatalogServerConfig != nil {
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

// Begin installation or deletion of Catalog Source
func (r *MarketplaceConfigReconciler) createCatalogSource(request reconcile.Request, catalogName string) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("func", "createCatalogSource", "Request.Namespace", request.Namespace, "Request.Name", request.Name)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
			reqLogger.Error(err, "failed to get marketplaceconfig")
			return err
		}

		// Get installation setting for Catalog Source (checks MarketplaceConfig.Spec if it doesn't exist, use flag)
		installCatalogSrcP := marketplaceConfig.Spec.InstallIBMCatalogSource
		var installCatalogSrc bool

		if installCatalogSrcP == nil {
			reqLogger.Info("MarketplaceConfig.Spec.InstallIBMCatalogSource not found. Using flag.")
			installCatalogSrc = r.cfg.Features.IBMCatalog

			marketplaceConfig.Spec.InstallIBMCatalogSource = &installCatalogSrc
			reqLogger.Info("updating marketplaceconfig")
			return r.Client.Update(context.TODO(), marketplaceConfig)
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
				if err := r.Client.Create(context.TODO(), newCatalogSrc); err != nil {
					reqLogger.Error(err, "Failed to create a CatalogSource.", "CatalogSource.Namespace ", newCatalogSrc.Namespace, "CatalogSource.Name", newCatalogSrc.Name)
					return err
				}

				ok := marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
					Type:    marketplacev1alpha1.ConditionInstalling,
					Status:  corev1.ConditionTrue,
					Reason:  marketplacev1alpha1.ReasonCatalogSourceInstall,
					Message: catalogName + " catalog source installed.",
				})

				if ok {
					reqLogger.Info("updating marketplaceconfig status")
					return r.Client.Status().Update(context.TODO(), marketplaceConfig)
				}

				return nil
			} else if err != nil {
				// Could not get catalog source
				reqLogger.Error(err, "Failed to get CatalogSource", "CatalogSource.Namespace ", catalogSrcNamespacedName.Namespace, "CatalogSource.Name", catalogSrcNamespacedName.Name)
				return err
			}

			reqLogger.Info("Found CatalogSource", "CatalogSource.Namespace ", catalogSrcNamespacedName.Namespace, "CatalogSource.Name", catalogSrcNamespacedName.Name)
		} else {
			// If catalog source exists, delete it.
			if err == nil {
				// Delete catalog source.
				reqLogger.Info("Deleting catalog source")
				catalogSrc.Name = catalogSrcNamespacedName.Name
				catalogSrc.Namespace = catalogSrcNamespacedName.Namespace
				if err = r.Client.Delete(context.TODO(), catalogSrc, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
					reqLogger.Info("Failed to delete the existing CatalogSource.", "CatalogSource.Namespace ", catalogSrc.Namespace, "CatalogSource.Name", catalogSrc.Name)
					return err
				}

				ok := marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
					Type:    marketplacev1alpha1.ConditionInstalling,
					Status:  corev1.ConditionTrue,
					Reason:  marketplacev1alpha1.ReasonCatalogSourceDelete,
					Message: catalogName + " catalog source deleted.",
				})

				if ok {
					reqLogger.Info("updating marketplaceconfig status")
					return r.Client.Status().Update(context.TODO(), marketplaceConfig)
				}

				return nil
			} else if err != nil && !k8serrors.IsNotFound(err) {
				// Could not get catalog source
				reqLogger.Error(err, "Failed to get CatalogSource", "CatalogSource.Namespace ", catalogSrcNamespacedName.Namespace, "CatalogSource.Name", catalogSrcNamespacedName.Name)
				return err
			}

			reqLogger.Info(catalogName + " catalog Source does not exist.")
		}

		return nil
	})
	return reconcile.Result{}, err
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

func (r *MarketplaceConfigReconciler) updateDeployedNamespaceLabels(marketplaceConfig *marketplacev1alpha1.MarketplaceConfig) (reconcile.Result, error) {
	// Add License Server tag to deployed namespace
	reqLogger := r.Log.WithValues("func", "updateDeployedNamespaceLabels", "Request.Namespace", marketplaceConfig.Namespace, "Request.Name", marketplaceConfig.Name)
	deployedNamespace := &corev1.Namespace{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: r.cfg.DeployedNamespace}, deployedNamespace); err != nil {
		return reconcile.Result{}, err
	}

	if deployedNamespace.Labels == nil {
		deployedNamespace.Labels = make(map[string]string)
	}

	if v, ok := deployedNamespace.Labels[utils.LicenseServerTag]; !ok || v != "true" {
		deployedNamespace.Labels[utils.LicenseServerTag] = "true"

		reqLogger.Info("updating namespace ")
		if err := r.Client.Update(context.TODO(), deployedNamespace); err != nil {
			reqLogger.Error(err, "Failed to update deployed namespace with license server tag")
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (r *MarketplaceConfigReconciler) initializeMarketplaceConfigSpec(
	request reconcile.Request,
	secretFetcher *utils.SecretFetcherBuilder,
) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("func", "initializeMarketplaceConfigSpec", "Request.Namespace", request.Namespace, "Request.Name", request.Name)

	// Initialize MarketplaceConfigSpec
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
			reqLogger.Error(err, "failed to get marketplaceconfig")
			return err
		}

		marketplaceConfigCopy := marketplaceConfig.DeepCopy()

		// IS_DISCONNECTED flag takes precedence, default IsDisconnected to false
		if r.cfg.IsDisconnected {
			marketplaceConfig.Spec.IsDisconnected = ptr.Bool(true)
		} else if marketplaceConfig.Spec.IsDisconnected == nil {
			marketplaceConfig.Spec.IsDisconnected = ptr.Bool(false)
		}

		// Initialize enabled features if not set, based on IsDisconnected state
		if marketplaceConfig.Spec.Features == nil {
			if *marketplaceConfig.Spec.IsDisconnected {
				marketplaceConfig.Spec.Features = &common.Features{
					Deployment:                         ptr.Bool(false),
					Registration:                       ptr.Bool(false),
					EnableMeterDefinitionCatalogServer: ptr.Bool(false),
				}
			} else {
				marketplaceConfig.Spec.Features = &common.Features{
					Deployment:                         ptr.Bool(true),
					Registration:                       ptr.Bool(true),
					EnableMeterDefinitionCatalogServer: ptr.Bool(false),
				}
			}
		}

		// Initilize individual features if nil or toggle based on IsDisconnected
		if marketplaceConfig.Spec.Features.Deployment == nil || *marketplaceConfig.Spec.IsDisconnected {
			marketplaceConfig.Spec.Features.Deployment = ptr.Bool(!*marketplaceConfig.Spec.IsDisconnected)
		}
		if marketplaceConfig.Spec.Features.Registration == nil || *marketplaceConfig.Spec.IsDisconnected {
			marketplaceConfig.Spec.Features.Registration = ptr.Bool(!*marketplaceConfig.Spec.IsDisconnected)
		}
		if marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer == nil || *marketplaceConfig.Spec.IsDisconnected {
			marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer = ptr.Bool(!*marketplaceConfig.Spec.IsDisconnected)
		}

		// Removing EnabledMetering field so setting them all to nil
		// this will no longer do anything
		if marketplaceConfig.Spec.EnableMetering != nil {
			marketplaceConfig.Spec.EnableMetering = nil
		}

		// Set Cluster DisplayName
		if !*marketplaceConfig.Spec.IsDisconnected {
			si, err := secretFetcher.ReturnSecret()
			if err == nil { // check for missing secret in during status update
				reqLogger.Info("found secret", "secret", si.Name)

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
			Name:      utils.RHM_CONTROLLER_DEPLOYMENT_NAME,
			Namespace: r.cfg.DeployedNamespace,
		}, dep)
		if err != nil {
			return err
		}
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
		if *marketplaceConfig.Spec.IsDisconnected {
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
			!*marketplaceConfig.Spec.IsDisconnected && *marketplaceConfig.Spec.Features.EnableMeterDefinitionCatalogServer,
		)

		if err = controllerutil.SetControllerReference(marketplaceConfig, meterBase, r.Scheme); err != nil {
			reqLogger.Error(err, "Failed to set controller ref")
			return reconcile.Result{}, err
		}

		reqLogger.Info("creating meterbase")
		err = r.Client.Create(context.TODO(), meterBase)
		if err != nil {
			reqLogger.Error(err, "Failed to create a new MeterBase CR.")
			return reconcile.Result{}, err
		}

		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
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
		})
		if err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get meterbase")
		return reconcile.Result{}, err
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
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
	})

	return reconcile.Result{}, err
}

func (r *MarketplaceConfigReconciler) createOrUpdateRazeeRazeeDeployment(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("func", "createOrUpdateRazeeRazeeDeployment", "Request.Namespace", request.Namespace, "Request.Name", request.Name)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
			reqLogger.Error(err, "failed to get marketplaceconfig")
			return err
		}

		//Check if RazeeDeployment exists, if not create one
		razeeDeployment := &marketplacev1alpha1.RazeeDeployment{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.RAZEE_NAME, Namespace: request.Namespace}, razeeDeployment)
		if err != nil && k8serrors.IsNotFound(err) {
			razeeDeployment := utils.BuildRazeeCr(marketplaceConfig.Namespace, marketplaceConfig.Spec.ClusterUUID, marketplaceConfig.Spec.DeploySecretName, marketplaceConfig.Spec.Features)

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

			// Disable razee in disconnected environment
			razeeDeployment.Spec.Enabled = !*marketplaceConfig.Spec.IsDisconnected

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
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	// Keep razeeDeployment spec updated, use retry to avoid conflict with razeedeployment controller: object has been modified
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
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

		razeeDeploymentCopy := razeeDeployment.DeepCopy()

		// Disable razee in disconnected environment
		razeeDeployment.Spec.Enabled = !*marketplaceConfig.Spec.IsDisconnected
		razeeDeployment.Spec.ClusterUUID = marketplaceConfig.Spec.ClusterUUID
		razeeDeployment.Spec.DeploySecretName = marketplaceConfig.Spec.DeploySecretName
		razeeDeployment.Spec.Features = marketplaceConfig.Spec.Features.DeepCopy()

		if marketplaceConfig.Spec.ClusterName != "" {
			razeeDeployment.Spec.ClusterDisplayName = marketplaceConfig.Spec.ClusterName
		}

		if !reflect.DeepEqual(razeeDeploymentCopy.Spec, razeeDeployment.Spec) {
			reqLogger.Info("updating razeedeployment")
			return r.Client.Update(context.TODO(), razeeDeployment)
		}

		return nil
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	// Update MarketplaceConfig with Razee watch label
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
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
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	// Set MarketplaceConfig Conditions
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
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
	})

	return reconcile.Result{}, err
}

func (r *MarketplaceConfigReconciler) findRegistrationStatus(
	request reconcile.Request,
	secretFetcher *utils.SecretFetcherBuilder,
) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("func", "findRegistrationStatus", "Request.Namespace", request.Namespace, "Request.Name", request.Name)

	marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
		reqLogger.Error(err, "failed to get marketplaceconfig")
		return reconcile.Result{}, err
	}

	// clear registration status for disconnected environment
	// registration status for cluster may not be up to date with marketplace while diconnected
	// or may be in error state if the disconnected flag was set incorrect of cluster connectivity state
	if *marketplaceConfig.Spec.IsDisconnected {
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
			if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
				reqLogger.Error(err, "failed to get marketplaceconfig")
				return err
			}

			updated := false
			updated = updated || marketplaceConfig.Status.Conditions.RemoveCondition(marketplacev1alpha1.ConditionRegistered)
			updated = updated || marketplaceConfig.Status.Conditions.RemoveCondition(marketplacev1alpha1.ConditionRegistrationError)

			if updated {
				reqLogger.Info("updating marketplaceconfig status")
				return r.Client.Status().Update(context.TODO(), marketplaceConfig)
			}

			return nil
		})

		return reconcile.Result{}, err
	} else { // get registration status for connected environment
		reqLogger.Info("Finding Cluster registration status")

		si, err := secretFetcher.ReturnSecret()
		if err != nil {
			return reconcile.Result{}, err
		}

		reqLogger.Info("found secret", "secret", si.Name)

		if si.Secret == nil {
			return reconcile.Result{}, nil
		}

		token, err := secretFetcher.ParseAndValidate(si)
		if err != nil {
			reqLogger.Error(err, "error validating secret")
			return reconcile.Result{}, err
		}

		tokenClaims, err := marketplace.GetJWTTokenClaim(token)
		if err != nil {
			reqLogger.Error(err, "error parsing token")
			return reconcile.Result{Requeue: true}, err
		}

		reqLogger.Info("attempting to update registration")
		marketplaceClient, err := marketplace.NewMarketplaceClientBuilder(r.cfg).
			NewMarketplaceClient(token, tokenClaims)

		if err != nil {
			reqLogger.Error(err, "error constructing marketplace client")
			return reconcile.Result{Requeue: true}, err
		}

		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
			if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, marketplaceConfig); err != nil {
				reqLogger.Error(err, "failed to get marketplaceconfig")
				return err
			}

			marketplaceClientAccount := &marketplace.MarketplaceClientAccount{
				AccountId:   marketplaceConfig.Spec.RhmAccountID,
				ClusterUuid: marketplaceConfig.Spec.ClusterUUID,
			}

			registrationStatusOutput, err := marketplaceClient.RegistrationStatus(marketplaceClientAccount)
			if err != nil {
				reqLogger.Error(err, "registration status failed")
				return err
			}

			reqLogger.Info("attempting to update registration", "status", registrationStatusOutput.RegistrationStatus)

			statusConditions := registrationStatusOutput.TransformConfigStatus()

			updated := false
			for _, cond := range statusConditions {
				updated = updated || marketplaceConfig.Status.Conditions.SetCondition(cond)
			}

			if updated {
				reqLogger.Info("updating marketplaceconfig status")
				return r.Client.Status().Update(context.TODO(), marketplaceConfig)
			}

			return nil
		})

		return reconcile.Result{}, err
	}
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
