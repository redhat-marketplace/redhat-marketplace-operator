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
	"time"
	"unicode/utf8"

	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
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

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	Client         client.Client
	Scheme         *runtime.Scheme
	Log            logr.Logger
	cc             ClientCommandRunner
	cfg            *config.OperatorConfig
	mclientBuilder *marketplace.MarketplaceClientBuilder
}

// Reconcile reads that state of the cluster for a MarketplaceConfig object and makes changes based on the state read
// and what is in the MarketplaceConfig.Spec
func (r *MarketplaceConfigReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MarketplaceConfig")
	cc := r.cc

	if r.mclientBuilder == nil {
		r.mclientBuilder = marketplace.NewMarketplaceClientBuilder(r.cfg)
	}

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

	// Update the OperatorGroup targetNamespace list
	// namespace list is from MarketPlaceConfig NamespaceLabelSelector or a default
	// In turn, OLM updates the olm.targetNamespaces annotation of
	// the member operator's ClusterServiceVersion (CSV) instances and is projected into their deployments.
	// The operatorGroupNamespace is guaranteed to be the same as the marketplaceConfig, unnecessary to use downwardAPI

	operatorGroupName, _ := getOperatorGroup()
	if len(operatorGroupName) != 0 {
		operatorGroup := &olmv1.OperatorGroup{}

		err = r.Client.Get(context.TODO(), types.NamespacedName{Name: operatorGroupName, Namespace: marketplaceConfig.Namespace}, operatorGroup)

		if err != nil && !k8serrors.IsNotFound(err) {
			return reconcile.Result{}, err
		} else if err == nil {
			operatorGroup.Spec.TargetNamespaces = []string{}
			operatorGroup.Spec.Selector = marketplaceConfig.Spec.NamespaceLabelSelector

			err = r.Client.Update(context.TODO(), operatorGroup)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
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

	//Fetch the Secret with name redhat-marketplace-pull-secret
	secret := v1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.RHMPullSecretName, Namespace: request.Namespace}, &secret)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			reqLogger.Error(err, "error finding", "name", utils.RHMPullSecretName)
			return reconcile.Result{}, nil
		}

		reqLogger.Error(err, "error fetching secret")
		return reconcile.Result{}, err
	}

	pullSecret, tokenIsValid := secret.Data[utils.RHMPullSecretKey]
	if !tokenIsValid {
		err := errors.New("rhm pull secret not found")
		reqLogger.Error(err, "couldn't find pull secret")
	}

	var updateInstanceSpec bool 
	if clusterDisplayName,ok := secret.Data[utils.ClusterDisplayNameKey]; ok {
		count := utf8.RuneCountInString(string(clusterDisplayName))
		clusterName := strings.Trim(string(clusterDisplayName),"\n")

		if !reflect.DeepEqual(marketplaceConfig.Spec.ClusterName,clusterName){
			if count <= 256 {
				marketplaceConfig.Spec.ClusterName = clusterName
				updateInstanceSpec = true
				reqLogger.Info("setting ClusterName","name", clusterName)
			} else {
				err := errors.New("CLUSTER_DISPLAY_NAME exceeds 256 chars")
				reqLogger.Error(err, "name",clusterDisplayName)
			}
		}
	} 

	token := string(pullSecret)
	tokenClaims, err := marketplace.GetJWTTokenClaim(token)
	if err != nil {
		tokenIsValid = false
		reqLogger.Error(err, "error parsing token")

	}

	if tokenIsValid {
		marketplaceClient, err := r.mclientBuilder.NewMarketplaceClient(token, tokenClaims)

		if err != nil {
			reqLogger.Error(err, "error constructing marketplace client")
			return reconcile.Result{Requeue: true}, nil
		}

		willBeDeleted := marketplaceConfig.GetDeletionTimestamp() != nil
		if willBeDeleted {
			result := r.unregister(marketplaceConfig, marketplaceClient, request, reqLogger)
			if !result.Is(Continue) {
				return result.Return()
			}
		}
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

	if marketplaceConfig.Labels == nil {
		marketplaceConfig.Labels = make(map[string]string)
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

	updatedRazee := foundRazee.DeepCopy()
	updatedRazee.Spec.ClusterUUID = marketplaceConfig.Spec.ClusterUUID
	updatedRazee.Spec.DeploySecretName = marketplaceConfig.Spec.DeploySecretName
	updatedRazee.Spec.Features = marketplaceConfig.Spec.Features.DeepCopy()

	if !reflect.DeepEqual(foundRazee, updatedRazee) {
		reqLogger.Info("updating razee cr")
		err = r.Client.Update(context.TODO(), updatedRazee)

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
			_ = r.Client.Status().Update(context.TODO(), marketplaceConfig)
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

	// Check if operator source exists, or create a new one
	foundOpSrc := &unstructured.Unstructured{}
	foundOpSrc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operators.coreos.com",
		Kind:    "OperatorSource",
		Version: "v1",
	})

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      utils.OPSRC_NAME,
		Namespace: utils.OPERATOR_MKTPLACE_NS},
		foundOpSrc)
	if err != nil && k8serrors.IsNotFound(err) {
		// Define a new operator source
		newOpSrc := utils.BuildNewOpSrc()
		reqLogger.Info("Creating a new opsource")
		err = r.Client.Create(context.TODO(), newOpSrc)
		if err != nil {
			reqLogger.Info("Failed to create an OperatorSource.", "OperatorSource.Namespace ", newOpSrc.GetNamespace(), "OperatorSource.Name", newOpSrc.GetName())
			return reconcile.Result{}, err
		}

		changed := marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionInstalling,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonOperatorSourceInstall,
			Message: "RHM Operator source installed.",
		})

		if changed {
			err = r.Client.Status().Update(context.TODO(), marketplaceConfig)

			if err != nil {
				reqLogger.Error(err, "failed to update status")
				return reconcile.Result{}, err
			}
		}

		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		// Could not get Operator Source
		reqLogger.Info("Failed to find OperatorSource")
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

	if tokenIsValid {
		reqLogger.Info("attempting to update registration")
		marketplaceClient, err := r.mclientBuilder.NewMarketplaceClient(token, tokenClaims)

		if err != nil {
			reqLogger.Error(err, "error constructing marketplace client")
			return reconcile.Result{Requeue: true}, nil
		}

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
		err = r.Client.Status().Update(context.TODO(), marketplaceConfig)
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

func (r *MarketplaceConfigReconciler) unregister(marketplaceConfig *marketplacev1alpha1.MarketplaceConfig, marketplaceClient *marketplace.MarketplaceClient, request reconcile.Request, reqLogger logr.Logger) *ExecResult {
	reqLogger.Info("attempting to un-register")

	marketplaceClientAccount := &marketplace.MarketplaceClientAccount{
		AccountId:   marketplaceConfig.Spec.RhmAccountID,
		ClusterUuid: marketplaceConfig.Spec.ClusterUUID,
	}

	reqLogger.Info("unregister", "marketplace client account", marketplaceClientAccount)

	registrationStatusOutput, err := marketplaceClient.UnRegister(marketplaceClientAccount)
	if err != nil {
		reqLogger.Error(err, "unregister failed")
		return &ExecResult{
			ReconcileResult: reconcile.Result{Requeue: true},
			Err:             nil,
		}
	}

	reqLogger.Info("unregister", "RegistrationStatus", registrationStatusOutput.RegistrationStatus)

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
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

func (m *MarketplaceConfigReconciler) InjectMarketplaceClientBuilder(mbuilder *marketplace.MarketplaceClientBuilder) error {
	m.mclientBuilder = mbuilder
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
