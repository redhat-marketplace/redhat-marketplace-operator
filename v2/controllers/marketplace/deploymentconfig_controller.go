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
	// "bytes"

	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/client-go/kubernetes"

	osappsv1 "github.com/openshift/api/apps/v1"

	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// blank assignment to verify that DeploymentConfigReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &DeploymentConfigReconciler{}

// var GlobalMeterdefStoreDB = &MeterdefStoreDB{}
// DeploymentConfigReconciler reconciles the DataService of a MeterBase object
type DeploymentConfigReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	CC     ClientCommandRunner

	cfg     *config.OperatorConfig
	factory *manifests.Factory
	patcher patch.Patcher
	kubeInterface kubernetes.Interface

}

type InstallMapping struct {
	PackageName               string   `json:"packageName"`
	Namespace                 string   `json:"namespace"`
	CsvName                   string   `json:"csvName"`
	CsvVersion                string   `json:"version"`
	InstalledMeterdefinitions []string `json:"installedMeterdefinitions"`
}

//TODO: mutex needed here ?
type MeterdefinitionStore struct {
	InstallMappings []InstallMapping `json:"installMappings"`
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

func (r *DeploymentConfigReconciler) InjectPatch(p patch.Patcher) error {
	r.patcher = p
	return nil
}

func (r *DeploymentConfigReconciler) InjectFactory(f *manifests.Factory) error {
	r.factory = f
	return nil
}

func (r *DeploymentConfigReconciler) InjectKubeInterface(k kubernetes.Interface) error {
	r.kubeInterface = k
	return nil
}

// adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *DeploymentConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {

	nsPred := predicates.NamespacePredicate(r.cfg.DeployedNamespace)

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(nsPred).
		For(&osappsv1.DeploymentConfig{}, builder.WithPredicates(
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
		)).
		Complete(r)

}

// +kubebuilder:rbac:groups=apps.openshift.io,resources=deploymentconfigs,verbs=get;list;watch
// +kubebuilder:rbac:urls=/rhm-meterdefinition-file-server.openshift-redhat-marketplace.svc/list-for-version,verbs=get;

// Reconcile reads that state of the cluster for a MeterdefConfigmap object and makes changes based on the state read
// and what is in the MeterdefConfigmap.Spec
func (r *DeploymentConfigReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	dc := &osappsv1.DeploymentConfig{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.DEPLOYMENT_CONFIG_NAME, Namespace: request.Namespace}, dc)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, "deploymentconfig does not exist")
			return reconcile.Result{}, nil
		}

		reqLogger.Error(err, "Failed to get deploymentconfig")
		return reconcile.Result{}, err
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

	result := r.pruneDeployPods(latestVersion, request, reqLogger)
	if !result.Is(Continue) {

		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed during pruning operation")
		}

		return result.Return()
	}

	// Fetch the install-map-cm instance
	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.METERDEF_INSTALL_MAP_NAME, Namespace: r.cfg.DeployedNamespace}, cm)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, "meterdefinition install map cm does not exist")
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get meterdefinition install map cm")
		return reconcile.Result{}, err
	}

	cmMdefStore := cm.Data["meterdefinitionStore"]

	meterdefStore := &MeterdefinitionStore{}

	err = json.Unmarshal([]byte(cmMdefStore), meterdefStore)
	if err != nil {
		reqLogger.Error(err, "error unmarshaling meterdefinition store")
		return reconcile.Result{}, err
	}

	updatedInstallMappings, result := r.sync(meterdefStore.InstallMappings, reqLogger)
	if !result.Is(Continue) {

		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed during sync operation")
		}

		return result.Return()
	}

	meterdefStore.InstallMappings = updatedInstallMappings
	out, err := json.Marshal(meterdefStore)
	if err != nil {
		reqLogger.Error(err, "error marshaling meterdefinition store")
		return reconcile.Result{}, err
	}

	meterdefStoreJSON := string(out)
	cm.Data["meterdefinitionStore"] = meterdefStoreJSON

	err = r.Client.Update(context.TODO(), cm)
	if err != nil {
		return reconcile.Result{}, err
	}

	reqLogger.Info("finished reconciling")
	return reconcile.Result{}, nil
}

func getMeterdefStoreFromCM(client client.Client, deployedNamespace string, reqLogger logr.Logger) (*MeterdefinitionStore, *ExecResult) {
	// Fetch the install-map-cm instance
	cm := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: utils.METERDEF_INSTALL_MAP_NAME, Namespace: deployedNamespace}, cm)
	if err != nil {
		reqLogger.Error(err, "Failed to get MeterdefintionConfigMap")
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	cmMdefStore := cm.Data["meterdefinitionStore"]

	meterdefStore := &MeterdefinitionStore{}

	err = json.Unmarshal([]byte(cmMdefStore), meterdefStore)
	if err != nil {
		reqLogger.Error(err, "error unmarshaling meterdefinition store")
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	return meterdefStore, &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func (r *DeploymentConfigReconciler) sync(installMappings []InstallMapping, reqLogger logr.Logger) ([]InstallMapping, *ExecResult) {

	reqLogger.Info("syncing meterdefinitions")
	reqLogger.Info("install mappings", "mapping", installMappings)

	updatedInstallMappings := []InstallMapping{}
	for _, installMap := range installMappings {
		csvPackageName := installMap.PackageName
		csvName := installMap.CsvName
		csvVersion := installMap.CsvVersion
		namespace := installMap.Namespace
		installedMeterDefs := installMap.InstalledMeterdefinitions

		meterDefNamesFromFileServer, meterDefsFromFileServer, result := ListMeterdefintionsFromFileServer(csvPackageName, csvVersion, namespace, r.Client,r.kubeInterface,r.cfg.DeployedNamespace ,reqLogger)
		if !result.Is(Continue) {
			return nil, result
		}

		meterDefsMapFromFileServer := make(map[string]marketplacev1beta1.MeterDefinition)

		for _, meterDefItem := range meterDefsFromFileServer {
			meterDefsMapFromFileServer[meterDefItem.ObjectMeta.Name] = meterDefItem
		}

		// get the list of meter defs to be deleted and delete them
		mdefDeleteList := utils.SliceDifference(installedMeterDefs, meterDefNamesFromFileServer)
		if len(mdefDeleteList) > 0 {
			reqLogger.Info("Delete Sync", "meterdefintion names from file server", meterDefNamesFromFileServer)
			reqLogger.Info("Delete Sync", "installed meterdefinitions", installedMeterDefs)
			reqLogger.Info("Delete Sync", "meterdefintion delete list", mdefDeleteList)
			err := deleteMeterDefintions(namespace, mdefDeleteList, r.Client, reqLogger)
			if err != nil {
				return updatedInstallMappings, &ExecResult{
					ReconcileResult: reconcile.Result{},
					Err:             err,
				}
			}
			reqLogger.Info("Successfully deleted meterdefintions", "Count", len(mdefDeleteList), "Namespace", namespace, "CSV", csvName)
		}

		// get the list of meter defs to be created and create them
		mdefCreateList := utils.SliceDifference(meterDefNamesFromFileServer, installedMeterDefs)
		if len(mdefCreateList) > 0 {
			reqLogger.Info("Create Sync", "meterdefintion names from file server", meterDefNamesFromFileServer)
			reqLogger.Info("Create Sync", "installed meterdefinitions", installedMeterDefs)
			reqLogger.Info("Create Sync", "meterdefintion update list", mdefCreateList)
			err := createMeterDefintions(r.Scheme, r.Client, namespace, csvName, mdefCreateList, meterDefsMapFromFileServer, reqLogger)
			if err != nil {
				return updatedInstallMappings, &ExecResult{
					ReconcileResult: reconcile.Result{},
					Err:             err,
				}
			}
			reqLogger.Info("Successfully created meterdefintions", "Count", len(mdefCreateList), "Namespace", namespace, "CSV", csvName)
		}
		// get ths list of meter defs to be checked for changes and invoke update operation
		mdefUpdateList := utils.SliceIntersection(installedMeterDefs, meterDefNamesFromFileServer)
		if len(mdefUpdateList) > 0 {
			err := updateMeterDefintions(namespace, mdefUpdateList, meterDefsMapFromFileServer, r.Client, reqLogger)
			if err != nil {
				return updatedInstallMappings, &ExecResult{
					ReconcileResult: reconcile.Result{},
					Err:             err,
				}
			}
		}
		installMap.InstalledMeterdefinitions = meterDefNamesFromFileServer
		updatedInstallMappings = append(updatedInstallMappings, installMap)
	}
	reqLogger.Info("Sync complete")
	reqLogger.Info("Current list of install mappings", "Install mappings", updatedInstallMappings)

	return updatedInstallMappings, &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func deleteMeterDefintions(namespace string, mdefNames []string, client client.Client, reqLogger logr.Logger) error {
	for _, mdefName := range mdefNames {

		installedMeterDefn := &marketplacev1beta1.MeterDefinition{}
		err := client.Get(context.TODO(), types.NamespacedName{Name: mdefName, Namespace: namespace}, installedMeterDefn)
		if err != nil && !errors.IsNotFound((err)) {
			reqLogger.Error(err, "could not get meter definition", "Name", mdefName)
			return err
		}

		// remove owner ref from meter definition before deleting
		installedMeterDefn.ObjectMeta.OwnerReferences = []metav1.OwnerReference{}
		err = client.Update(context.TODO(), installedMeterDefn)
		if err != nil {
			reqLogger.Error(err, "Failed updating meter definition", "Name", mdefName, "Namespace", namespace)
			return err
		}
		reqLogger.Info("Removed owner reference from meterdefintion", "Name", mdefName, "Namespace", namespace)

		reqLogger.Info("Deleteing MeterDefinition")
		err = client.Delete(context.TODO(), installedMeterDefn)
		if err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "could not delete MeterDefinition", "Name", mdefName)
			return err
		}
		reqLogger.Info("Deleted meterdefintion", "Name", mdefName, "Namespace", namespace)
	}
	return nil
}

func createMeterDefintions(scheme *runtime.Scheme, client client.Client, namespace string, csvName string, mdefNames []string, meterDefsMap map[string]marketplacev1beta1.MeterDefinition, reqLogger logr.Logger) error {
	// Fetch the ClusterServiceVersion instance
	csv := &olmv1alpha1.ClusterServiceVersion{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: csvName, Namespace: namespace}, csv)
	if err != nil {
		reqLogger.Error(err, "could not fetch ClusterServiceversion isntance", "CSVName", csvName)
		return err
	}

	gvk, err := apiutil.GVKForObject(csv, scheme)
	if err != nil {
		return err
	}

	// create owner reference instance
	ref := metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               csv.GetName(),
		UID:                csv.GetUID(),
		BlockOwnerDeletion: pointer.BoolPtr(false),
		Controller:         pointer.BoolPtr(false),
	}

	// create meter definitions
	for _, mdefName := range mdefNames {
		meterDefn := meterDefsMap[mdefName]
		meterDefn.ObjectMeta.SetOwnerReferences([]metav1.OwnerReference{ref})
		meterDefn.ObjectMeta.Namespace = namespace

		err = client.Create(context.TODO(), &meterDefn)
		if err != nil {

			reqLogger.Error(err, "Failed creating meter definition", "Name", mdefName, "Namespace", namespace)
			return err
		}

		reqLogger.Info("Created meterdefintion", "Name", mdefName, "Namespace", namespace)
	}
	return nil
}

func updateMeterDefintions(namespace string, mdefNames []string, meterDefsMap map[string]marketplacev1beta1.MeterDefinition, client client.Client, reqLogger logr.Logger) error {
	for _, mdefName := range mdefNames {
		meterdefFromCluster := &marketplacev1beta1.MeterDefinition{}
		err := client.Get(context.TODO(), types.NamespacedName{Name: mdefName, Namespace: namespace}, meterdefFromCluster)
		if err != nil {
			return err
		}

		updatedMeterdefinition := meterdefFromCluster.DeepCopy()
		updatedMeterdefinition.Spec = meterDefsMap[mdefName].Spec
		updatedMeterdefinition.ObjectMeta.Annotations = meterDefsMap[mdefName].ObjectMeta.Annotations

		if !reflect.DeepEqual(updatedMeterdefinition, meterdefFromCluster) {
			reqLogger.Info("meterdefintion is out of sync with latest meterdef catalog", "Name", meterdefFromCluster.Name)
			err = client.Update(context.TODO(), updatedMeterdefinition)
			if err != nil {
				reqLogger.Error(err, "Failed updating meter definition", "Name", mdefName, "Namespace", namespace)
				return err
			}
			reqLogger.Info("Updated meterdefintion", "Name", mdefName, "Namespace", namespace)
		}
	}
	return nil
}

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
