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
//TODO: Not being used see meterdefinition_install_controller
package marketplace

import (
	// "bytes"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"

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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/pointer"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// blank assignment to verify that MeterdefConfigMapReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &MeterdefConfigMapReconciler{}

// var GlobalMeterdefStoreDB = &MeterdefStoreDB{}
// MeterdefConfigMapReconciler reconciles the DataService of a MeterBase object
type MeterdefConfigMapReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	CC     ClientCommandRunner

	cfg           *config.OperatorConfig
	factory       *manifests.Factory
	patcher       patch.Patcher
	latestVersion int64
	message       string
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

func (r *MeterdefConfigMapReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (r *MeterdefConfigMapReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	r.cfg = cfg
	return nil
}

func (r *MeterdefConfigMapReconciler) InjectCommandRunner(ccp ClientCommandRunner) error {
	r.Log.Info("command runner")
	r.CC = ccp
	return nil
}

func (r *MeterdefConfigMapReconciler) InjectPatch(p patch.Patcher) error {
	r.patcher = p
	return nil
}

func (r *MeterdefConfigMapReconciler) InjectFactory(f *manifests.Factory) error {
	r.factory = f
	return nil
}

// adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *MeterdefConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {

	nsPred := predicates.NamespacePredicate(r.cfg.DeployedNamespace)

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(nsPred).
		For(&osappsv1.DeploymentConfig{}, builder.WithPredicates(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {

					if e.Meta.GetName() == utils.DEPLOYMENT_CONFIG_NAME {
						return true
					}
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					if e.MetaNew.GetName() == utils.DEPLOYMENT_CONFIG_NAME {
						// return e.MetaOld.GetResourceVersion() != e.MetaNew.GetResourceVersion()
						oldDeploymentConfig, ok := e.ObjectOld.(*osappsv1.DeploymentConfig)
						if !ok {
							fmt.Println("could not convert to DeploymentConfig")
							return false
						}

						newDeploymentConfig, ok := e.ObjectNew.(*osappsv1.DeploymentConfig)
						if !ok {
							fmt.Println("could not convert to DeploymentConfig")
							return false
						}

						if oldDeploymentConfig.Status.LatestVersion != newDeploymentConfig.Status.LatestVersion {
							newConditions := newDeploymentConfig.Status.Conditions
							oldConditions := oldDeploymentConfig.Status.Conditions
							for i, c := range newConditions {
								if c.Type == osappsv1.DeploymentProgressing {
									if c.Reason == "NewReplicationControllerAvailable" && c.Status == corev1.ConditionTrue && oldConditions[i].Message != c.Message {
										r.latestVersion = newDeploymentConfig.Status.LatestVersion
										r.message = c.Message
										return true
									}
								}

							}
						}
					}

					return false
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					if e.Meta.GetName() == utils.DEPLOYMENT_CONFIG_NAME {
						return true
					}
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {

					if e.Meta.GetName() == utils.DEPLOYMENT_CONFIG_NAME {
						return true
					}
					return false
				},
			},
		)).
		Complete(r)

}

// +kubebuilder:rbac:groups=apps.openshift.io,resources=deploymentconfigs,verbs=get;list;watch

// Reconcile reads that state of the cluster for a MeterdefConfigmap object and makes changes based on the state read
// and what is in the MeterdefConfigmap.Spec
func (r *MeterdefConfigMapReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	reqLogger.Info("deployment config latest version", "version", r.latestVersion)
	reqLogger.Info("deployment config status message", "message", r.message)

	// Fetch the mdefKVStore instance
	cm := &corev1.ConfigMap{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.METERDEF_INSTALL_MAP_NAME, Namespace: r.cfg.DeployedNamespace}, cm)
	if err != nil {
		reqLogger.Error(err, "Failed to get MeterdefintionConfigMap")
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

	reqLogger.Info("finished reconciling for meterdefinition configmap controller")
	return reconcile.Result{}, nil
}

func (r *MeterdefConfigMapReconciler) sync(installMappings []InstallMapping, reqLogger logr.Logger) ([]InstallMapping, *ExecResult) {

	reqLogger.Info("syncing meterdefinitions")
	reqLogger.Info("install mappings", "mapping", installMappings)

	updatedInstallMappings := []InstallMapping{}
	for _, installMap := range installMappings {
		csvPackageName := installMap.PackageName
		csvName := installMap.CsvName
		csvVersion := installMap.CsvVersion
		namespace := installMap.Namespace
		installedMeterDefs := installMap.InstalledMeterdefinitions

		meterDefsFromFileServer, result := ListMeterdefintionsFromFileServer(csvPackageName, csvVersion, namespace, reqLogger)
		if !result.Is(Continue) {

			if result.Is(Error) {
				reqLogger.Error(result.GetError(), "Failed retrieving meterdefinitions from file server")
			}

			return updatedInstallMappings, result
		}
		meterDefsList := []string{}
		meterDefsMap := make(map[string]marketplacev1beta1.MeterDefinition)

		for _, meterDefItem := range meterDefsFromFileServer {
			meterDefsList = append(meterDefsList, meterDefItem.ObjectMeta.Name)
			meterDefsMap[meterDefItem.ObjectMeta.Name] = meterDefItem
		}

		// get the list of meter defs to be deleted and delete them
		mdefDeleteList := utils.SliceDifference(installedMeterDefs, meterDefsList)
		if len(mdefDeleteList) > 0 {
			err := deleteMeterDefintions(namespace, mdefDeleteList, r.Client, reqLogger)
			if err != nil {
				return updatedInstallMappings, &ExecResult{
					ReconcileResult: reconcile.Result{},
					Err:             err,
				}
			}
		}

		// get the list of meter defs to be created and create them
		mdefCreateList := utils.SliceDifference(meterDefsList, installedMeterDefs)
		if len(mdefCreateList) > 0 {
			err := r.createMeterDefintions(namespace, csvName, mdefCreateList, meterDefsMap, reqLogger)
			if err != nil {
				return updatedInstallMappings, &ExecResult{
					ReconcileResult: reconcile.Result{},
					Err:             err,
				}
			}
		}
		// get ths list of meter defs to be checked for changes and invoke update operation
		mdefUpdateList := utils.SliceIntersection(installedMeterDefs, meterDefsList)
		if len(mdefUpdateList) > 0 {
			err := updateMeterDefintions(namespace, mdefUpdateList, meterDefsMap, r.Client, reqLogger)
			if err != nil {
				return updatedInstallMappings, &ExecResult{
					ReconcileResult: reconcile.Result{},
					Err:             err,
				}
			}
		}
		installMap.InstalledMeterdefinitions = meterDefsList
		updatedInstallMappings = append(updatedInstallMappings, installMap)
	}

	return updatedInstallMappings, &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func deleteMeterDefintions(namespace string, mdefNames []string, client client.Client, reqLogger logr.Logger) error {
	for _, mdefName := range mdefNames {

		meterDefn := marketplacev1beta1.MeterDefinition{}
		err := client.Get(context.TODO(), types.NamespacedName{Name: mdefName, Namespace: namespace}, &meterDefn)
		if err != nil && !errors.IsNotFound((err)) {
			reqLogger.Error(err, "could not get meter definition", "Name", mdefName)
			return err
		}

		// should we actually remove owner ref before triggering this DELETE action?
		reqLogger.Info("Deleteing MeterDefinition")
		err = client.Delete(context.TODO(), &meterDefn)
		if err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "could not delete MeterDefinition", "Name", mdefName)
			return err
		}
	}
	return nil
}

func (r *MeterdefConfigMapReconciler) createMeterDefintions(namespace string, csvName string, mdefNames []string, meterDefsMap map[string]marketplacev1beta1.MeterDefinition, reqLogger logr.Logger) error {
	// Fetch the ClusterServiceVersion instance
	csv := &olmv1alpha1.ClusterServiceVersion{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: csvName, Namespace: namespace}, csv)
	if err != nil {
		reqLogger.Error(err, "could not fetch ClusterServiceversion isntance", "CSVName", csvName)
		return err
	}

	gvk, err := apiutil.GVKForObject(csv, r.Scheme)
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

		err = r.Client.Create(context.TODO(), &meterDefn)
		if err != nil {
			reqLogger.Error(err, "Failed creating meter definition", "Name", mdefName, "Namespace", namespace)
			return err
		}
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
				reqLogger.Error(err, "Failed updating definition", "Name", mdefName, "Namespace", namespace)
				return err
			}
			reqLogger.Info("Updated meterdefintion", "Name", mdefName, "Namespace", namespace)
		}
	}
	return nil
}

func getMeterdefintionFromFileServer(packageName string, namespace string, mdefName string, reqLogger logr.Logger) (*marketplacev1beta1.MeterDefinition, error) {

	url := fmt.Sprintf("http://rhm-meterdefinition-file-server.openshift-redhat-marketplace.svc.cluster.local:8100/get/%s/%s", packageName, mdefName)
	response, err := http.Get(url)
	if err != nil {
		if err == io.EOF {
			reqLogger.Error(err, "Meterdefintion not found")
			return nil, err
		}

		reqLogger.Error(err, "Error querying file server")
		return nil, err
	}

	mdef := marketplacev1beta1.MeterDefinition{}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		reqLogger.Error(err, "no data in response")
		return nil, err
	}

	meterDefsData := strings.Replace(string(data), "<<NAMESPACE-PLACEHOLDER>>", namespace, -1)
	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(meterDefsData)), 100).Decode(&mdef)
	if err != nil {
		reqLogger.Error(err, "error decoding meterdefstore string")
		return nil, err
	}

	return &mdef, nil
}
