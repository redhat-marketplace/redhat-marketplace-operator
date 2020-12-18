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

package marketplaceconfig

import (
	"context"
	"reflect"
	"time"

	"github.com/gotidy/ptr"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	opsrcv1 "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	"github.com/operator-framework/operator-sdk/pkg/status"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/marketplace"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	pflag "github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	CSCFinalizer                    = "finalizer.MarketplaceConfigs.operators.coreos.com"
	DEFAULT_IMAGE_MARKETPLACE_AGENT = "marketplace-agent:latest"
	IBM_CATALOG_SOURCE_FLAG         = true
)

var (
	log                      = logf.Log.WithName("controller_marketplaceconfig")
	marketplaceConfigFlagSet *pflag.FlagSet
	generateMetricsFlag      = false
)

// Init declares our FlagSet for the MarketplaceConfig
// Currently only has 1 set of flags for setting the Image
func init() {
	marketplaceConfigFlagSet = pflag.NewFlagSet("marketplaceconfig", pflag.ExitOnError)
}

// FlagSet returns our FlagSet
func FlagSet() *pflag.FlagSet {
	return marketplaceConfigFlagSet
}

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new MarketplaceConfig Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(
	mgr manager.Manager,
	ccprovider ClientCommandRunnerProvider,
	cfg config.OperatorConfig,
) error {
	return add(mgr, newReconciler(mgr, ccprovider,cfg))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(
	mgr manager.Manager,
	ccprovider ClientCommandRunnerProvider,
	operatorCfg config.OperatorConfig,
) reconcile.Reconciler {
	return &ReconcileMarketplaceConfig{client: mgr.GetClient(), scheme: mgr.GetScheme(), ccprovider: ccprovider,cfg: operatorCfg}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("marketplaceconfig-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MarketplaceConfig
	err = c.Watch(&source.Kind{Type: &marketplacev1alpha1.MarketplaceConfig{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner MarketplaceConfig
	//
	ownerHandler := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MarketplaceConfig{},
	}

	err = c.Watch(&source.Kind{Type: &marketplacev1alpha1.RazeeDeployment{}}, ownerHandler)
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &marketplacev1alpha1.MeterBase{}}, ownerHandler)
	if err != nil {
		return err
	}

	// Watch for RazeeDeployment
	err = c.Watch(&source.Kind{Type: &marketplacev1alpha1.RazeeDeployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MarketplaceConfig{},
	})
	if err != nil {
		return err
	}

	// Watch for MeterBase
	err = c.Watch(&source.Kind{Type: &marketplacev1alpha1.MeterBase{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MarketplaceConfig{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileMarketplaceConfig implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMarketplaceConfig{}

// ReconcileMarketplaceConfig reconciles a MarketplaceConfig object
type ReconcileMarketplaceConfig struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	cfg 		config.OperatorConfig
	client     client.Client
	scheme     *runtime.Scheme
	ccprovider ClientCommandRunnerProvider
}

// Reconcile reads that state of the cluster for a MarketplaceConfig object and makes changes based on the state read
// and what is in the MarketplaceConfig.Spec
func (r *ReconcileMarketplaceConfig) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MarketplaceConfig")

	cc := r.ccprovider.NewCommandRunner(r.client, r.scheme, reqLogger)

	// Fetch the MarketplaceConfig instance
	marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
	err := r.client.Get(context.TODO(), request.NamespacedName, marketplaceConfig)
	if err != nil {
		if errors.IsNotFound(err) {
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

	// Removing EnabledMetering field so setting them all to nil
	// this will no longer do anything
	if marketplaceConfig.Spec.EnableMetering != nil {
		marketplaceConfig.Spec.EnableMetering = nil
	}

	//Initialize enabled features if not set
	if marketplaceConfig.Spec.Features == nil {
		marketplaceConfig.Spec.Features = &common.Features{
			Deployment:   ptr.Bool(true),
			Registration: ptr.Bool(true),
		}
	} else {
		if marketplaceConfig.Spec.Features.Deployment == nil {
			marketplaceConfig.Spec.Features.Deployment = ptr.Bool(true)
		}
		if marketplaceConfig.Spec.Features.Registration == nil {
			marketplaceConfig.Spec.Features.Registration = ptr.Bool(true)
		}
	}

	deployedNamespace := &corev1.Namespace{}
	err = r.client.Get(context.TODO(),types.NamespacedName{Name: r.cfg.ControllerValues.DeploymentNamespace},deployedNamespace)
	if err != nil {
		reqLogger.Error(err,"err getting deployed ns")
	}

	if deployedNamespace.Labels == nil {
		deployedNamespace.Labels = make(map[string]string)
	}

	if v, ok := deployedNamespace.Labels[utils.LicenseServerTag]; !ok || v != "true" {
		deployedNamespace.Labels[utils.LicenseServerTag] = "true"

		err = r.client.Update(context.TODO(), deployedNamespace)
		if err != nil {
			reqLogger.Error(err, "Failed to update deployed namespace with license server tag")
			return reconcile.Result{}, err
		}

		return reconcile.Result{Requeue: true}, nil
	}

	newRazeeCrd := utils.BuildRazeeCr(marketplaceConfig.Namespace, marketplaceConfig.Spec.ClusterUUID, marketplaceConfig.Spec.DeploySecretName, marketplaceConfig.Spec.Features)
	newMeterBaseCr := utils.BuildMeterBaseCr(marketplaceConfig.Namespace)
	// Add finalizer and execute it if the resource is deleted
	if result, _ := cc.Do(
		context.TODO(),
		Call(SetFinalizer(marketplaceConfig, utils.CONTROLLER_FINALIZER)),
		Call(
			RunFinalizer(marketplaceConfig, utils.CONTROLLER_FINALIZER,
				HandleResult(
					GetAction(
						types.NamespacedName{
							Namespace: newRazeeCrd.Namespace, Name: newRazeeCrd.Name}, newRazeeCrd),
					OnContinue(DeleteAction(newRazeeCrd))),
				HandleResult(
					GetAction(
						types.NamespacedName{
							Namespace: newMeterBaseCr.Namespace, Name: newMeterBaseCr.Name}, newMeterBaseCr),
					OnContinue(DeleteAction(newMeterBaseCr))),
			)),
	); !result.Is(Continue) {

		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to get MeterBase.")
		}

		if result.Is(Return) {
			reqLogger.Info("Delete is complete.")
		}

		return result.Return()
	}

	if marketplaceConfig.Annotations == nil {
		marketplaceConfig.Annotations = make(map[string]string)
	}

	if v, ok := marketplaceConfig.Annotations[utils.RazeeWatchResource]; !ok || v != utils.RazeeWatchLevelDetail {
		marketplaceConfig.Annotations[utils.RazeeWatchResource] = utils.RazeeWatchLevelDetail

		err = r.client.Update(context.TODO(), marketplaceConfig)

		if err != nil {
			reqLogger.Error(err, "Failed to create to updatee the marketplace config")
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
			err = r.client.Status().Update(context.TODO(), marketplaceConfig)

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
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: utils.RAZEE_NAME, Namespace: marketplaceConfig.Namespace}, foundRazee)
	if err != nil && errors.IsNotFound(err) {
		newRazeeCrd := utils.BuildRazeeCr(marketplaceConfig.Namespace, marketplaceConfig.Spec.ClusterUUID, marketplaceConfig.Spec.DeploySecretName, marketplaceConfig.Spec.Features)

		// Sets the owner for foundRazee
		if err = controllerutil.SetControllerReference(marketplaceConfig, newRazeeCrd, r.scheme); err != nil {
			reqLogger.Error(err, "Failed to create a new RazeeDeployment CR.")
			return reconcile.Result{}, err
		}

		reqLogger.Info("creating razee cr")
		err = r.client.Create(context.TODO(), newRazeeCrd)

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
			err = r.client.Status().Update(context.TODO(), marketplaceConfig)

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

	updatedRazee := foundRazee.DeepCopy()
	updatedRazee.Spec.ClusterUUID = marketplaceConfig.Spec.ClusterUUID
	updatedRazee.Spec.DeploySecretName = marketplaceConfig.Spec.DeploySecretName
	updatedRazee.Spec.Features = marketplaceConfig.Spec.Features.DeepCopy()

	if !reflect.DeepEqual(foundRazee, updatedRazee) {
		reqLogger.Info("updating razee cr")
		err = r.client.Update(context.TODO(), updatedRazee)

		if err != nil {
			reqLogger.Error(err, "Failed to create a new RazeeDeployment CR.")
			return reconcile.Result{}, err
		}

		ok := marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionInstalling,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonRazeeInstalled,
			Message: "RazeeDeployment updated.",
		})

		if ok {
			_ = r.client.Status().Update(context.TODO(), marketplaceConfig)
		}
		return reconcile.Result{Requeue: true}, nil
	}

	foundMeterBase := &marketplacev1alpha1.MeterBase{}
	result, _ := cc.Do(
		context.TODO(),
		GetAction(
			types.NamespacedName{Name: utils.METERBASE_NAME, Namespace: marketplaceConfig.Namespace},
			foundMeterBase,
		),
	)

	if result.Is(Error) {
		return result.Return()
	}

	reqLogger.Info("meterbase install info", "found", !result.Is(NotFound))

	reqLogger.Info("meterbase is enabled")
	// Check if MeterBase exists, if not create one
	if result.Is(NotFound) {
		newMeterBaseCr := utils.BuildMeterBaseCr(marketplaceConfig.Namespace)

		if err = controllerutil.SetControllerReference(marketplaceConfig, newMeterBaseCr, r.scheme); err != nil {
			reqLogger.Error(err, "Failed to set controller ref")
			return reconcile.Result{}, err
		}

		reqLogger.Info("creating meterbase")
		err = r.client.Create(context.TODO(), newMeterBaseCr)
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
			err = r.client.Status().Update(context.TODO(), marketplaceConfig)

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

	// Check if operator source exists, or create a new one
	foundOpSrc := &opsrcv1.OperatorSource{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      utils.OPSRC_NAME,
		Namespace: utils.OPERATOR_MKTPLACE_NS},
		foundOpSrc)
	if err != nil && errors.IsNotFound(err) {
		// Define a new operator source
		newOpSrc := utils.BuildNewOpSrc()
		reqLogger.Info("Creating a new opsource")
		err = r.client.Create(context.TODO(), newOpSrc)
		if err != nil {
			reqLogger.Info("Failed to create an OperatorSource.", "OperatorSource.Namespace ", newOpSrc.Namespace, "OperatorSource.Name", newOpSrc.Name)
			return reconcile.Result{}, err
		}

		changed := marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionInstalling,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonOperatorSourceInstall,
			Message: "RHM Operator source installed.",
		})

		if changed {
			err = r.client.Status().Update(context.TODO(), marketplaceConfig)

			if err != nil {
				reqLogger.Error(err, "failed to update status")
				return reconcile.Result{}, err
			}
		}

		// Operator Source created successfully - return and requeue
		newOpSrc.ForceUpdate()
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		// Could not get Operator Source
		reqLogger.Error(err, "Failed to get OperatorSource")
	}

	reqLogger.Info("Found opsource")

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
		marketplaceConfig.Status.RazeeSubConditions = &status.Conditions{}
	}

	if foundRazee != nil && foundRazee.Status.Conditions != nil {
		if !utils.ConditionsEqual(
			foundRazee.Status.Conditions,
			*marketplaceConfig.Status.RazeeSubConditions) {
			*marketplaceConfig.Status.RazeeSubConditions = foundRazee.Status.Conditions
			updated = updated || true
		}
	}

	if marketplaceConfig.Status.MeterBaseSubConditions == nil {
		marketplaceConfig.Status.MeterBaseSubConditions = &status.Conditions{}
	}

	if foundMeterBase != nil && foundMeterBase.Status.Conditions != nil {
		if !utils.ConditionsEqual(
			*foundMeterBase.Status.Conditions,
			*marketplaceConfig.Status.MeterBaseSubConditions) {
			*marketplaceConfig.Status.MeterBaseSubConditions = *foundMeterBase.Status.Conditions
			updated = updated || true
		}
	}

	reqLogger.Info("Finding Cluster registration status")
	//Fetch the Secret with name redhat-marketplace-pull-secret
	secret := v1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: utils.RHMPullSecretName, Namespace: request.Namespace}, &secret)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, "error finding", "name", utils.RHMPullSecretName)
			return reconcile.Result{}, nil
		}

		reqLogger.Error(err, "error fetching secret")
		return reconcile.Result{}, err
	}
	//Setting MarketplaceClientAccount
	pullSecret, ok := secret.Data[utils.RHMPullSecretKey]

	if !ok {
		reqLogger.Error(err, "secret is missing appropriate field and can't check status")
	}

	if ok {
		reqLogger.Info("attempting to update registration")
		marketplaceClient, err := marketplace.NewMarketplaceClient(&marketplace.MarketplaceClientConfig{
			Url:      r.cfg.Marketplace.URL,
			Token:    string(pullSecret),
			Insecure: r.cfg.Marketplace.InsecureClient,
		})

		marketplaceClientAccount := &marketplace.MarketplaceClientAccount{
			AccountId:   marketplaceConfig.Spec.RhmAccountID,
			ClusterUuid: marketplaceConfig.Spec.ClusterUUID,
		}

		registrationStatusOutput, err := marketplaceClient.RegistrationStatus(marketplaceClientAccount)

		if err != nil {
			reqLogger.Error(err, "registration status failed")
			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.Info("attempting to update registration", "status", registrationStatusOutput.RegistrationStatus)

		statusConditions := registrationStatusOutput.TransformConfigStatus()

		for _, cond := range statusConditions {
			updated = updated || marketplaceConfig.Status.Conditions.SetCondition(cond)
		}
	}

	if updated {
		//Updating Marketplace Config with Cluster Registration status
		err = r.client.Status().Update(context.TODO(), marketplaceConfig)
		if err != nil {
			reqLogger.Error(err, "Failed to update status")
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	reqLogger.Info("reconciling finished")
	return reconcile.Result{RequeueAfter: time.Second * 30}, nil
}

// labelsForMarketplaceConfig returs the labels for selecting the resources
// belonging to the given marketplaceConfig custom resource name
func labelsForMarketplaceConfig(name string) map[string]string {
	return map[string]string{"app": "marketplaceconfig", "marketplaceconfig_cr": name}
}

// Begin installation or deletion of Catalog Source
func (r *ReconcileMarketplaceConfig) createCatalogSource(request reconcile.Request, marketplaceConfig *marketplacev1alpha1.MarketplaceConfig, catalogName string) (bool, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name, "CatalogSource.Name", catalogName)

	// Get installation setting for Catalog Source (checks MarketplaceConfig.Spec if it doesn't exist, use flag)
	installCatalogSrcP := marketplaceConfig.Spec.InstallIBMCatalogSource
	var installCatalogSrc bool

	if installCatalogSrcP == nil {

		reqLogger.Info("MarketplaceConfig.Spec.InstallIBMCatalogSource not found. Using flag.")
		installCatalogSrc = r.cfg.Features.IBMCatalog

		marketplaceConfig.Spec.InstallIBMCatalogSource = &installCatalogSrc
		r.client.Update(context.TODO(), marketplaceConfig)
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
	err := r.client.Get(context.TODO(), catalogSrcNamespacedName, catalogSrc)

	// If installCatalogSrc is true: install Catalog Source
	// if installCatalogSrc is false: do not install Catalog Source, and delete existing one (if it exists)
	reqLogger.Info("Checking Install Catalog Src", "InstallCatalogSource: ", installCatalogSrc)
	if installCatalogSrc {
		// If the Catalog Source does not exist, create one
		if err != nil && errors.IsNotFound(err) {
			// Create catalog source
			var newCatalogSrc *operatorsv1alpha1.CatalogSource
			if utils.IBM_CATALOGSRC_NAME == catalogName {
				newCatalogSrc = utils.BuildNewIBMCatalogSrc()
			} else { // utils.OPENCLOUD_CATALOGSRC_NAME
				newCatalogSrc = utils.BuildNewOpencloudCatalogSrc()
			}

			reqLogger.Info("Creating catalog source")
			err = r.client.Create(context.TODO(), newCatalogSrc)
			if err != nil {
				reqLogger.Info("Failed to create a CatalogSource.", "CatalogSource.Namespace ", newCatalogSrc.Namespace, "CatalogSource.Name", newCatalogSrc.Name)
				return false, err
			}

			ok := marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonCatalogSourceInstall,
				Message: catalogName + " catalog source installed.",
			})

			if ok {
				err = r.client.Status().Update(context.TODO(), marketplaceConfig)

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
			err = r.client.Delete(context.TODO(), catalogSrc, client.PropagationPolicy(metav1.DeletePropagationBackground))
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
				err = r.client.Status().Update(context.TODO(), marketplaceConfig)
				if err != nil {
					reqLogger.Error(err, "failed to update status")
					return false, err
				}
			}

			// catalog source deleted successfully - return and requeue
			return true, nil
		} else if err != nil && !errors.IsNotFound(err) {
			// Could not get catalog source
			reqLogger.Error(err, "Failed to get CatalogSource", "CatalogSource.Namespace ", catalogSrcNamespacedName.Namespace, "CatalogSource.Name", catalogSrcNamespacedName.Name)
			return false, err
		}

		reqLogger.Info(catalogName + " catalog Source does not exist.")

	}
	return false, nil
}
