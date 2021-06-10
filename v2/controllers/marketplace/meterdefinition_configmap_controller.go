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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// type MeterdefStoreDB struct {
// 	sync.Mutex
// 	Meterdefinitions []marketplacev1beta1.MeterDefinition
// }
type InstallMapping struct {
	PackageName               string   `json:"packageName"`
	Namespace                 string   `json:"namespace"`
	Version                   string   `json:"version"` //TODO: not being used
	VersionRangeDir           string   `json:"versionRange"`
	InstalledMeterdefinitions []string `json:"installedMeterdefinitions"`
}

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

// add adds a new Controller to mgr with r as the reconcile.Reconciler
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

// +kubebuilder:rbac:groups=apps.openshift.io,resources=deploymentconfigs,verbs=get;list;watch;create

// Reconcile reads that state of the cluster for a MeterdefConfigmap object and makes changes based on the state read
// and what is in the MeterdefConfigmap.Spec
func (r *MeterdefConfigMapReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	reqLogger.Info("deployment config latest version", "version", r.latestVersion)
	reqLogger.Info("deployment config status message", "message", r.message)
	// Fetch the mdefKVStore instance
	mdefKVStore := &corev1.ConfigMap{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.METERDEF_INSTALL_MAP_NAME, Namespace: request.Namespace}, mdefKVStore)
	if err != nil {
		if errors.IsNotFound(err) {

			reqLogger.Info("meterdef install map cm not found")

			return reconcile.Result{Requeue: true}, nil
		}

		reqLogger.Error(err, "Failed to get MeterdefintionConfigMap")
		return reconcile.Result{}, err
	}

	result := r.sync(reqLogger)
	if !result.Is(Continue) {

		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to create meterdef store.")
		}

		return result.Return()
	}

	reqLogger.Info("finished reconciling")
	return reconcile.Result{}, nil
}

func (r *MeterdefConfigMapReconciler) getMeterdefInstallMappings(reqLogger logr.Logger) ([]InstallMapping, error) {
	// Fetch the mdefKVStore instance
	cm := &corev1.ConfigMap{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.METERDEF_INSTALL_MAP_NAME, Namespace: r.cfg.DeployedNamespace}, cm)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, err
		}

		reqLogger.Error(err, "Failed to get MeterdefintionConfigMap")
		return nil, err
	}

	cmMdefStore := cm.Data["meterdefinitionStore"]

	meterdefStore := &MeterdefinitionStore{}

	err = json.Unmarshal([]byte(cmMdefStore), meterdefStore)
	if err != nil {
		reqLogger.Error(err, "error unmarshaling meterdefinition store")
		return nil, err
	}

	return meterdefStore.InstallMappings, nil
}

func (r *MeterdefConfigMapReconciler) sync(reqLogger logr.Logger) *ExecResult {

	reqLogger.Info("syncing meterdefinitions")

	installMappings, err := r.getMeterdefInstallMappings(reqLogger)
	if err != nil {
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	reqLogger.Info("install mappings", "mapping", installMappings)

	// can we change this logic like this? - After we have the new folder structure for meter def repository inplace
	// for each packageName found in the installmappings list, call the file server with packagename and the csv version
	// of the package/packageName and get a list of meterdefinitions

	// Now prepare two lists
	// A ---> list of meter definitions for the (packageName and csv version) combo found in the cluster
	// B ---> list of meterdefinitions returned from file server for the (packageName and csv version) combo

	// Find the difference A - B ---> delete these meter defs
	// Find the difference B - A ----> Create these new meter definitions
	// Find the intersection of A & B ----> for these items, comapre the existing meter def
	// with what we got from file server and update it if the meter def from file server has changed
	for _, installMap := range installMappings {
		for _, installedMeterdefName := range installMap.InstalledMeterdefinitions {
			meterdefinitionFromCatalog, err := getMeterdefintionFromFileServer(installMap.PackageName, installMap.Namespace, installMap.VersionRangeDir, installedMeterdefName, reqLogger)
			if err != nil {
				return &ExecResult{
					ReconcileResult: reconcile.Result{},
					Err:             err,
				}
			}

			reqLogger.Info("meterdefintions returned from file server", installMap.PackageName, meterdefinitionFromCatalog.Annotations)

			meterdefFromCluster := &marketplacev1beta1.MeterDefinition{}

			// Check if the meterdef is on the cluster already
			err = r.Client.Get(context.TODO(), types.NamespacedName{Name: installedMeterdefName, Namespace: installMap.Namespace}, meterdefFromCluster)
			if err != nil {
				if errors.IsNotFound(err) {
					reqLogger.Info("meterdefinition not found", "meterdef name", meterdefFromCluster.Name)
					// meterdefinitionFromCatalog, err := getMeterdefintionFromFileServer(installMap.PackageName,installMap.VersionRangeDir,installedMeterdefName,reqLogger)
					if err != nil {
						return &ExecResult{
							ReconcileResult: reconcile.Result{},
							Err:             err,
						}
					}

					err = r.Client.Create(context.TODO(), meterdefinitionFromCatalog)
					if err != nil {
						reqLogger.Error(err, "Could not create MeterDefinition", "mdef", meterdefinitionFromCatalog.Name)
						return &ExecResult{
							ReconcileResult: reconcile.Result{},
							Err:             err,
						}
					}

					reqLogger.Info("Created meterdefinition", "mdef", meterdefinitionFromCatalog.Name)

					return &ExecResult{
						ReconcileResult: reconcile.Result{Requeue: true},
						Err:             nil,
					}
				}

				reqLogger.Error(err, "Failed to get meterdefinition")
				return &ExecResult{
					ReconcileResult: reconcile.Result{},
					Err:             err,
				}
			}

			reqLogger.Info("found installed meterdefinition on cluster", "name", meterdefFromCluster.Name)

			updatedMeterdefinition := meterdefFromCluster.DeepCopy()
			updatedMeterdefinition.Spec = meterdefinitionFromCatalog.Spec
			updatedMeterdefinition.ObjectMeta.Annotations = meterdefinitionFromCatalog.ObjectMeta.Annotations

			if !reflect.DeepEqual(updatedMeterdefinition, meterdefFromCluster) {
				reqLogger.Info("meterdefintion is out of sync with latest meterdef catalog", "name", meterdefFromCluster.Name)
				err = r.Client.Update(context.TODO(), updatedMeterdefinition)
				if err != nil {
					reqLogger.Error(err, "Failed to update meterdefinition", "name", meterdefFromCluster.Name)
					return &ExecResult{
						ReconcileResult: reconcile.Result{Requeue: true},
						Err:             err,
					}
				}
				reqLogger.Info("Updated meterdefintion", "meterdef name", meterdefFromCluster.Name)
			}

		}
	}

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func getMeterdefintionFromFileServer(packageName string, namespace string, versionRange string, mdefName string, reqLogger logr.Logger) (*marketplacev1beta1.MeterDefinition, error) {

	mdefFileName := fmt.Sprintf("%s.yaml", mdefName)
	url := fmt.Sprintf("http://rhm-meterdefinition-file-server.openshift-redhat-marketplace.svc.cluster.local:8100/get/%s/%s/%s", packageName, versionRange, mdefFileName)
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
