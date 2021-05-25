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
	"bytes"
	"context"
	"sync"
	"time"

	// "archive/tar"

	emperror "emperror.dev/errors"
	semver "github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"

	// "golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

var GlobalMeterdefStoreDB = &MeterdefStoreDB{}
// MeterdefConfigMapReconciler reconciles the DataService of a MeterBase object
type MeterdefConfigMapReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	CC     ClientCommandRunner

	cfg     *config.OperatorConfig
	factory *manifests.Factory
	patcher patch.Patcher
}

type MeterdefStoreDB struct {
	sync.Mutex
	Meterdefinitions []marketplacev1beta1.MeterDefinition
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
	For(&corev1.ConfigMap{}, builder.WithPredicates(
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {

				if e.Meta.GetName() == utils.METERDEF_STORE_NAME {
					return true
				}
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				if e.MetaNew.GetName() == utils.METERDEF_STORE_NAME {
					return e.MetaOld.GetResourceVersion() != e.MetaNew.GetResourceVersion()
				}
				
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				if e.Meta.GetName() == utils.METERDEF_STORE_NAME {
					return true
				}
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {

				if e.Meta.GetName() == utils.METERDEF_STORE_NAME {
					return true
				}
				return false
			},
		},
	)).
	Complete(r)

}

// Reconcile reads that state of the cluster for a MeterdefConfigmap object and makes changes based on the state read
// and what is in the MeterdefConfigmap.Spec
func (r *MeterdefConfigMapReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("reconciling meterdef configmap")

	// // Fetch the mdefKVStore instance
	// mdefKVStore := &corev1.ConfigMap{}
	// err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.METERDEF_STORE_NAME,Namespace: request.Namespace}, mdefKVStore)
	// if err != nil {
	// 	if errors.IsNotFound(err) {

	// 		reqLogger.Info("meterdef store not found, creating")

	// 		result := createMeterdefStore(r.factory,r.Client,reqLogger)
	// 		// result := createMeterdefStore(r.Client,reqLogger)
	// 		if !result.Is(Continue) {
				
	// 			if result.Is(Error) {
	// 				reqLogger.Error(result.GetError(), "Failed to create meterdef store.")
	// 			}
		
	// 			return result.Return()
	// 		}

	// 		return reconcile.Result{Requeue: true}, nil
	// 	}

	// 	reqLogger.Error(err, "Failed to get MeterdefintionConfigMap")
	// 	return reconcile.Result{}, err
	// }


	// result := GlobalMeterdefStoreDB.Populate(mdefKVStore,reqLogger)
	// if !result.Is(Continue) {
				
	// 	if result.Is(Error) {
	// 		reqLogger.Error(result.GetError(), "Failed to create meterdef store.")
	// 	}

	// 	return result.Return()
	// }

	// utils.PrettyPrint(GlobalMeterdefStoreDB)

	// mdef, result := pollQuay(reqLogger)
	// if !result.Is(Continue) {
				
	// 	if result.Is(Error) {
	// 		reqLogger.Error(result.GetError(), "Failed to create meterdef store.")
	// 	}

	// 	return result.Return()
	// }

	// utils.PrettyPrint(mdef)

	// response, err := http.Get("http://meterdef-file-server.openshift-redhat-marketplace.svc.cluster.local:8100/get/meterdef-1")
    // if err != nil {
    //     fmt.Printf("HTTP ERROR %s\n", err)
    // } 

	// mdef := &marketplacev1beta1.MeterDefinition{}
	// data, _ := ioutil.ReadAll(response.Body)
	// fmt.Printf("RESPONSE DATA %s",string(data))
	// err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(string(data))), 100).Decode(&mdef)
	// if err != nil {
	// 	reqLogger.Error(err,"error decoding meterdefstore string")
	// 	return reconcile.Result{},err
	// }

	// utils.PrettyPrint(mdef)

	reqLogger.Info("finished reconciling")
	return reconcile.Result{RequeueAfter: time.Second * 10}, nil
}

func(m *MeterdefStoreDB) Populate(kvStore *corev1.ConfigMap, reqLogger logr.Logger) *ExecResult{
	meterdefList := &[]marketplacev1beta1.MeterDefinition{}
	meterdefStoreString := kvStore.Data["meterdefinitions"]
	if len(meterdefStoreString) == 0 {
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err: emperror.New("no meterdefinitions in meterdef store"),
		}
	}
	err := yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(meterdefStoreString)), 100).Decode(&meterdefList)
	if err != nil {
		reqLogger.Error(err,"error decoding meterdefstore string")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err: err,
		}
	}

	m.Lock()
	defer m.Unlock()

	m.Meterdefinitions = *meterdefList

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func(m *MeterdefStoreDB) GetMeterdefinitionsForPackage(packageName string) []marketplacev1beta1.MeterDefinition {
	m.Lock()
	defer m.Unlock()

	mdefList := []marketplacev1beta1.MeterDefinition{}

	for _,mdef  := range m.Meterdefinitions {
		if mdef.Annotations["packageName"] == packageName {
			mdefList = append(mdefList, mdef)
		}
	}

	return mdefList
}

func(m *MeterdefStoreDB) ListMeterdefinitions () []marketplacev1beta1.MeterDefinition {
	m.Lock()
	defer m.Unlock()
	return m.Meterdefinitions
}

func(m *MeterdefStoreDB) AddMeterdefinition (newMeterdefinition marketplacev1beta1.MeterDefinition) {
	m.Lock()
	defer m.Unlock()
	m.Meterdefinitions = append(m.Meterdefinitions, newMeterdefinition)
}

func(m *MeterdefStoreDB) GetVersionConstraints (packageName string) (constraint *semver.Constraints,returnErr error){
	
	for _,mdef  := range m.Meterdefinitions {
		if mdef.Annotations["packageName"] == packageName {
			versionRange := mdef.Annotations["versionRange"]
			constraint, returnErr = semver.NewConstraint(versionRange)
			if returnErr != nil {
				return nil,returnErr
			}
		}
	}

	return constraint,nil
}

// TODO: we can use thiss later
func createMeterdefStore(factory *manifests.Factory,client client.Client,reqLogger logr.Logger)*ExecResult{
	mdefConfigMap, err := factory.NewMeterdefinitionConfigMap()
	if err != nil {
	
		reqLogger.Error(err, "Failed to build MeterdefinitoinConfigMap")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err: err,
		}
	}	

	err = client.Create(context.Background(),mdefConfigMap)
	if err != nil {
		reqLogger.Error(err, "Failed to create MeterdefinitoinConfigMap")
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err: err,
		}
	}
	
	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

// func createMeterdefStore(client client.Client,reqLogger logr.Logger)(*ExecResult){
	
// 	mdefList := []marketplacev1beta1.MeterDefinition{	
// 		{
// 			ObjectMeta: metav1.ObjectMeta{
// 				Name:      "joget-meterdef",
// 				Namespace: "openshift-redhat-marketplace",
// 				Annotations: map[string]string{
// 					"versionRange": "0.0.1 - 1.4.5",
// 					"packageName" : "joget-dx-operator-rhmp",
// 				},
// 			},
// 			Spec: marketplacev1beta1.MeterDefinitionSpec{
// 				Group: "marketplace.redhat.com",
// 				Kind:  "Pod",

// 				ResourceFilters: []marketplacev1beta1.ResourceFilter{
// 					{
// 						WorkloadType: marketplacev1beta1.WorkloadTypePod,
// 						Label: &marketplacev1beta1.LabelFilter{
// 							LabelSelector: &metav1.LabelSelector{
// 								MatchLabels: map[string]string{
// 									"app.kubernetes.io/name": "rhm-metric-state",
// 								},
// 							},
// 						},
// 					},
// 				},
// 				Meters: []marketplacev1beta1.MeterWorkload{
// 					{
// 						Aggregation: "sum",
// 						Period: &metav1.Duration{
// 							Duration: time.Duration(time.Minute*15),
// 						},
// 						Query:        "kube_pod_info{} or on() vector(0)",
// 						Metric:       "meterdef_controller_test_query",
// 						WorkloadType: marketplacev1beta1.WorkloadTypePod,
// 						Name:         "meterdef_controller_test_query",
// 					},
// 				},
// 			},
// 		},
// 		{
// 			ObjectMeta: metav1.ObjectMeta{
// 				Name:      "couchbase-meterdef",
// 				Namespace: "openshift-redhat-marketplace",
// 				Annotations: map[string]string{
// 					"versionRange": "0.0.1 - 1.4.5",
// 					"packageName" : "joget-dx-operator-rhmp",
// 				},
// 			},
// 			Spec: marketplacev1beta1.MeterDefinitionSpec{
// 				Group: "marketplace.redhat.com",
// 				Kind:  "Pod",

// 				ResourceFilters: []marketplacev1beta1.ResourceFilter{
// 					{
// 						WorkloadType: marketplacev1beta1.WorkloadTypeService,
// 						OwnerCRD:  &marketplacev1beta1.OwnerCRDFilter{
// 							common.GroupVersionKind{
// 								APIVersion: "couchbase.com/v2",
// 								Kind: "CouchbaseCluster",
// 							},
// 						},
// 						Namespace: &marketplacev1beta1.NamespaceFilter{
// 							UseOperatorGroup: true,
// 						},
// 					},
// 				},
// 				Meters: []marketplacev1beta1.MeterWorkload{
// 					{
// 						Aggregation: "sum",
// 						Period: &metav1.Duration{
// 							Duration: time.Duration(time.Hour * 1),
// 						},
// 						Query:        "kube_service_labels{namespace='openshift-redhat-marketplace',label_couchbase_cluster=~'.+',service=~'.+-ui'}",
// 						Metric:       "couchbase_cluster_count",
// 						WorkloadType: marketplacev1beta1.WorkloadTypeService,
// 						Name:         "couchbase_cluster_count",
// 						Without: []string{"label_couchbase_cluster","label_app","label_operator_couchbase_com_version"},
// 					},
// 				},
// 			},
// 		},
// 		{
// 			ObjectMeta: metav1.ObjectMeta{
// 				Name:      "robin-meterdef",
// 				Namespace: "openshift-redhat-marketplace",
// 				Annotations: map[string]string{
// 					"versionRange": "0.0.1 - 12",
// 					"packageName" : "joget-dx-operator-rhmp",
// 				},
// 			},
// 			Spec: marketplacev1beta1.MeterDefinitionSpec{
// 				Group: "robinclusters.robin.io",
// 				Kind:  "RobinCluster",

// 				ResourceFilters: []marketplacev1beta1.ResourceFilter{
// 					{
// 						WorkloadType: marketplacev1beta1.WorkloadTypePod,
// 						OwnerCRD:  &marketplacev1beta1.OwnerCRDFilter{
// 							common.GroupVersionKind{
// 								APIVersion: "manage.robin.io/v1",
// 								Kind: "RobinCluster",
// 							},
// 						},
// 						Namespace: &marketplacev1beta1.NamespaceFilter{
// 							UseOperatorGroup: true,
// 						},
// 					},
// 				},
// 				Meters: []marketplacev1beta1.MeterWorkload{
// 					{
// 						Aggregation: "avg",
// 						Period: &metav1.Duration{
// 							Duration: time.Duration(time.Hour * 1),
// 						},
// 						Query:        "min_over_time((kube_pod_info{created_by_kind='DaemonSet',created_by_name='robin',node=~'.*'}or on() vector(0))[60m:60m])",
// 						Metric:       "node_hour2",
// 						WorkloadType: marketplacev1beta1.WorkloadTypePod,
// 						Name:         "robin storage deamon set usage",
// 					},
// 				},
// 			},
// 		},
// 	}

// 	mdefStoreString, err := json.Marshal(mdefList)

// 	if err != nil {
// 		return &ExecResult{
// 			ReconcileResult: reconcile.Result{},
// 			Err: err,
// 		}
// 	}

// 	mdefStoreCM := &corev1.ConfigMap{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      utils.METERDEF_STORE_NAME,
// 			Namespace: "openshift-redhat-marketplace",

// 		},
// 		Data: map[string]string{
// 			"meterdefinitions" : string(mdefStoreString),
// 		},
// 	}

// 	err = client.Create(context.TODO(),mdefStoreCM)
// 	if err != nil {
// 		return &ExecResult{
// 			ReconcileResult: reconcile.Result{},
// 			Err: err,
// 		}
// 	}

// 	return &ExecResult{
// 		Status: ActionResultStatus(Continue),
// 	}
// }
