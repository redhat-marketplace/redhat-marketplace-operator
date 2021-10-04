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

	"github.com/go-logr/logr"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	osappsv1 "github.com/openshift/api/apps/v1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/rhmotransport"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
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
	Client            client.Client
	Scheme            *runtime.Scheme
	Log               logr.Logger
	CC                ClientCommandRunner
	cfg               *config.OperatorConfig
	factory           *manifests.Factory
	patcher           patch.Patcher
	CatalogClient     *catalog.CatalogClient
	AuthBuilderConfig *rhmotransport.AuthBuilderConfig
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

func (r *DeploymentConfigReconciler) InjectAuthBuilderConfig(authBuilderConfig *rhmotransport.AuthBuilderConfig) error {
	r.Log.Info("AuthBuilder")
	r.AuthBuilderConfig = authBuilderConfig
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

				return meterbaseOld.Spec.MeterdefinitionCatalogServerConfig != meterbaseNew.Spec.MeterdefinitionCatalogServerConfig
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
				return e.Meta.GetName() == utils.DeploymentConfigName
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return e.MetaNew.GetName() == utils.DeploymentConfigName
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return e.Meta.GetName() == utils.DeploymentConfigName
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return e.Meta.GetName() == utils.DeploymentConfigName
			},
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(nsPred).
		For(&osappsv1.DeploymentConfig{}).
		WithOptions(controller.Options{
			Reconciler: r,
			Log:        r.Log,
		}).
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
// +kubebuilder:rbac:urls=/get-community-meterdefs,verbs=get;post;create;
// +kubebuilder:rbac:urls=/get-system-meterdefs,verbs=get;post;create;
// +kubebuilder:rbac:urls=/community-meterdef-index/*,verbs=get;
// +kubebuilder:rbac:urls=/system-meterdef-index/*,verbs=get;
// +kubebuilder:rbac:urls=/global-community-meterdef-index,verbs=get;
// +kubebuilder:rbac:urls=/global-system-meterdef-index,verbs=get;
// +kubebuilder:rbac:groups="authentication.k8s.io",resources=tokenreviews,verbs=create;get
// +kubebuilder:rbac:groups="authorization.k8s.io",resources=subjectaccessreviews,verbs=create;get

// Reconcile reads that state of the cluster for a DeploymentConfig object and makes changes based on the state read
func (r *DeploymentConfigReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	instance := &marketplacev1alpha1.MeterBase{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.METERBASE_NAME, Namespace: request.Namespace}, instance)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			reqLogger.Error(err, "meterbase does not exist must have been deleted - ignoring for now")
			return reconcile.Result{}, nil
		}

		reqLogger.Error(err, "Failed to get meterbase")
		return reconcile.Result{}, err
	}

	if instance.Spec.MeterdefinitionCatalogServerConfig == nil {
		reqLogger.Info("meterbase doesn't have file server feature flags set")
		return reconcile.Result{}, nil
	}

	err = r.CatalogClient.SetRetryForCatalogClient(r.AuthBuilderConfig, reqLogger)
	if err != nil {
		return reconcile.Result{}, err
	}

	// if SyncSystemMeterDefinitions is disabled delete all system meterdefs for csvs originating from rhm
	if !instance.Spec.MeterdefinitionCatalogServerConfig.SyncSystemMeterDefinitions {
		isRunning := r.isDeploymentConfigRunning(reqLogger)
		if isRunning {
			reqLogger.Info("sync for system meterdefs has been disabled, uninstalling system meterdefs")

			err = r.deleteAllSystemMeterDefsForRhmCvs(reqLogger)
			if err != nil {
				return reconcile.Result{}, err
			}

			reqLogger.Info("done removing system meterdefinitions")

		}
	}

	// if SyncCommunityMeterDefinitions is disabled delete all community meterdefs for csvs originating from rhm
	if !instance.Spec.MeterdefinitionCatalogServerConfig.SyncCommunityMeterDefinitions {
		isRunning := r.isDeploymentConfigRunning(reqLogger)
		if isRunning {
			reqLogger.Info("sync for community meterdefs has been disabled, uninstalling system meterdefs")

			err = r.deleteAllCommunityMeterDefsForRhmCvs(reqLogger)
			if err != nil {
				return reconcile.Result{}, err
			}

			reqLogger.Info("done removing community meterdefinitions")
		}
	}

	// catalog server not enabled. Uninstall deploymentconfig resources, uninstall all community & system meterdefs, and stop reconciling
	if !instance.Spec.MeterdefinitionCatalogServerConfig.DeployMeterDefinitionCatalogServer {
		result := r.uninstallFileServerDeploymentResources(reqLogger)
		if !result.Is(Continue) {
			result.Return()
		}

		reqLogger.Info("done uninstalling catalog server resources,stopping reconcile")
		return reconcile.Result{}, nil
	}

	result := r.reconcileCatalogServerResources(instance, request, reqLogger)
	if !result.Is(Continue) {
		return result.Return()
	}

	// get the latest deploymentconfig
	dc := &osappsv1.DeploymentConfig{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.DeploymentConfigName, Namespace: r.cfg.DeployedNamespace}, dc)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			reqLogger.Info("deployment config not found, ignoring")
			return reconcile.Result{}, nil
		}

		reqLogger.Error(err, "Failed to get deploymentconfig")
		return reconcile.Result{}, err
	}

	//TODO: zach recheck for image pull failure
	for _, c := range dc.Status.Conditions {
		// DeploymentAvailable means the DeploymentConfig has the minimum available replicas required
		if c.Type == osappsv1.DeploymentProgressing && c.Status == corev1.ConditionFalse {
			err = errors.New(c.Message)
			return result.ReconcileResult, err

		}

		if c.Type == osappsv1.DeploymentReplicaFailure {
			err = errors.New(c.Message)
			return result.ReconcileResult, err
		}
	}

	//TODO: remove requeue [x]
	for _, c := range dc.Status.Conditions {
		if c.Type == osappsv1.DeploymentAvailable {
			if c.Status != corev1.ConditionTrue {
				err = errors.New(c.Message)
				return reconcile.Result{}, err
			}
		}

		// catch the deploymentconfig as it's rolling out a new deployment and requeue until finished
		// DeploymentProgressing is: * True: the DeploymentConfig has been successfully deployed or is currently getting deployed.
		// TODO: re-test this and see if we can remove this
		if c.Type == osappsv1.DeploymentProgressing {
			if c.Reason != "NewReplicationControllerAvailable" || c.Status != corev1.ConditionTrue || dc.Status.LatestVersion == dc.Status.ObservedGeneration {
				err = errors.New(c.Message)
				return reconcile.Result{}, err
			}
		}
	}

	reqLogger.Info("deploymentconfig is in ready state")

	//syncs the latest meterdefinitions from the catalog with the community & system (templated) meterdefinitions on the cluster
	result = r.sync(instance, request, reqLogger)
	if !result.Is(Continue) {
		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "error on sync")
		}

		return result.Return()
	}

	reqLogger.Info("finished reconciling")
	return reconcile.Result{}, nil
}

//TODO: zach remove ExecResult - low priority
func (r *DeploymentConfigReconciler) sync(instance *marketplacev1alpha1.MeterBase, request reconcile.Request, reqLogger logr.Logger) *ExecResult {
	subs, err := listSubs(r.Client)
	if err != nil {
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	for _, sub := range subs {
		isRHMSub := checkOperatorTag(&sub)
		if !isRHMSub {
			reqLogger.Info("subscription does not have operator tag", "sub", sub.Name)
			continue
		}

		var csvName string
		if sub.Status.InstalledCSV == "" {
			err = fmt.Errorf("subscription does not have InstalledCSV set: %s, subscription namespace: %s", sub.Name, sub.Namespace)
			return &ExecResult{
				Status: ActionResultStatus(Error),
				Err:    err,
			}
		}

		csvName = sub.Status.InstalledCSV
		reqLogger.Info("Subscription has InstalledCSV", "csv", csvName)

		csv := &olmv1alpha1.ClusterServiceVersion{}

		// find InstalledCSV in the same namespace as the subscription
		err = r.Client.Get(context.TODO(), types.NamespacedName{Name: csvName, Namespace: sub.Namespace}, csv)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				err = fmt.Errorf("could not find csv: %s, from subscription: %s, subscription namespace: %s", csv.Name, sub.Name, sub.Namespace)
				return &ExecResult{
					Status: ActionResultStatus(Error),
					Err:    err,
				}
			}

			reqLogger.Error(err, "Failed to get clusterserviceversion")
			return &ExecResult{
				Status: ActionResultStatus(Error),
				Err:    err,
			}
		}

		reqLogger.Info("found installed csv from subscription", "csv", csv.Name)

		cr := &catalog.CatalogRequest{
			CSVInfo: catalog.CSVInfo{
				Name:      csvName,
				Namespace: csv.Namespace,
				Version:   csv.Spec.Version.Version.String(),
			},
			SubInfo: catalog.SubInfo{
				PackageName:   sub.Spec.Package,
				CatalogSource: sub.Spec.CatalogSource,
			},
		}

		if instance.Spec.MeterdefinitionCatalogServerConfig != nil {
			if instance.Spec.MeterdefinitionCatalogServerConfig.SyncSystemMeterDefinitions {
				err = r.syncSystemMeterDefs(csv, cr, reqLogger)
				if err != nil {
					reqLogger.Info(err.Error())
				}
			}
		}

		if instance.Spec.MeterdefinitionCatalogServerConfig != nil {
			if instance.Spec.MeterdefinitionCatalogServerConfig.SyncCommunityMeterDefinitions {
				err = r.syncCommunityMeterDefs(cr, csv, reqLogger)
				if err != nil {
					reqLogger.Info(err.Error())
				}
			}
		}
	}

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func checkOperatorTag(sub *olmv1alpha1.Subscription) bool {
	if value, ok := sub.GetLabels()[utils.OperatorTag]; ok {
		if value == utils.OperatorTagValue {
			return true
		}
	}

	return false
}

func (r *DeploymentConfigReconciler) syncSystemMeterDefs(csv *olmv1alpha1.ClusterServiceVersion, cr *catalog.CatalogRequest, reqLogger logr.Logger) error {
	latestSystemMeterDefsFromCatalog, err := r.CatalogClient.GetSystemMeterdefs(cr)
	if err != nil {
		return err
	}

	err = r.createOrUpdate(latestSystemMeterDefsFromCatalog, csv, reqLogger)
	if err != nil {
		return err
	}

	systemMeterDefIndexLabels, err := r.CatalogClient.GetSystemMeterDefIndexLabels(csv.Name, cr.PackageName, cr.CatalogSource)
	if err != nil {
		return err
	}

	systemMeterDefsOnCluster, err := r.listMeterDefsForCsvWithIndex(systemMeterDefIndexLabels)
	if err != nil {
		return err
	}

	err = r.deleteOnDiff(systemMeterDefsOnCluster.Items, latestSystemMeterDefsFromCatalog, reqLogger)
	if err != nil {
		return err
	}

	return nil
}

func (r *DeploymentConfigReconciler) syncCommunityMeterDefs(cr *catalog.CatalogRequest, csv *olmv1alpha1.ClusterServiceVersion, reqLogger logr.Logger) error {
	/*
		pings the file server for a map of labels we use to index meterdefintions that originated from the file server
		these labels also get added to a meterdefinition by the file server	before it returns
		{
			"marketplace.redhat.com/installedOperatorNameTag": "<csvName>",
			"marketplace.redhat.com/isCommunityMeterdefintion": "true"
			"subscription.package": <sub.Spec.Package>
			"subscription.name": <sub.Spec.Catalog>
		}

	*/
	communityIndexLabels, err := r.CatalogClient.GetCommunityMeterdefIndexLabels(csv.Name, cr.PackageName, cr.CatalogSource)
	if err != nil {
		return err
	}

	/*
		csv is on the cluster but doesn't have mdefs in the catalog
		delete all community meterdefs for that csv
		if no community meterdefs are found, skip to next csv
		if community meterdefs are found and deleted successfully, skip to next csv
	*/
	//TODO: handle when the file server can't be reached - return err [x]
	latestCommunityMeterDefsFromCatalog, err := r.CatalogClient.ListMeterdefintionsFromFileServer(cr)
	if err != nil {
		if errors.Is(err, catalog.ErrCatalogNoContent) {
			reqLogger.Info("catalog meterdefinitions have been deleted from that catalog for csv", "csv", csv.Name)

			/*
				deletes mdefs that match:
				labels:
					marketplace.redhat.com/installedOperatorNameTag: memcached-operator.v0.0.1
					marketplace.redhat.com/isCommunityMeterdefintion: '1'
					subscription.name: memcached-operator-rhmp
					subscription.source: max-test-catalog
			*/
			err = r.deleteMeterdefsWithIndex(communityIndexLabels, reqLogger)
			if err != nil {
				return err
			}

			reqLogger.Info("skipping sync for csv", "csv", csv.Name)
			return nil
		}

		return err
	}

	err = r.createOrUpdate(latestCommunityMeterDefsFromCatalog, csv, reqLogger)
	if err != nil {
		return err
	}

	/*
		delete if there is a meterdef installed on the cluster that originated from the catalog, but that meterdef isn't in the latest file server image
	*/
	catalogMdefsOnCluster, err := r.listMeterDefsForCsvWithIndex(communityIndexLabels)
	if err != nil {
		return err
	}

	err = r.deleteOnDiff(catalogMdefsOnCluster.Items, latestCommunityMeterDefsFromCatalog, reqLogger)
	if err != nil {
		return err
	}

	return nil
}

/*
	//TODO:
	setting this to a var so I can mock it in deploymentconfig_conttroller_test.go
	was having trouble setting the status for subscriptions created in the test env
*/
var listSubs = func(k8sclient client.Client) ([]olmv1alpha1.Subscription, error) {
	subList := &olmv1alpha1.SubscriptionList{}
	if err := k8sclient.List(context.TODO(), subList); err != nil {
		return nil, err
	}

	return subList.Items, nil
}

func (r *DeploymentConfigReconciler) createOrUpdate(latestMeterDefsFromCatalog []marketplacev1beta1.MeterDefinition, csv *olmv1alpha1.ClusterServiceVersion, reqLogger logr.Logger) error {

	for _, catalogMeterdef := range latestMeterDefsFromCatalog {
		installedMdef := &marketplacev1beta1.MeterDefinition{}

		reqLogger.Info("finding meterdefintion", catalogMeterdef.Name, catalogMeterdef.Namespace)

		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			err := r.Client.Get(context.TODO(), types.NamespacedName{Name: catalogMeterdef.Name, Namespace: catalogMeterdef.Namespace}, installedMdef)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					reqLogger.Info("meterdef not found during sync, creating", "name", catalogMeterdef.Name, "namespace", catalogMeterdef.Namespace)
					err := r.createMeterdefWithOwnerRef(catalogMeterdef, csv, reqLogger)
					if err != nil {
						return err
					}

					return nil
				}

				return err
			}

			/*
				update a meterdef for a csv if a meterdef from the catalog is also on the cluster
				&& the meterdef from the catalog contains an update to .Spec or .Annotations
				//TODO: what fields should we check a diff for ?
			*/
			return r.updateMeterdef(installedMdef, catalogMeterdef, reqLogger)

		})

		if err != nil {
			//TODO: should we create meterdefs here on IsNotFound errs ? or keep as is
			return err
		}
	}

	return nil
}

func (r *DeploymentConfigReconciler) isDeploymentConfigRunning(reqLogger logr.Logger) bool {
	dc := &osappsv1.DeploymentConfig{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.DeploymentConfigName, Namespace: r.cfg.DeployedNamespace}, dc)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			reqLogger.Info("deployment config not found, ignoring")
			return false
		}

		reqLogger.Error(err, "Failed to get deploymentconfig")
		return false
	}

	for _, c := range dc.Status.Conditions {
		if c.Type == osappsv1.DeploymentAvailable {
			if c.Status != corev1.ConditionTrue {
				return false
			}
		}
	}

	return true
}

func (r *DeploymentConfigReconciler) uninstallFileServerDeploymentResources(reqLogger logr.Logger) (result *ExecResult) {
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

func (r *DeploymentConfigReconciler) uninstallDeploymentConfig(reqLogger logr.Logger) *ExecResult {
	foundDeploymentConfig := &osappsv1.DeploymentConfig{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.DeploymentConfigName, Namespace: r.cfg.DeployedNamespace}, foundDeploymentConfig)
	if err != nil {
		if k8serrors.IsNotFound(err) {
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

	err = r.Client.Delete(context.TODO(), foundDeploymentConfig)
	if err != nil && !k8serrors.IsNotFound(err) {
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

func (r *DeploymentConfigReconciler) uninstallService(reqLogger logr.Logger) *ExecResult {
	foundFileServerService := &corev1.Service{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.DeploymentConfigName, Namespace: r.cfg.DeployedNamespace}, foundFileServerService)
	if err != nil {
		if k8serrors.IsNotFound(err) {
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

	err = r.Client.Delete(context.TODO(), foundFileServerService)
	if err != nil && !k8serrors.IsNotFound(err) {
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

func (r *DeploymentConfigReconciler) uninstallImageStream(reqLogger logr.Logger) *ExecResult {
	foundImageStream := &osimagev1.ImageStream{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.DeploymentConfigName, Namespace: r.cfg.DeployedNamespace}, foundImageStream)
	if err != nil {
		if k8serrors.IsNotFound(err) {
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

	err = r.Client.Delete(context.TODO(), foundImageStream)
	if err != nil && !k8serrors.IsNotFound(err) {
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

func (r *DeploymentConfigReconciler) reconcileCatalogServerResources(instance *marketplacev1alpha1.MeterBase, request reconcile.Request, reqLogger logr.Logger) *ExecResult {
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
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.DeploymentConfigName, Namespace: r.cfg.DeployedNamespace}, foundDeploymentConfig)
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

		}

		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		updated := r.factory.UpdateDeploymentConfigOnChange(foundDeploymentConfig)
		if updated {
			err = r.Client.Update(context.TODO(), foundDeploymentConfig)
			if err != nil {
				return err
			}

			reqLogger.Info("updated deploymentconfig")
		}

		return nil
	})

	if err != nil {
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	foundfileServerService := &corev1.Service{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.DeploymentConfigName, Namespace: r.cfg.DeployedNamespace}, foundfileServerService)
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
				ReconcileResult: reconcile.Result{},
				Err:             err,
			}
		}

		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	foundImageStream := &osimagev1.ImageStream{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.DeploymentConfigName, Namespace: r.cfg.DeployedNamespace}, foundImageStream)
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
				return &ExecResult{
					ReconcileResult: reconcile.Result{},
					Err:             err,
				}
			}

			reqLogger.Info("created new image stream")
		}

		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		imageStreamUpdated := r.factory.UpdateImageStreamOnChange(foundImageStream)
		if imageStreamUpdated {
			err = r.Client.Update(context.TODO(), foundImageStream)
			if err != nil {
				return err
			}

			reqLogger.Info("updated ImageStream")

		}

		return nil
	})

	if err != nil {
		reqLogger.Error(err, "error on reconciliation of image stream")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}

}

func (r *DeploymentConfigReconciler) updateMeterdef(installedMdef *marketplacev1beta1.MeterDefinition, catalogMdef marketplacev1beta1.MeterDefinition, reqLogger logr.Logger) error {
	updatedMeterdefinition := installedMdef.DeepCopy()
	updatedMeterdefinition.Spec = catalogMdef.Spec
	updatedMeterdefinition.ObjectMeta.Annotations = catalogMdef.ObjectMeta.Annotations

	if !reflect.DeepEqual(updatedMeterdefinition, installedMdef) {
		reqLogger.Info("meterdefintion is out of sync with latest meterdef catalog", "name", installedMdef.Name)
		err := r.Client.Update(context.TODO(), updatedMeterdefinition)
		if err != nil {
			reqLogger.Error(err, "Failed updating meter definition", "name", updatedMeterdefinition.Name, "namespace", updatedMeterdefinition.Namespace)
			return err
		}

		reqLogger.Info("Updated meterdefintion", "name", updatedMeterdefinition.Name, "namespace", updatedMeterdefinition.Namespace)
	}

	return nil
}

func (r *DeploymentConfigReconciler) createMeterdefWithOwnerRef(meterDefinition marketplacev1beta1.MeterDefinition, csv *olmv1alpha1.ClusterServiceVersion, reqLogger logr.InfoLogger) error {

	gvk, err := apiutil.GVKForObject(csv, r.Scheme)
	if err != nil {
		return nil
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
		return nil
	}

	reqLogger.Info("Created meterdefinition", "mdef", meterDefinition.Name, "CSV", csv.Name)

	return nil
}

func (r *DeploymentConfigReconciler) deleteOnDiff(catalogMdefsOnCluster []marketplacev1beta1.MeterDefinition, latestMeterdefsFromCatalog []marketplacev1beta1.MeterDefinition, reqLogger logr.Logger) error {
	reqLogger.Info("delete on diff")

	deleteList := utils.FindMeterdefSliceDiff(catalogMdefsOnCluster, latestMeterdefsFromCatalog)
	if len(deleteList) != 0 {
		for _, mdef := range deleteList {
			reqLogger.Info("meterdef has been selected for deletion", "meterdef", mdef.Name)

			err := r.deleteMeterDef(mdef.Name, mdef.Namespace, reqLogger)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *DeploymentConfigReconciler) listMeterDefsForCsvWithIndex(indexLabels map[string]string) (*marketplacev1beta1.MeterDefinitionList, error) {
	installedMeterdefList := &marketplacev1beta1.MeterDefinitionList{}

	// look for meterdefs that originated from the meterdefinition catalog
	listOpts := []client.ListOption{
		client.MatchingLabels(indexLabels),
	}

	err := r.Client.List(context.TODO(), installedMeterdefList, listOpts...)
	if err != nil {
		return nil, err
	}

	return installedMeterdefList, nil
}

func (r *DeploymentConfigReconciler) deleteAllSystemMeterDefsForRhmCvs(reqLogger logr.Logger) error {
	reqLogger.Info("deleting all meterdefs community meterdefs")
	/*
		returns the label:
			{
				"marketplace.redhat.com/isCommunityMeterdefintion": "1"
			}
	*/
	globalSystemIndexLabel, err := r.CatalogClient.GetGlobalSystemMeterDefIndexLabels()
	if err != nil {
		return err
	}

	err = r.deleteMeterdefsWithIndex(globalSystemIndexLabel, reqLogger)
	if err != nil {
		return err
	}

	return nil
}

func (r *DeploymentConfigReconciler) deleteAllCommunityMeterDefsForRhmCvs(reqLogger logr.Logger) error {
	reqLogger.Info("deleting all meterdefs community meterdefs")

	/*
		returns the label:
		{
			"marketplace.redhat.com/isCommunityMeterdefintion": "1"
		}
	*/
	globalCommunityIndexLabel, err := r.CatalogClient.GetGlobalCommunityMeterdefIndexLabel()
	if err != nil {
		return err
	}

	err = r.deleteMeterdefsWithIndex(globalCommunityIndexLabel, reqLogger)
	if err != nil {
		return err
	}

	return nil
}

func (r *DeploymentConfigReconciler) deleteMeterdefsWithIndex(indexLabels map[string]string, reqLogger logr.Logger) error {
	reqLogger.Info("deleting meterdefinitions with index", "index", indexLabels)

	installedMeterdefList := &marketplacev1beta1.MeterDefinitionList{}

	// look for meterdefs that are from the meterdefinition catalog
	listOpts := []client.ListOption{
		client.MatchingLabels(indexLabels),
	}

	err := r.Client.List(context.TODO(), installedMeterdefList, listOpts...)
	if err != nil {
		reqLogger.Info("client list error", "err", err.Error())
		return err
	}

	if len(installedMeterdefList.Items) == 0 {
		reqLogger.Info("no meterdefinitions found on cluster for csv with index", "index", indexLabels)
		return nil
	}

	for _, mdef := range installedMeterdefList.Items {
		reqLogger.Info("deleting meterdefintion", "name", mdef.Name)
		err := r.deleteMeterDef(mdef.Name, mdef.Namespace, reqLogger)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *DeploymentConfigReconciler) deleteMeterDef(mdefName string, namespace string, reqLogger logr.Logger) error {
	installedMeterDefn := &marketplacev1beta1.MeterDefinition{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: mdefName, Namespace: namespace}, installedMeterDefn)
	if err != nil && !k8serrors.IsNotFound((err)) {
		reqLogger.Error(err, "could not get meter definition", "name", mdefName)
		return err
	}

	// remove owner ref from meter definition before deleting
	installedMeterDefn.ObjectMeta.OwnerReferences = []metav1.OwnerReference{}

	err = r.Client.Update(context.TODO(), installedMeterDefn)
	if err != nil {
		reqLogger.Error(err, "Failed updating owner reference on meter definition", "name", mdefName, "namespace", namespace)
		return err
	}

	reqLogger.Info("Removed owner reference from meterdefintion", "name", mdefName, "namespace", namespace)

	reqLogger.Info("Deleting MeterDefinition", "mdef", mdefName)

	err = r.Client.Delete(context.TODO(), installedMeterDefn)
	if err != nil && !k8serrors.IsNotFound(err) {
		reqLogger.Error(err, "could not delete MeterDefinition", "name", mdefName)
		return err
	}

	reqLogger.Info("Deleted meterdefintion", "name", mdefName, "namespace", namespace)

	return nil
}
