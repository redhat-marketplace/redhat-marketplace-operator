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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	emperror "emperror.dev/errors"
	"github.com/go-logr/logr"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
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
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	csvProp      string = "operatorframework.io/properties"
	versionRange string = "versionRange"
	packageName  string = "packageName"
	startVersion string = "startVersion"
)

// blank assignment to verify that ReconcileClusterServiceVersion implements reconcile.Reconciler
var _ reconcile.Reconciler = &MeterdefinitionInstallReconciler{}

// MeterdefinitionInstallReconciler reconciles a ClusterServiceVersion object
type MeterdefinitionInstallReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	cfg    *config.OperatorConfig
}

// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions;subscriptions,verbs=get;list;watch
// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions,verbs=update;patch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions;meterdefinitions/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions;meterdefinitions/status,verbs=get;list;watch;create;update;patch;delete

// Reconcile reads that state of the cluster for a ClusterServiceVersion object and creates corresponding meter definitions if found
func (r *MeterdefinitionInstallReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Name", request.Name, "Request.Namespace", request.Namespace)
	reqLogger.Info("Reconciling ClusterServiceVersion")

	// Fetch the ClusterServiceVersion instance
	CSV := &olmv1alpha1.ClusterServiceVersion{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, CSV)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Error(err, "clusterserviceversion does not exist")
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get clusterserviceversion")
		return reconcile.Result{}, err
	}

	sub := &olmv1alpha1.SubscriptionList{}
	if err := r.Client.List(context.TODO(), sub, client.InNamespace(request.NamespacedName.Namespace)); err != nil {
		return reconcile.Result{}, err
	}

	if len(sub.Items) > 0 {
		reqLogger.V(4).Info("found Subscription in namespaces", "count", len(sub.Items))

		// apply meter definition if subscription has rhm/operator label
		for _, s := range sub.Items {
			if value, ok := s.GetLabels()[operatorTag]; ok {
				if value == "true" {

					csvPackageName, csvVersion, result := getPackageNameAndVersion(CSV, request, reqLogger)
					if !result.Is(Continue) {
						result.Return()
					}

					if len(s.Status.InstalledCSV) == 0 {
						reqLogger.Info("Requeue clusterserviceversion to wait for subscription getting installedCSV updated")
						return reconcile.Result{RequeueAfter: time.Second * 5}, nil
					}

					if CSV.Status.Phase == olmv1alpha1.CSVPhaseDeleting {
						// deleting meterdefinitions will be automatically handled as we have set owner references during its creating
						// clean up install list
						result := deleteInstallMapping(csvPackageName, request, r.Client, reqLogger)
						if !result.Is(Continue) {
							if result.Is(Error) {
								reqLogger.Error(result.GetError(), "Failed retrieving meterdefinitions from file server")
							}
							return result.Return()
						}
					}

					// handle CSV updates: change in CSV version might change applicable meter definitions
					if CSV.Status.Phase == olmv1alpha1.CSVPhaseReplacing {
						/*
							Easy flow: Delete all existing meter definitions for the operator and
							install fresh set set of meterdefinitions for the current version of the CSV
						*/
					}

					if s.Status.InstalledCSV == request.NamespacedName.Name {
						reqLogger.Info("found Subscription with installed CSV")

						selectedMeterDefinitions, result := listMeterdefintionsFromFileServer(csvPackageName, csvVersion, CSV.Namespace, reqLogger)
						if !result.Is(Continue) {

							if result.Is(Error) {
								reqLogger.Error(result.GetError(), "Failed retrieving meterdefinitions from file server")
							}

							return result.Return()
						}

						// var installedMeterdefintions []string

						// create meter definitions
						for _, meterDefItem := range selectedMeterDefinitions {
							gvk, err := apiutil.GVKForObject(CSV, r.Scheme)
							if err != nil {
								return reconcile.Result{}, err
							}

							ref := metav1.OwnerReference{
								APIVersion:         gvk.GroupVersion().String(),
								Kind:               gvk.Kind,
								Name:               CSV.GetName(),
								UID:                CSV.GetUID(),
								BlockOwnerDeletion: pointer.BoolPtr(false),
								Controller:         pointer.BoolPtr(false),
							}

							ownerRef := meterDefItem.ObjectMeta.OwnerReferences
							if ownerRef != nil {
								meterDefItem.ObjectMeta.OwnerReferences = append(meterDefItem.ObjectMeta.OwnerReferences, ref)
							} else {
								meterDefItem.ObjectMeta.SetOwnerReferences([]metav1.OwnerReference{ref})
							}

							meterDefItem.ObjectMeta.Namespace = CSV.Namespace

							// Check if the meterdef is on the cluster already
							meterdef := &marketplacev1beta1.MeterDefinition{}
							err = r.Client.Get(context.TODO(), types.NamespacedName{Name: meterDefItem.Name, Namespace: request.Namespace}, meterdef)
							if err != nil {
								if errors.IsNotFound(err) {

									reqLogger.Info("meterdefinition not found,creating", "meterdef name", meterdef.Name)
									err = r.Client.Create(context.TODO(), &meterDefItem)
									if err != nil {
										reqLogger.Error(err, "Could not create MeterDefinition", "mdef", &meterDefItem.Name)
										return reconcile.Result{}, err
									}

									reqLogger.Info("Created meterdefinition", "mdef", &meterDefItem.Name)

									t := meterDefItem.GetAnnotations()["versionRange"]
									vr := strings.Split(t, "-")
									begin := strings.TrimSpace(vr[0])
									end := strings.TrimSpace(vr[1])
									versionRangeDir := fmt.Sprintf("%s-%s", begin, end)

									result = r.setInstalledMeterdefinition(meterDefItem.GetAnnotations()["packageName"], csvVersion, versionRangeDir, meterDefItem.Name, request, reqLogger)
									if !result.Is(Continue) {

										if result.Is(Error) {
											reqLogger.Error(result.GetError(), "Failed to create meterdef store.")
										}

										return result.Return()
									}

									return reconcile.Result{Requeue: true}, nil
								}

								reqLogger.Error(err, "Failed to get meterdefinition")
								return reconcile.Result{}, err
							}
						}
					}
				}
			}
		}
	} else {
		reqLogger.Info("Did not find Subscription in namespaces")
	}

	reqLogger.Info("reconcilation complete")
	return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
}

func _csvFilter(metaNew metav1.Object) int {
	ann := metaNew.GetAnnotations()

	//annotation values
	ignoreVal, hasIgnoreTag := ann[ignoreTag]

	if !hasIgnoreTag || ignoreVal != ignoreTagValue {
		return 1
	}
	return 0
}

func getPackageNameAndVersion(csv *olmv1alpha1.ClusterServiceVersion, request reconcile.Request, reqLogger logr.Logger) (packageName string, version string, result *ExecResult) {

	v, ok := csv.GetAnnotations()[csvProp]
	if !ok {
		err := emperror.New("could not find annotations for CSV properties")
		reqLogger.Error(err, request.Name)
		return "", "", &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	csvProperties := fetchCSVInfo(v)

	_packageName := csvProperties["packageName"]
	packageName, ok = _packageName.(string)
	if !ok {
		err := emperror.New("TYPE CONVERSION ERROR PACKAGE NAME")
		reqLogger.Error(err, request.Name)
		return "", "", &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	_version := csvProperties["version"]
	version, ok = _version.(string)
	if !ok {
		err := emperror.New("type conversion error on versions")
		reqLogger.Error(err, request.Name)
		return "", "", &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	if strings.Contains(version, "-") {
		version = strings.Split(version, "-")[0]
	}

	return packageName, version, &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func listMeterdefintionsFromFileServer(packageName string, version string, namespace string, reqLogger logr.Logger) ([]marketplacev1beta1.MeterDefinition, *ExecResult) {

	// var selectedMeterDefinitions []marketplacev1beta1.MeterDefinition

	// returns all the meterdefinitions for associated with a particular CSV version
	url := fmt.Sprintf("http://rhm-meterdefinition-file-server.openshift-redhat-marketplace.svc.cluster.local:8100/list-for-version/%s/%s", packageName, version)
	response, err := http.Get(url)
	if err != nil {
		if err == io.EOF {
			reqLogger.Error(err, "Meterdefintion not found")
			return nil, &ExecResult{
				ReconcileResult: reconcile.Result{},
				Err:             emperror.New("empty response"),
			}
		}

		reqLogger.Error(err, "Error querying file server")
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	mdefSlice := []marketplacev1beta1.MeterDefinition{}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	if len(data) == 0 {
		reqLogger.Error(err, "no data in response")
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	meterDefsData := strings.Replace(string(data), "<<NAMESPACE-PLACEHOLDER>>", namespace, -1)
	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(meterDefsData)), 100).Decode(&mdefSlice)
	if err != nil {
		reqLogger.Error(err, "error decoding meterdefstore string")
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	reqLogger.Info("meterdefintions returned from file server", packageName, mdefSlice)

	return mdefSlice, &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func (r *MeterdefinitionInstallReconciler) setInstalledMeterdefinition(packageName string, csvVersion string, versionRange string, installedMeterdef string, request reconcile.Request, reqLogger logr.InfoLogger) *ExecResult {

	// Fetch the mdefKVStore instance
	mdefKVStoreCM := &corev1.ConfigMap{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.METERDEF_INSTALL_MAP_NAME, Namespace: r.cfg.DeployedNamespace}, mdefKVStoreCM)
	if err != nil {
		if errors.IsNotFound(err) {
			return &ExecResult{
				ReconcileResult: reconcile.Result{},
				Err:             err,
			}

		}

		reqLogger.Error(err, "Failed to get MeterdefintionConfigMap")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	mdefStore := mdefKVStoreCM.Data["meterdefinitionStore"]
	updatedStore, err := addOrUpdateInstallList(packageName, csvVersion, versionRange, installedMeterdef, mdefStore, request, reqLogger)
	if err != nil {
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	mdefKVStoreCM.Data["meterdefinitionStore"] = updatedStore

	err = r.Client.Update(context.TODO(), mdefKVStoreCM)
	if err != nil {
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	//set installed meterdefinition on cm
	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

// string of meterdefinitions > add json to ./json-store
// adds a new InstalledMeterdefinition to the json store
func addOrUpdateInstallList(packageName string, csvVersion string, versionRange string, installedMeterdef string, cmMdefStore string, request reconcile.Request, reqLogger logr.Logger) (string, error) {

	meterdefStore := &MeterdefinitionStore{}

	err := json.Unmarshal([]byte(cmMdefStore), meterdefStore)
	if err != nil {
		reqLogger.Error(err, "error unmarshaling meterdefinition store")
		return "", err
	}

	// if the install mapping already exists update it
	var installMappingFound bool
	for i, item := range meterdefStore.InstallMappings {
		// update a certain package's meterdefinition install list
		if item.PackageName == packageName && item.Namespace == request.Namespace {
			installMappingFound = true
			reqLogger.Info("found existing meterdefinition mapping", "mapping", item)
			// if there are missing meterdefinitions from the list add them
			if !utils.Contains(item.InstalledMeterdefinitions, installedMeterdef) {
				reqLogger.Info("updating installed meterdef list", "updates", installedMeterdef)
				meterdefStore.InstallMappings[i].InstalledMeterdefinitions = append(meterdefStore.InstallMappings[i].InstalledMeterdefinitions, installedMeterdef)
			}
		}
	}

	// if the install mapping isn't found add it
	if !installMappingFound {
		reqLogger.Info("no meterdef mapping found, adding")
		newInstallMapping := InstallMapping{
			PackageName:               packageName,
			Namespace:                 request.Namespace,
			CsvVersion:                csvVersion,
			InstalledMeterdefinitions: []string{installedMeterdef},
		}

		meterdefStore.InstallMappings = append(meterdefStore.InstallMappings, newInstallMapping)
	}

	out, err := json.Marshal(meterdefStore)
	if err != nil {
		return "", err
	}

	meterdefStoreJson := string(out)
	return meterdefStoreJson, nil
}

// deletes install mapping for (packageName + namespace) combination
func deleteInstallMapping(packageName string, request reconcile.Request, client client.Client, reqLogger logr.Logger) *ExecResult {

	// Fetch the mdefKVStore instance
	mdefStoreCM := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: utils.METERDEF_INSTALL_MAP_NAME, Namespace: request.Namespace}, mdefStoreCM)
	if err != nil {
		if errors.IsNotFound(err) {
			return &ExecResult{
				ReconcileResult: reconcile.Result{},
				Err:             err,
			}

		}

		reqLogger.Error(err, "Failed to get MeterdefintionConfigMap")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	mdefStore := mdefStoreCM.Data["meterdefinitionStore"]

	meterdefStore := &MeterdefinitionStore{}

	err = json.Unmarshal([]byte(mdefStore), meterdefStore)
	if err != nil {
		reqLogger.Error(err, "error unmarshaling meterdefinition store")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	for i, item := range meterdefStore.InstallMappings {
		// remove the package's InstallMapping
		if item.PackageName == packageName && item.Namespace == request.Namespace {
			meterdefStore.InstallMappings = append(meterdefStore.InstallMappings[:i], meterdefStore.InstallMappings[i+1:]...)
			break
		}
	}

	err = client.Update(context.TODO(), mdefStoreCM)
	if err != nil {
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}



func fetchCSVInfo(csvProps string) map[string]interface{} {
	var unmarshalledProps map[string]interface{}
	json.Unmarshal([]byte(csvProps), &unmarshalledProps)

	properties := unmarshalledProps["properties"].([]interface{})
	reqProperty := properties[len(properties)-1]

	csvProperty := reqProperty.(map[string]interface{})

	return csvProperty["value"].(map[string]interface{})
}

var rhmCSVControllerPredicates predicate.Funcs = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {

		oldCSV, ok := e.ObjectOld.(*olmv1alpha1.ClusterServiceVersion)
		if !ok {
			return false
		}

		newCSV, ok := e.ObjectNew.(*olmv1alpha1.ClusterServiceVersion)
		if !ok {
			return false
		}

		if oldCSV.Status.Phase != newCSV.Status.Phase {
			fmt.Println("NEW CSV STATUS PHASE", newCSV.Name, newCSV.Status.Phase)
			return true
		}

		return false
	},
	DeleteFunc: func(evt event.DeleteEvent) bool {
		return true
	},
	CreateFunc: func(evt event.CreateEvent) bool {
		return csvFilter(evt.Meta) > 0
	},
	GenericFunc: func(evt event.GenericEvent) bool {
		return false
	},
}

func (r *MeterdefinitionInstallReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (m *MeterdefinitionInstallReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	m.cfg = cfg
	return nil
}

func (r *MeterdefinitionInstallReconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&olmv1alpha1.ClusterServiceVersion{}, builder.WithPredicates(rhmCSVControllerPredicates)).
		Complete(r)
}
