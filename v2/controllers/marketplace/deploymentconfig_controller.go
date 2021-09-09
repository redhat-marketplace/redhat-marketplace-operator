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
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	osappsv1 "github.com/openshift/api/apps/v1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/matcher"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	osimagev1 "github.com/openshift/api/image/v1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/catalog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// blank assignment to verify that DeploymentConfigReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &DeploymentConfigReconciler{}

// DeploymentConfigReconciler reconciles the DataService of a MeterBase object
type DeploymentConfigReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client        client.Client
	Scheme        *runtime.Scheme
	Log           logr.Logger
	CC            ClientCommandRunner
	cfg           *config.OperatorConfig
	factory       *manifests.Factory
	patcher       patch.Patcher
	CatalogClient *catalog.CatalogClient
}

func (r *DeploymentConfigReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (r *DeploymentConfigReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	r.cfg = cfg
	return nil
}

func (r *DeploymentConfigReconciler) InjectCommandRunner(ccp ClientCommandRunner) error {
	r.Log.Info("command runner")
	r.CC = ccp
	return nil
}

func (r *DeploymentConfigReconciler) InjectCatalogClient(catalogClient *catalog.CatalogClient) error {
	r.Log.Info("catalog client")
	r.CatalogClient = catalogClient
	return nil
}

func (r *DeploymentConfigReconciler) InjectPatch(p patch.Patcher) error {
	r.patcher = p
	return nil
}

func (r *DeploymentConfigReconciler) InjectFactory(f *manifests.Factory) error {
	r.factory = f
	return nil
}

// adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *DeploymentConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Log.Info("SetupWithManager DeploymentConfigReconciler")

	nsPred := predicates.NamespacePredicate(r.cfg.DeployedNamespace)


	meterBaseSubSectionPred := []predicate.Predicate{
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				meterbaseOld, ok := e.ObjectOld.(*marketplacev1alpha1.MeterBase)
				if !ok {
					return false
				}

				meterbaseNew, ok := e.ObjectNew.(*marketplacev1alpha1.MeterBase)
				if !ok {
					return false
				}

				return meterbaseOld.Spec.MeterdefinitionCatalogServer != meterbaseNew.Spec.MeterdefinitionCatalogServer
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return true
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return true
			},
		},
	}

	deploymentConfigPred := []predicate.Predicate{
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return e.Meta.GetName() == utils.DEPLOYMENT_CONFIG_NAME
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return e.MetaNew.GetName() == utils.DEPLOYMENT_CONFIG_NAME
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return e.Meta.GetName() == utils.DEPLOYMENT_CONFIG_NAME
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return e.Meta.GetName() == utils.DEPLOYMENT_CONFIG_NAME
			},
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(nsPred).
		For(&marketplacev1alpha1.MeterBase{}).
		Watches(
			&source.Kind{Type: &marketplacev1alpha1.MeterBase{}},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(meterBaseSubSectionPred...)).
		Watches(
			&source.Kind{Type: &osappsv1.DeploymentConfig{}},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(deploymentConfigPred...)).
		Watches(
			&source.Kind{Type: &osimagev1.ImageStream{}},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(deploymentConfigPred...)).
		Complete(r)
}

// +kubebuilder:rbac:groups=apps.openshift.io,resources=deploymentconfigs,verbs=get;create;list;update;watch;delete
// +kubebuilder:rbac:groups=image.openshift.io,resources=imagestreams,verbs=get;create;update;list;watch;delete
// +kubebuilder:rbac:urls=/list-for-version/*,verbs=get;
// +kubebuilder:rbac:urls=/get-system-meterdefs/*,verbs=get;post;create;
// +kubebuilder:rbac:urls=/meterdef-index-label/*,verbs=get;
// +kubebuilder:rbac:urls=/system-meterdef-index-label/*,verbs=get;
// +kubebuilder:rbac:groups="authentication.k8s.io",resources=tokenreviews,verbs=create;get
// +kubebuilder:rbac:groups="authorization.k8s.io",resources=subjectaccessreviews,verbs=create;get

// Reconcile reads that state of the cluster for a DeploymentConfig object and makes changes based on the state read
// and what is in the MeterdefConfigmap.Spec
func (r *DeploymentConfigReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	instance := &marketplacev1alpha1.MeterBase{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.METERBASE_NAME,Namespace: request.Namespace}, instance)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			reqLogger.Error(err, "meterbase does not exist must have been deleted - ignoring for now")
			return reconcile.Result{}, nil
		}

		reqLogger.Error(err, "Failed to get meterbase")
		return reconcile.Result{}, err
	}

	if instance.Spec.MeterdefinitionCatalogServer == nil {
		reqLogger.Info("meterbase doesn't have file server feature flags set")
		return reconcile.Result{},nil
	}

	err = r.CatalogClient.CheckAuth(reqLogger)
	if err != nil {
		return reconcile.Result{},err
	}

	// if LicenceUsageMeteringEnabled is disabled delete all system meterdefs for csvs originating from rhm
	if !instance.Spec.MeterdefinitionCatalogServer.LicenceUsageMeteringEnabled {
		reqLogger.Info("license usage metering disabled, uninstalling system meterdefs")
		result := r.deleteAllSystemMeterDefsForRhmCvs(reqLogger)
		if !result.Is(Continue){
			reqLogger.Info(result.Error())
			result.Return()
		}
	}

	// catalog server not enabled, stop uninstall and stop reconciling
	if !instance.Spec.MeterdefinitionCatalogServer.MeterdefinitionCatalogServerEnabled {
		result := r.uninstallFileServerResources(reqLogger)
		if !result.Is(Continue){
			result.Return()
		}

		reqLogger.Info("done uninstalling catalog server resources")
		return reconcile.Result{},nil
	}

	result := r.reconcileMeterdefCatalogServerResources(instance, request, reqLogger)
	if !result.Is(Continue) {
		return result.Return()
	}

	// get the latest deploymentconfig
	dc := &osappsv1.DeploymentConfig{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.DEPLOYMENT_CONFIG_NAME, Namespace: r.cfg.DeployedNamespace}, dc)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			reqLogger.Info("deployment config not found, ignoring")
			return reconcile.Result{}, nil
		}

		reqLogger.Error(err, "Failed to get deploymentconfig")
		return reconcile.Result{}, err
	}

	for _, c := range dc.Status.Conditions {
		if c.Type == osappsv1.DeploymentAvailable {
			if c.Status != corev1.ConditionTrue {
				return reconcile.Result{RequeueAfter: time.Minute * 1}, err
			}
		}
	}

	// catch the deploymentconfig as it's rolling out a new deployment and requeue until finished
	for _, c := range dc.Status.Conditions {
		if c.Type == osappsv1.DeploymentProgressing {
			if c.Reason != "NewReplicationControllerAvailable" || c.Status != corev1.ConditionTrue || dc.Status.LatestVersion == dc.Status.ObservedGeneration {
				reqLogger.Info("deploymentconfig has not finished rollout, requeueing")
				return reconcile.Result{RequeueAfter: time.Minute * 2}, err
			}
		}
	}

	reqLogger.Info("deploymentconfig is in ready state")
	latestVersion := dc.Status.LatestVersion

	result = r.pruneDeployPods(latestVersion, request, reqLogger)
	if !result.Is(Continue) {
		return result.Return()
	}

	//syncs the latest meterdefinitions from the catalog with the community meterdefinitions on the cluster
	result = r.sync(instance,request, reqLogger)
	if !result.Is(Continue) {
		return result.Return()
	}

	reqLogger.Info("finished reconciling")
	return reconcile.Result{}, nil
}

func (r *DeploymentConfigReconciler) sync(instance *marketplacev1alpha1.MeterBase,request reconcile.Request, reqLogger logr.Logger) *ExecResult {
	
	csvList := &olmv1alpha1.ClusterServiceVersionList{}

	err := r.Client.List(context.TODO(), csvList)
	if err != nil {
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	for _, csv := range csvList.Items {
	
		subs, err := listSubs(r.Client,&csv)
		if err != nil {
			return &ExecResult{
				ReconcileResult: reconcile.Result{},
				Err:             err,
			}
		}
	
		packageName,err := matcher.ParsePackageName(&csv)
		if err != nil {
			reqLogger.Info(err.Error())
		}
	
		foundSub,err := matcher.MatchCsvToSub(r.cfg.ControllerValues.RhmCatalogName,packageName,subs,&csv)
		if err != nil {
			reqLogger.Info(err.Error())
		}

		isRhmCsv := matcher.CheckOperatorTag(foundSub)
		if !isRhmCsv {
			reqLogger.Info("csv is not an rhm resource", "csv", csv.Name)
			continue
		}

		if instance.Spec.MeterdefinitionCatalogServer.LicenceUsageMeteringEnabled {
			result := r.syncSystemMeterDefs(csv,reqLogger)
			if !result.Is(Continue){
				result.Return()
			}
		}

		result := r.syncCommunityMeterDefs(csv,reqLogger)
		if !result.Is(Continue){
			result.Return()
		}
	}

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

var listSubs = func (k8sclient client.Client,csv *olmv1alpha1.ClusterServiceVersion) ([]olmv1alpha1.Subscription,error) {
	subList := &olmv1alpha1.SubscriptionList{}
	if err := k8sclient.List(context.TODO(), subList, client.InNamespace(csv.Namespace)); err != nil {
		return nil,err
	}

	return subList.Items,nil
}

func (r *DeploymentConfigReconciler) syncSystemMeterDefs(csv olmv1alpha1.ClusterServiceVersion,reqLogger logr.Logger) *ExecResult {
	
	systemMeterDefs, err := r.CatalogClient.GetSystemMeterdefs(&csv, reqLogger)
	if err != nil {
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	result := r.createOrUpdate(systemMeterDefs, csv, reqLogger)
	if !result.Is(Continue) {
		return result
	}

	systemMeterDefIndexLabels, err := r.CatalogClient.GetSystemMeterDefIndexLabels(reqLogger, csv.Name)
	if err != nil {	
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	systemMeterDefsOnCluster, result := r.listAllMeterDefsForCsv(systemMeterDefIndexLabels)
	if !result.Is(Continue) {
		return result
	}

	result = r.deleteOnDiff(systemMeterDefsOnCluster.Items, systemMeterDefs, reqLogger)
	if !result.Is(Continue) {
		return result
	}
	
	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func (r *DeploymentConfigReconciler) syncCommunityMeterDefs(csv olmv1alpha1.ClusterServiceVersion,reqLogger logr.Logger) *ExecResult {
	/*
		csv.Name = memcached-operator.v0.0.1
		splitName = memcached-operator
	*/
	splitName := strings.Split(csv.Name, ".")[0]
	csvVersion := csv.Spec.Version.Version.String()
	csvNamespace := csv.Namespace

	/*
		pings the file server for a map of labels we use to index meterdefintions that originated from the file server
		these labels also get added to a meterdefinition by the file server	before it returns
		split-name gets dynamically added on the call to get labels - could probably just use the split name in the controller however
		{
			"marketplace.redhat.com/installedOperatorNameTag": "<split-name>",
			"marketplace.redhat.com/isCommunityMeterdefintion": "true"
		}

	*/
	communityIndexLabels, err := r.CatalogClient.GetMeterdefIndexLabels(reqLogger, csv.Name)
	if err != nil {
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	/*
		csv is on the cluster but doesn't have a csv dir or doesn't have mdefs in it's catalog listing
		if an isv removes their catalog listing, meterdefs could be orphaned on the cluster
		delete all community meterdefs for that csv
		if no community meterdefs are found, skip to next csv
		if community meterdefs are found and deleted, skip to next csv
	*/
	latestCommunityMeterDefsFromCatalog, err := r.CatalogClient.ListMeterdefintionsFromFileServer(splitName, csvVersion, csvNamespace, reqLogger)
	if err != nil {
		if errors.Is(err, catalog.CatalogNoContentErr) {
			reqLogger.Info("csv has no meterdefinitions in catalog", "csv", csv.Name)

			result := r.deleteMeterdefsForCsv(communityIndexLabels, reqLogger)
			if result.Is(Continue) {
				reqLogger.Info("skipping sync for csv", "csv", csv.Name)
				result.Return()

			} else if !result.Is(Continue) {
				return result
			}
		}

		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	result := r.createOrUpdate(latestCommunityMeterDefsFromCatalog, csv, reqLogger)
	if !result.Is(Continue) {
		if result.Is(Error){
			reqLogger.Info(result.Error())
		}
		return result
	}

	/*
		delete if there is a meterdef installed on the cluster that originated from the catalog, but that meterdef isn't in the latest file server image
	*/
	catalogMdefsOnCluster, result := r.listAllMeterDefsForCsv(communityIndexLabels)
	if !result.Is(Continue) {
		return result
	}

	result = r.deleteOnDiff(catalogMdefsOnCluster.Items, latestCommunityMeterDefsFromCatalog, reqLogger)
	if !result.Is(Continue) {
		return result
	}

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func (r *DeploymentConfigReconciler) createOrUpdate(latestMeterDefsFromCatalog []marketplacev1beta1.MeterDefinition, csv olmv1alpha1.ClusterServiceVersion, reqLogger logr.Logger) *ExecResult {
	for _, catalogMeterdef := range latestMeterDefsFromCatalog {
		
		installedMdef := &marketplacev1beta1.MeterDefinition{}

		reqLogger.Info("finding meterdefintion", catalogMeterdef.Name, catalogMeterdef.Namespace)

		err := r.Client.Get(context.TODO(), types.NamespacedName{Name: catalogMeterdef.Name, Namespace: catalogMeterdef.Namespace}, installedMdef)
		if err != nil && k8serrors.IsNotFound(err) {
			reqLogger.Info("meterdef not found during sync, creating", "name", catalogMeterdef.Name, "namespace", catalogMeterdef.Namespace)
			/*
				create a meterdef for a csv if the csv has a meterdefinition listed in the catalog
				&& that meterdef is not on the cluster
			*/
			result := r.createMeterdefWithOwnerRef(catalogMeterdef, &csv, reqLogger)
			if !result.Is(Continue) {
				return result
			}
		} else if err != nil {
			return &ExecResult{
				ReconcileResult: reconcile.Result{},
				Err:             err,
			}
		} else {
			/*
				update a meterdef for a csv if a meterdef from the catalog is also on the cluster
				&& the meterdef from the catalog contains an update to .Spec or .Annotations
				//TODO: what fields should we check a diff for ?
			*/
			result := r.updateMeterdef(installedMdef, catalogMeterdef, reqLogger)
			if !result.Is(Continue) {
				return result
			}
		}
	}

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

/* 
	seems like removing owner refs is enough to remove these resources, but doing a pass through with a deletion to be sure
*/
func(r *DeploymentConfigReconciler) uninstallFileServerResources(reqLogger logr.Logger) (result *ExecResult) {

	result = r.uninstallDeploymentConfig(reqLogger)
	if !result.Is(Continue) {
		result.Return()
	}

	result = r.uninstallService(reqLogger)
	if !result.Is(Continue) {
		result.Return()
	}

	result = r.uninstallImageStream(reqLogger)
	if !result.Is(Continue) {
		result.Return()
	}

	return result
}

func(r *DeploymentConfigReconciler) uninstallDeploymentConfig(reqLogger logr.Logger) *ExecResult {
	foundDeploymentConfig := &osappsv1.DeploymentConfig{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.DEPLOYMENT_CONFIG_NAME, Namespace: r.cfg.DeployedNamespace}, foundDeploymentConfig)	
	if err != nil {
		if k8serrors.IsNotFound(err){
			reqLogger.Info("deploymentconfig not found,skipping uninstall")
			return &ExecResult{
				Status: ActionResultStatus(Continue),
			}
		}

		reqLogger.Error(err, "could not uninstall deploymentconfig")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	foundDeploymentConfig.OwnerReferences = []metav1.OwnerReference{}
	err = r.Client.Delete(context.TODO(),foundDeploymentConfig)
	if err != nil && !k8serrors.IsNotFound(err){
		reqLogger.Error(err, "could not uninstall deploymentconfig")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func(r *DeploymentConfigReconciler) uninstallService(reqLogger logr.Logger) *ExecResult {
	foundFileServerService := &corev1.Service{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.DEPLOYMENT_CONFIG_NAME, Namespace: r.cfg.DeployedNamespace}, foundFileServerService)
	if err != nil {
		if k8serrors.IsNotFound(err){
			reqLogger.Info("catalog server service not found,skipping uninstall")
			return &ExecResult{
				Status: ActionResultStatus(Continue),
			}
		}

		reqLogger.Error(err, "could not uninstall catalog server service")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	foundFileServerService.OwnerReferences = []metav1.OwnerReference{}
	err = r.Client.Delete(context.TODO(),foundFileServerService)
	if err != nil && !k8serrors.IsNotFound(err){
		reqLogger.Error(err, "could not uninstall catalog server service")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func(r *DeploymentConfigReconciler) uninstallImageStream(reqLogger logr.Logger) *ExecResult {
	foundImageStream := &osimagev1.ImageStream{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.DEPLOYMENT_CONFIG_NAME, Namespace: r.cfg.DeployedNamespace}, foundImageStream)
	if err != nil {
		if k8serrors.IsNotFound(err){
			reqLogger.Info("image stream not found,skipping uninstall")
			return &ExecResult{
				Status: ActionResultStatus(Continue),
			}
		}

		reqLogger.Error(err, "could not uninstall image stream")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	foundImageStream.OwnerReferences = []metav1.OwnerReference{}
	err = r.Client.Delete(context.TODO(),foundImageStream)
	if err != nil && !k8serrors.IsNotFound(err){
		reqLogger.Error(err, "could not uninstall image stream")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func (r *DeploymentConfigReconciler) reconcileMeterdefCatalogServerResources(instance *marketplacev1alpha1.MeterBase,request reconcile.Request, reqLogger logr.Logger) *ExecResult {
	gvk, err := apiutil.GVKForObject(instance, r.Scheme)
	if err != nil {
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	// create owner ref object
	ref := metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               instance.GetName(),
		UID:                instance.GetUID(),
		BlockOwnerDeletion: pointer.BoolPtr(false),
		Controller:         pointer.BoolPtr(false),
	}
	
	foundDeploymentConfig := &osappsv1.DeploymentConfig{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.DEPLOYMENT_CONFIG_NAME, Namespace: r.cfg.DeployedNamespace}, foundDeploymentConfig)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			reqLogger.Info("meterdef file server deployment config not found, creating")

			newDeploymentConfig, err := r.factory.NewMeterdefintionFileServerDeploymentConfig()
			if err != nil {
				return &ExecResult{
					ReconcileResult: reconcile.Result{},
					Err:             err,
				}
			}

			newDeploymentConfig.ObjectMeta.SetOwnerReferences([]metav1.OwnerReference{ref})

			err = r.Client.Create(context.TODO(), newDeploymentConfig)
			if err != nil {
				reqLogger.Error(err, "failed to create deploymentconfig")
				return &ExecResult{
					ReconcileResult: reconcile.Result{},
					Err:             err,
				}
			}

			reqLogger.Info("created new deploymentconfig")

			return &ExecResult{
				ReconcileResult: reconcile.Result{Requeue: true},
				Err:             nil,
			}
		}

		reqLogger.Error(err, "Failed to get meterdef file server deploymentconfig")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}

	}

	updated := r.factory.UpdateDeploymentConfigOnChange(foundDeploymentConfig)
	if updated {
		err = r.Client.Update(context.TODO(), foundDeploymentConfig)
		if err != nil {
			reqLogger.Error(err, "Failed to update file server deploymentconfig")
			return &ExecResult{
				ReconcileResult: reconcile.Result{},
				Err:             err,
			}
		}

		reqLogger.Info("updated deploymentconfig")

		return &ExecResult{
			ReconcileResult: reconcile.Result{Requeue: true},
			Err:             nil,
		}
	}

	foundfileServerService := &corev1.Service{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.DEPLOYMENT_CONFIG_NAME, Namespace: r.cfg.DeployedNamespace}, foundfileServerService)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			reqLogger.Info("meterdef file server service not found, creating")

			newService, err := r.factory.NewMeterdefintionFileServerService()
			if err != nil {
				return &ExecResult{
					ReconcileResult: reconcile.Result{},
					Err:             err,
				}
			}

			newService.ObjectMeta.SetOwnerReferences([]metav1.OwnerReference{ref})

			err = r.Client.Create(context.TODO(), newService)
			if err != nil {
				reqLogger.Error(err, "failed to create file server service")
				return &ExecResult{
					ReconcileResult: reconcile.Result{},
					Err:             err,
				}
			}

			reqLogger.Info("created new catalog server service")
			return &ExecResult{
				ReconcileResult: reconcile.Result{Requeue: true},
				Err:             nil,
			}
		}

		reqLogger.Error(err, "Failed to get meterdef file server service")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	foundImageStream := &osimagev1.ImageStream{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.DEPLOYMENT_CONFIG_NAME, Namespace: r.cfg.DeployedNamespace}, foundImageStream)
	if err != nil {
		if k8serrors.IsNotFound(err) {

			reqLogger.Info("image stream not found, creating")

			newImageStream, err := r.factory.NewMeterdefintionFileServerImageStream()
			if err != nil {
				return &ExecResult{
					ReconcileResult: reconcile.Result{},
					Err:             err,
				}
			}

			newImageStream.ObjectMeta.SetOwnerReferences([]metav1.OwnerReference{ref})

			err = r.Client.Create(context.TODO(), newImageStream)
			if err != nil {
				reqLogger.Error(err, "failed to create image stream")
				return &ExecResult{
					ReconcileResult: reconcile.Result{},
					Err:             err,
				}
			}

			reqLogger.Info("created new image stream")

			return &ExecResult{
				ReconcileResult: reconcile.Result{Requeue: true},
				Err:             nil,
			}
		}

		reqLogger.Error(err, "Failed to get image stream")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	imageStreamUpdated := r.factory.UpdateImageStreamOnChange(foundImageStream)
	if imageStreamUpdated {
		err = r.Client.Update(context.TODO(), foundImageStream)
		if err != nil {
			reqLogger.Error(err, "Failed to update image stream")
			return &ExecResult{
				ReconcileResult: reconcile.Result{Requeue: true},
				Err:             err,
			}
		}

		reqLogger.Info("updated ImageStream")

		return &ExecResult{
			ReconcileResult: reconcile.Result{Requeue: true},
			Err:             nil,
		}
	}

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}

}

func (r *DeploymentConfigReconciler) updateMeterdef(installedMdef *marketplacev1beta1.MeterDefinition, catalogMdef marketplacev1beta1.MeterDefinition, reqLogger logr.Logger) *ExecResult {
	
	updatedMeterdefinition := installedMdef.DeepCopy()
	updatedMeterdefinition.Spec = catalogMdef.Spec
	updatedMeterdefinition.ObjectMeta.Annotations = catalogMdef.ObjectMeta.Annotations

	if !reflect.DeepEqual(updatedMeterdefinition, installedMdef) {
		reqLogger.Info("meterdefintion is out of sync with latest meterdef catalog", "Name", installedMdef.Name)
		err := r.Client.Update(context.TODO(), updatedMeterdefinition)
		if err != nil {
			reqLogger.Error(err, "Failed updating meter definition", "Name", updatedMeterdefinition.Name, "Namespace", updatedMeterdefinition.Namespace)
			return &ExecResult{
				ReconcileResult: reconcile.Result{},
				Err:             err,
			}
		}
		reqLogger.Info("Updated meterdefintion", "Name", updatedMeterdefinition.Name, "Namespace", updatedMeterdefinition.Namespace)

		return &ExecResult{
			ReconcileResult: reconcile.Result{Requeue: true},
			Err:             nil,
		}
	}

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func (r *DeploymentConfigReconciler) createMeterdefWithOwnerRef(meterDefinition marketplacev1beta1.MeterDefinition, csv *olmv1alpha1.ClusterServiceVersion, reqLogger logr.InfoLogger) *ExecResult {

	gvk, err := apiutil.GVKForObject(csv, r.Scheme)
	if err != nil {
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	// create owner ref object
	ref := metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               csv.GetName(),
		UID:                csv.GetUID(),
		BlockOwnerDeletion: pointer.BoolPtr(false),
		Controller:         pointer.BoolPtr(false),
	}

	meterDefinition.ObjectMeta.SetOwnerReferences([]metav1.OwnerReference{ref})
	meterDefinition.ObjectMeta.Namespace = csv.Namespace

	err = r.Client.Create(context.TODO(), &meterDefinition)
	if err != nil {
		reqLogger.Error(err, "Could not create meterdefinition", "mdef", meterDefinition.Name, "CSV", csv.Name)
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	reqLogger.Info("Created meterdefinition", "mdef", meterDefinition.Name, "CSV", csv.Name)

	return &ExecResult{
		ReconcileResult: reconcile.Result{Requeue: true},
		Err:             nil,
	}
}

func (r *DeploymentConfigReconciler) deleteOnDiff(catalogMdefsOnCluster []marketplacev1beta1.MeterDefinition, latestMeterdefsFromCatalog []marketplacev1beta1.MeterDefinition, reqLogger logr.Logger) *ExecResult {
	reqLogger.Info("delete on diff")
	deleteList := utils.FindMeterdefDiff(catalogMdefsOnCluster, latestMeterdefsFromCatalog)
	if len(deleteList) != 0 {
		for _, mdef := range deleteList {
			reqLogger.Info("meterdef has been selected for deletion", "meterdef", mdef.Name)

			result := r.deleteMeterDef(mdef.Name, mdef.Namespace, reqLogger)
			if !result.Is(Continue) {
				return result
			}
		}
	}

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func (r *DeploymentConfigReconciler) listAllMeterDefsForCsv(indexLabels map[string]string) (*marketplacev1beta1.MeterDefinitionList, *ExecResult) {
	
	installedMeterdefList := &marketplacev1beta1.MeterDefinitionList{}

	// look for meterdefs that originated from the meterdefinition catalog
	listOpts := []client.ListOption{
		client.MatchingLabels(indexLabels),
	}

	err := r.Client.List(context.TODO(), installedMeterdefList, listOpts...)
	if err != nil {
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	return installedMeterdefList, &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func (r *DeploymentConfigReconciler) deleteAllSystemMeterDefsForRhmCvs(reqLogger logr.Logger) *ExecResult {

	csvList := &olmv1alpha1.ClusterServiceVersionList{}
	err := r.Client.List(context.TODO(), csvList)
	if err != nil {
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	for _, csv := range csvList.Items {
		subs, err := listSubs(r.Client,&csv)
		if err != nil {
			return &ExecResult{
				ReconcileResult: reconcile.Result{},
				Err:             err,
			}
		}

		packageName,err := matcher.ParsePackageName(&csv)
		if err != nil {
			reqLogger.Info(err.Error())
		}
	
		foundSub,err := matcher.MatchCsvToSub(r.cfg.ControllerValues.RhmCatalogName,packageName,subs,&csv)
		if err != nil {
			reqLogger.Info(err.Error())
		}

		isRhmCsv := matcher.CheckOperatorTag(foundSub)
		if !isRhmCsv {
			reqLogger.Info("csv is not an rhm resource", "csv", csv.Name)
			continue
		}

		systemMeterDefIndexLabels, err := r.CatalogClient.GetSystemMeterDefIndexLabels(reqLogger, csv.Name)
		if err != nil {	
			return &ExecResult{
				ReconcileResult: reconcile.Result{},
				Err:             err,
			}
		}

		result := r.deleteMeterdefsForCsv(systemMeterDefIndexLabels, reqLogger)
		if result.Is(Noop){
			reqLogger.Info("skipping deletion of system meterdefs", "csv", csv.Name)
			continue
		}

		if !result.Is(Continue) && !result.Is(Noop){
			return result
		}
	}

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func (r *DeploymentConfigReconciler) deleteMeterdefsForCsv(indexLabels map[string]string, reqLogger logr.Logger) *ExecResult {
	reqLogger.Info("deleting meterdefinitions with index", "index", indexLabels)

	installedMeterdefList := &marketplacev1beta1.MeterDefinitionList{}

	// look for meterdefs that are from the meterdefinition catalog
	listOpts := []client.ListOption{
		client.MatchingLabels(indexLabels),
	}

	err := r.Client.List(context.TODO(), installedMeterdefList, listOpts...)
	if err != nil {
		reqLogger.Info("client list error", "err", err.Error())
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	for _, mdef := range installedMeterdefList.Items {
		result := r.deleteMeterDef(mdef.Name, mdef.Namespace, reqLogger)
		if !result.Is(Continue) {
			return result
		}
	}

	if len(installedMeterdefList.Items) == 0 {
		reqLogger.Info("no meterdefinitions found on cluster for csv with index", "index", indexLabels)
		return &ExecResult{
			Status: ActionResultStatus(Noop),
		}
	}

	for _, mdef := range installedMeterdefList.Items {
		reqLogger.Info("deleting meterdefintion", "name", mdef.Name)
		result := r.deleteMeterDef(mdef.Name, mdef.Namespace, reqLogger)
		if !result.Is(Continue) {
			return result
		}
	}

	return &ExecResult{
		Status: ActionResultStatus(Noop),
	}
}

func (r *DeploymentConfigReconciler) deleteMeterDef(mdefName string, namespace string, reqLogger logr.Logger) *ExecResult {

	installedMeterDefn := &marketplacev1beta1.MeterDefinition{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: mdefName, Namespace: namespace}, installedMeterDefn)
	if err != nil && !k8serrors.IsNotFound((err)) {
		reqLogger.Error(err, "could not get meter definition", "Name", mdefName)
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	// remove owner ref from meter definition before deleting
	installedMeterDefn.ObjectMeta.OwnerReferences = []metav1.OwnerReference{}
	err = r.Client.Update(context.TODO(), installedMeterDefn)
	if err != nil {
		reqLogger.Error(err, "Failed updating owner reference on meter definition", "Name", mdefName, "Namespace", namespace)
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}
	reqLogger.Info("Removed owner reference from meterdefintion", "Name", mdefName, "Namespace", namespace)

	//TODO: requeue here ?

	reqLogger.Info("Deleteing MeterDefinition")
	err = r.Client.Delete(context.TODO(), installedMeterDefn)
	if err != nil && !k8serrors.IsNotFound(err) {
		reqLogger.Error(err, "could not delete MeterDefinition", "Name", mdefName)
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}
	reqLogger.Info("Deleted meterdefintion", "Name", mdefName, "Namespace", namespace)

	//TODO: requeue here ?

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

/*
	TODO: test whether revisionHistoryLimit will handle this
*/
func (r *DeploymentConfigReconciler) pruneDeployPods(latestVersion int64, request reconcile.Request, reqLogger logr.Logger) *ExecResult {
	reqLogger.Info("pruning old deploy pods")

	latestPodName := fmt.Sprintf("rhm-meterdefinition-file-server-%d", latestVersion)
	reqLogger.Info("Prune", "latest version", latestVersion)
	reqLogger.Info("Prune", "latest pod name", latestPodName)

	dcPodList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(request.Namespace),
		client.HasLabels{"openshift.io/deployer-pod-for.name"},
	}

	err := r.Client.List(context.TODO(), dcPodList, listOpts...)
	if err != nil {
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	for _, pod := range dcPodList.Items {
		reqLogger.Info("Prune", "deploy pod", pod.Name)
		podLabelValue := pod.GetLabels()["openshift.io/deployer-pod-for.name"]
		if podLabelValue != latestPodName {

			err := r.Client.Delete(context.TODO(), &pod)
			if err != nil {
				return &ExecResult{
					ReconcileResult: reconcile.Result{},
					Err:             err,
				}
			}

			reqLogger.Info("Successfully pruned deploy pod", "pod name", pod.Name)
		}
	}

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}
