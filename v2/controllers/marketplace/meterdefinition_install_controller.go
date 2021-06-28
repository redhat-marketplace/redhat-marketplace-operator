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
	"crypto/tls"
	"crypto/x509"
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
	prom "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
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
	kubeInterface kubernetes.Interface
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
			// Request object not found, check the meterdef store if there is an existing InstallMapping,delete, and return empty result
			// If no install mapping found return empty result 
			_csvName := request.Name
			reqLogger.Info("clusterserviceversion does not exist,checking install map for package","name",_csvName)
			result := deleteInstallMapping(_csvName,r.cfg.DeployedNamespace ,request, r.Client, reqLogger)
			if !result.Is(Continue) {
				if result.Is(Error) {
					reqLogger.Error(result.GetError(), "Failed deleting install mapping from meter definition store")
				}
				return result.Return()
			}

		}

		reqLogger.Error(err, "Failed to get clusterserviceversion")
		return reconcile.Result{}, err
	}

	csvPackageName, csvVersion, result := parsePackageNameAndVersion(CSV, request, reqLogger)
	if !result.Is(Continue) {
		result.Return()
	}

	// handle CSV updates: change in CSV version might change applicable meter definitions
	if CSV.Status.Phase == olmv1alpha1.CSVPhaseReplacing {
		/*
			Easy flow: Delete all existing meter definitions for the operator and
			install fresh set set of meterdefinitions for the current version of the CSV
		*/

		//TODO: pseudo code, needs testing in a real update scenario
		mdefStore,result := getMeterdefStoreFromCM(r.Client,r.cfg.DeployedNamespace,reqLogger)
		if !result.Is(Continue) {
			if result.Is(Error) {
				reqLogger.Error(result.GetError(), "Failed getting meterdefstore")
			}
			return result.Return()
		}

		// delete meterdefs for previous version
		for _, installMap := range mdefStore.InstallMappings {
			namespace := installMap.Namespace
			installedMeterDefs := installMap.InstalledMeterdefinitions

			if installMap.PackageName == csvPackageName {
				err := deleteMeterDefintions(namespace, installedMeterDefs, r.Client, reqLogger)
				if err != nil {
					return reconcile.Result{},err
				}
				reqLogger.Info("Successfully deleted meterdefintions", "Count", len(installedMeterDefs), "Namespace", namespace, "CSV", csvPackageName)
			}
		}

		// install appropriate meterdefs for new version
		meterDefNamesFromFileServer,meterDefsFromFileServer, result := ListMeterdefintionsFromFileServer(csvPackageName, csvVersion, CSV.Namespace, reqLogger)
		if !result.Is(Continue) {
			if result.Is(Error) {
				reqLogger.Error(result.GetError(), "Failed retrieving meterdefinitions from file server")
			}
			return result.Return()
		}

		meterDefsMapFromFileServer := make(map[string]marketplacev1beta1.MeterDefinition)

		for _, meterDefItem := range meterDefsFromFileServer {
			meterDefsMapFromFileServer[meterDefItem.ObjectMeta.Name] = meterDefItem
		}

		err := createMeterDefintions(r.Scheme,r.Client,CSV.Namespace, csvPackageName, meterDefNamesFromFileServer, meterDefsMapFromFileServer, reqLogger)
		if err != nil {
			return reconcile.Result{},err
		}

		reqLogger.Info("Successfully created meterdefintions", "Count", len(meterDefNamesFromFileServer), "Namespace", CSV.Namespace, "CSV", csvPackageName)
		return reconcile.Result{Requeue: true},nil

	}

	/* New CSV install */
	sub := &olmv1alpha1.SubscriptionList{}
	if err := r.Client.List(context.TODO(), sub, client.InNamespace(request.NamespacedName.Namespace)); err != nil {
		return reconcile.Result{}, err
	}

	if len(sub.Items) > 0 {
		reqLogger.Info("found Subscription in namespaces", "count", len(sub.Items))

		// apply meter definition if subscription has rhm/operator label
		for _, subscription := range sub.Items {
			if value, ok := subscription.GetLabels()[operatorTag]; ok {
				if value == "true" {
					if len(subscription.Status.InstalledCSV) == 0 {
						reqLogger.Info("Requeue clusterserviceversion to wait for subscription getting installedCSV updated")
						return reconcile.Result{RequeueAfter: time.Second * 5}, nil
					}

					if subscription.Status.InstalledCSV == request.NamespacedName.Name {
						reqLogger.Info("found Subscription with installed CSV")

						_,selectedMeterDefinitions, result := ListMeterdefintionsFromFileServer(csvPackageName, csvVersion, CSV.Namespace, reqLogger)
						if !result.Is(Continue) {

							if result.Is(Error) {
								reqLogger.Error(result.GetError(), "Failed retrieving meterdefinitions from file server")
							}

							return result.Return()
						}

						gvk, err := apiutil.GVKForObject(CSV, r.Scheme)
						if err != nil {
							return reconcile.Result{}, err
						}

						// create meter definitions
						for _, meterDefItem := range selectedMeterDefinitions {

							// create owner ref object
							ref := metav1.OwnerReference{
								APIVersion:         gvk.GroupVersion().String(),
								Kind:               gvk.Kind,
								Name:               CSV.GetName(),
								UID:                CSV.GetUID(),
								BlockOwnerDeletion: pointer.BoolPtr(false),
								Controller:         pointer.BoolPtr(false),
							}

							meterDefItem.ObjectMeta.SetOwnerReferences([]metav1.OwnerReference{ref})
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

									reqLogger.Info("Created meterdefinition", "mdef", meterDefItem.Name)

									result = r.setInstalledMeterdefinition(meterDefItem.GetAnnotations()["packageName"], CSV.Name, csvVersion, meterDefItem.Name, request, reqLogger)
									if !result.Is(Continue) {

										if result.Is(Error) {
											reqLogger.Error(result.GetError(), "Failed to update meterdef store")
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
		reqLogger.Info("Subscriptions not found in the namespace")
		return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
	}

	reqLogger.Info("reconcilation complete")
	return reconcile.Result{}, nil
}

func reconcileCSV(metaNew metav1.Object) bool {
	ann := metaNew.GetAnnotations()

	ignoreVal, hasIgnoreTag := ann[ignoreTag]

	// we need to pick up the csv
	if !hasIgnoreTag || ignoreVal != ignoreTagValue {
		return true
	}

	//ignore
	return false
}

func parsePackageNameAndVersion(csv *olmv1alpha1.ClusterServiceVersion, request reconcile.Request, reqLogger logr.Logger) (packageName string, version string, result *ExecResult) {

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

func ListMeterdefintionsFromFileServer(packageName string, version string, namespace string, reqLogger logr.Logger) ([]string,[]marketplacev1beta1.MeterDefinition, *ExecResult) {

	// returns all the meterdefinitions for associated with a particular CSV version
	url := fmt.Sprintf("http://rhm-meterdefinition-file-server.openshift-redhat-marketplace.svc.cluster.local:8100/list-for-version/%s/%s", packageName, version)
	response, err := http.Get(url)
	if err != nil {
		if err == io.EOF {
			reqLogger.Error(err, "Meterdefintion not found")
			return nil,nil, &ExecResult{
				ReconcileResult: reconcile.Result{},
				Err:             emperror.New("empty response"),
			}
		}

		reqLogger.Error(err, "Error querying file server")
		return nil,nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	mdefSlice := []marketplacev1beta1.MeterDefinition{}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil,nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	if len(data) == 0 {
		reqLogger.Error(err, "no data in response")
		return nil,nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	meterDefsData := strings.Replace(string(data), "<<NAMESPACE-PLACEHOLDER>>", namespace, -1)
	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(meterDefsData)), 100).Decode(&mdefSlice)
	if err != nil {
		reqLogger.Error(err, "error decoding meterdefstore string")
		return nil,nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	var meterDefNames []string
	for _, meterDefItem := range mdefSlice {
		meterDefNames = append(meterDefNames, meterDefItem.ObjectMeta.Name)
	}

	reqLogger.Info("meterdefintions returned from file server",packageName,meterDefNames)
	
	return meterDefNames,mdefSlice, &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func (r *MeterdefinitionInstallReconciler) setInstalledMeterdefinition(packageName string, csvName string, csvVersion string, installedMeterdef string, request reconcile.Request, reqLogger logr.InfoLogger) *ExecResult {

	// Fetch the mdefStoreCM instance
	mdefStoreCM := &corev1.ConfigMap{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.METERDEF_INSTALL_MAP_NAME, Namespace: r.cfg.DeployedNamespace}, mdefStoreCM)
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
	updatedStore, err := addOrUpdateInstallList(packageName, csvName, csvVersion, installedMeterdef, mdefStore, request, reqLogger)
	if err != nil {
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	mdefStoreCM.Data["meterdefinitionStore"] = updatedStore

	err = r.Client.Update(context.TODO(), mdefStoreCM)
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
func addOrUpdateInstallList(packageName string, csvName string, csvVersion string, installedMeterdef string, cmMdefStore string, request reconcile.Request, reqLogger logr.Logger) (string, error) {

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
			CsvName:                   csvName,
			CsvVersion:                csvVersion,
			InstalledMeterdefinitions: []string{installedMeterdef},
		}

		meterdefStore.InstallMappings = append(meterdefStore.InstallMappings, newInstallMapping)
	}

	out, err := json.Marshal(meterdefStore)
	if err != nil {
		return "", err
	}

	meterdefStoreJSON := string(out)
	return meterdefStoreJSON, nil
}

// deletes install mapping for (packageName + namespace) combination
func deleteInstallMapping(csvName string,deployedNamespace string ,request reconcile.Request, client client.Client, reqLogger logr.Logger) *ExecResult {
	
	// Fetch the mdefStoreCM instance
	mdefStoreCM := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: utils.METERDEF_INSTALL_MAP_NAME, Namespace: deployedNamespace}, mdefStoreCM)
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

	var installMappingFound bool
	for i, item := range meterdefStore.InstallMappings {
		// remove the package's InstallMapping. Need to search by csvName here that's what is returned in the request
		if item.CsvName == csvName && item.Namespace == request.Namespace {
			reqLogger.Info("deletemeterdefinitions()","install map found",item)
			installMappingFound = true
			meterdefStore.InstallMappings = append(meterdefStore.InstallMappings[:i], meterdefStore.InstallMappings[i+1:]...)
			break
		}
	}

	if !installMappingFound {
		reqLogger.Info("delete install mapping","no install mapping found for package, ignoring",csvName)
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             nil,
		}
	}

	out, err := json.Marshal(meterdefStore)
	if err != nil {
		reqLogger.Error(err, "error marshaling meterdefinition store")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	meterdefStoreJSON := string(out)
	mdefStoreCM.Data["meterdefinitionStore"] = meterdefStoreJSON

	err = client.Update(context.TODO(), mdefStoreCM)
	if err != nil {
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	reqLogger.Info("deleted install mapping","successfully deleted install map for package, exiting reconcile",csvName)
	return &ExecResult{
		ReconcileResult: reconcile.Result{},
		Err:             nil,
	}
}

func (r *MeterdefinitionInstallReconciler) getFileServerService (reqLogger logr.InfoLogger) (*corev1.Service,error){
	service := &corev1.Service{}

	err := r.Client.Get(context.TODO(),types.NamespacedName{Namespace: r.cfg.DeployedNamespace,Name: "rhm-meterdefinition-file-server"},service)
	if err != nil {
		return nil, err
	}

	return service,nil
}

func (r *MeterdefinitionInstallReconciler) getCertFromConfigMap(reqLogger logr.Logger)([]byte,error){
	cm := &corev1.ConfigMap{}
	err := r.Client.Get(context.TODO(),types.NamespacedName{Namespace: r.cfg.DeployedNamespace,Name: "serving-certs-ca-bundle"},cm)
	if err != nil {
		return nil, err
	}

	reqLogger.Info("extracting cert from config map")

	out, ok := cm.Data["service-ca.crt"]

	if !ok {
		err = emperror.New("Error retrieving cert from config map")
		return nil, err
	}

	cert := []byte(out)
	return cert, nil
	
}

func (r *MeterdefinitionInstallReconciler) createTlsConfig(reqLogger logr.Logger) (*tls.Config,*ExecResult) {
	service, err := r.getFileServerService(reqLogger)
	if err != nil {
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	cert, err := r.getCertFromConfigMap(reqLogger)
	if err != nil {
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}
	saClient := prom.NewServiceAccountClient(r.cfg.DeployedNamespace, r.kubeInterface)
	authToken, err := saClient.NewServiceAccountToken(utils.OPERATOR_SERVICE_ACCOUNT, utils.PrometheusAudience, 3600, reqLogger)
	if err != nil {
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}


	caCertPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get system cert pool")
	}

	ok := caCertPool.AppendCertsFromPEM(caCert)
	if !ok {
		err = emperror.New("failed to append cert to cert pool")
		reqLogger.Error(err, "cert pool error")
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	return &tls.Config{
		RootCAs: caCertPool,
	},  &ExecResult{
		Status: ActionResultStatus(Continue),
	}


	// prometheusAPI, err := NewPromAPI(service, &cert, authToken)
	// if err != nil {
	// 	return nil, err
	// }
	// return prometheusAPI, nil
	
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
		if !reconcileCSV(e.MetaNew) || !reconcileCSV(e.MetaOld){
			return false
		}

		oldCSV, ok := e.ObjectOld.(*olmv1alpha1.ClusterServiceVersion)
		if !ok {
			return false
		}

		// Limiting update operation to only handle this for now
		// TODO: does this need to use the new object ? 
		if oldCSV.Status.Phase == olmv1alpha1.CSVPhaseReplacing {
			return true
		}

		return false
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return reconcileCSV(e.Meta)
	},
	CreateFunc: func(e event.CreateEvent) bool {
		return reconcileCSV(e.Meta) 

	},
	GenericFunc: func(e event.GenericEvent) bool {
		return reconcileCSV(e.Meta) 
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

func (r *MeterdefinitionInstallReconciler) InjectKubeInterface(k kubernetes.Interface) error {
	r.kubeInterface = k
	return nil
}

func (r *MeterdefinitionInstallReconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&olmv1alpha1.ClusterServiceVersion{}, builder.WithPredicates(rhmCSVControllerPredicates)).
		Complete(r)
}
