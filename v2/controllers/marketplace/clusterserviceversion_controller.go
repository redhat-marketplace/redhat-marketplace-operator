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
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	emperrors "emperror.dev/errors"
	"github.com/go-logr/logr"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/scheme"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	utils "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

//var log = logf.Log.WithName("controller_olm_clusterserviceversion_watcher")

const (
	olmCopiedFromTag string = "olm.copiedFrom"
	olmNamespace     string = "olm.operatorNamespace"
	ignoreTag        string = "marketplace.redhat.com/ignore"
	ignoreTagValue   string = "2"
	meterDefStatus   string = "marketplace.redhat.com/meterDefinitionStatus"
	meterDefError    string = "marketplace.redhat.com/meterDefinitionError"
)

// blank assignment to verify that ReconcileClusterServiceVersion implements reconcile.Reconciler
var _ reconcile.Reconciler = &ClusterServiceVersionReconciler{}

// ClusterServiceVersionReconciler reconciles a ClusterServiceVersion object
type ClusterServiceVersionReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions;meterdefinitions/status,verbs=get;list;watch;create;update;patch;delete

// Reconcile reads that state of the cluster for a ClusterServiceVersion object and makes changes based on the state read
// and what is in the ClusterServiceVersion.Spec
func (r *ClusterServiceVersionReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
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

	annotations := CSV.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	//
	// csvFilter checks the annotations, validate again
	//

	// checks if it is possible to build MeterDefinition from annotations of CSV
	reqLogger.Info("retrieving MeterDefinition string from csv")
	meterDefinitionString, ok := annotations[utils.CSV_METERDEFINITION_ANNOTATION]
	if !ok || len(meterDefinitionString) == 0 {
		reqLogger.Info("No value for ", "key: ", utils.CSV_METERDEFINITION_ANNOTATION)
		return reconcile.Result{}, nil
	}

	//
	ns, ok := CSV.GetAnnotations()[olmNamespace]
	if ok && ns != CSV.GetNamespace() {
		reqLogger.Info("MeterDefinition is global and this CSV is not the head")
		return reconcile.Result{}, nil
	}

	if !ok {
		reqLogger.Info("olm.operatorNamespace annotation is not set yet, requeuing")
		return reconcile.Result{RequeueAfter: time.Second * 5}, nil
	}

	// builds a meterdefinition from our string (from the annotation)
	reqLogger.Info("retrieval successful", "str", meterDefinitionString)

	var errAlpha, errBeta error
	meterDefinitionBeta := &marketplacev1beta1.MeterDefinition{}
	meterDefinitionAlpha := &marketplacev1alpha1.MeterDefinition{}

	unstructured := &unstructured.Unstructured{}
	meterDefinition := &marketplacev1beta1.MeterDefinition{}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	_, objectKind, _ := decode([]byte(meterDefinitionString), nil, nil)

	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(meterDefinitionString)), 100).Decode(unstructured)
	if err == nil {
		switch {
		case objectKind.Version == "v1beta1":
			reqLogger.Info("mdef is a v1beta1", "value", meterDefinitionBeta)
			_ = meterDefinitionBeta.BuildMeterDefinitionFromString(
				meterDefinitionString,
				CSV.GetName(), CSV.GetNamespace(),
				utils.CSV_ANNOTATION_NAME, utils.CSV_ANNOTATION_NAMESPACE)

			meterDefinition = meterDefinitionBeta
		case objectKind.Version == "v1alpha1":
			reqLogger.Info("mdef is an v1alpha1")
			errAlpha = meterDefinitionAlpha.BuildMeterDefinitionFromString(
				meterDefinitionString,
				CSV.GetName(), CSV.GetNamespace(),
				utils.CSV_ANNOTATION_NAME, utils.CSV_ANNOTATION_NAMESPACE)

			if errAlpha == nil {
				err = meterDefinitionAlpha.ConvertTo(meterDefinition)

				if err != nil {
					reqLogger.Error(err, "Failed to convert to v1beta1")
				}
			}
		default:
			reqLogger.Info("mdef is neither")
			err = emperrors.Combine(err, errBeta, errAlpha)
			reqLogger.Error(err, "Failed to read the json annotation as a meterdefinition")
		}
	}

	if err != nil {
		reqLogger.Error(err, "Could not build a local copy of the MeterDefinition")
		// Consider setting err Status on MarketplaceConfig
	}
	reqLogger.Info("marketplacev1beta1.MeterDefinitionList >>>> ")

	// Case 1: The CSV is old: compare vs. expected MeterDefinition
	list := &marketplacev1beta1.MeterDefinitionList{}
	err = r.Client.List(context.TODO(), list, client.InNamespace(meterDefinition.GetNamespace()))

	if err != nil {
		reqLogger.Error(err, "Could not retrieve the existing MeterDefinition")
		return reconcile.Result{}, err
	}
	reqLogger.Info("marketplacev1beta1.MeterDefinitionList End --- ")
	var actualMeterDefinition *marketplacev1beta1.MeterDefinition

	// Find the meterdef, we're use the InstalledBy field
	for _, meterDef := range list.Items {
		if meterDef.Spec.InstalledBy != nil &&
			meterDef.Spec.InstalledBy.Namespace == CSV.Namespace &&
			meterDef.Spec.InstalledBy.Name == CSV.Name {
			actualMeterDefinition = &meterDef
			reqLogger.Info("Found meterDef", "meterDef", meterDef)
			break
		}
	}

	// Check if the name has changed
	if actualMeterDefinition != nil && actualMeterDefinition.Name != meterDefinition.Name {
		reqLogger.Info("Discovered name change", "name", actualMeterDefinition.Name, "newName", meterDefinition.Name)
		err := r.Client.Delete(context.TODO(), actualMeterDefinition)

		if err != nil {
			return reconcile.Result{}, err
		}

		actualMeterDefinition = nil
	}

	// If not nil, we update
	if actualMeterDefinition != nil {
		if !reflect.DeepEqual(meterDefinition.Spec, actualMeterDefinition.Spec) &&
			!reflect.DeepEqual(meterDefinition.ObjectMeta, actualMeterDefinition.ObjectMeta) {
			reqLogger.Info("The actual meterdefinition is different from the expected meterdefinition")

			patch, err := json.Marshal(meterDefinition)
			if err != nil {
				return reconcile.Result{}, err
			}
			err = r.Client.Patch(context.TODO(), meterDefinition, client.RawPatch(types.MergePatchType, patch))
			if err != nil {
				return reconcile.Result{Requeue: true}, err
			}
			reqLogger.Info("Patch to update MeterDefinition successful. Requeuing")
			return reconcile.Result{Requeue: true}, nil
		}
		reqLogger.Info("meter definition matches")
		return reconcile.Result{}, nil
	}

	// Case 2: The CSV is new: we must track it & we must create the Meter Definition
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

	meterDefinition.ObjectMeta.OwnerReferences = append(meterDefinition.ObjectMeta.OwnerReferences, ref)

	if err != nil {
		reqLogger.Error(err, "Failed to create.", "obj", meterDefinition)
		return reconcile.Result{}, err
	}

	meterDefinition.ObjectMeta.Namespace = CSV.Namespace

	err = r.Client.Create(context.TODO(), meterDefinition)
	if err != nil {
		reqLogger.Error(err, "Could not create MeterDefinition", "mdef", meterDefinition)
		// Consider setting err Status on MarketplaceConfig
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// Has an annotation with a MeterDefinition and is is the head CSV (not a copy)
func csvFilter(metaNew metav1.Object) bool {
	ann := metaNew.GetAnnotations()
	labels := metaNew.GetLabels()

	_, hasCopiedFrom := labels[olmCopiedFromTag]
	_, hasMeterDefinition := ann[utils.CSV_METERDEFINITION_ANNOTATION]
	_, hasOlmNamespace := ann[olmNamespace]

	return (hasOlmNamespace && hasMeterDefinition && !hasCopiedFrom)
}

// Update to MeterDefinition annotation which may not be a generation update
func checkForUpdateToMdef(evt event.UpdateEvent) bool {

	oldMeterDef, _ := evt.ObjectOld.GetAnnotations()[utils.CSV_METERDEFINITION_ANNOTATION]
	newMeterDef, newMeterDefOk := evt.ObjectNew.GetAnnotations()[utils.CSV_METERDEFINITION_ANNOTATION]

	oldOlmCopied, _ := evt.ObjectOld.GetLabels()[olmCopiedFromTag]
	newOlmCopied, newOlmCopiedOk := evt.ObjectNew.GetLabels()[olmCopiedFromTag]

	oldOlmNamespace, _ := evt.ObjectOld.GetAnnotations()[olmNamespace]
	newOlmNamespace, newOlmNamespaceOk := evt.ObjectNew.GetAnnotations()[olmNamespace]

	// If all expected annotations are set, and one of them changed, then reconcile
	return (newMeterDefOk && !newOlmCopiedOk && newOlmNamespaceOk) && ((oldMeterDef != newMeterDef) ||
		(oldOlmCopied != newOlmCopied) ||
		(oldOlmNamespace != newOlmNamespace))
}

var clusterServiceVersionPredictates predicate.Funcs = predicate.Funcs{
	UpdateFunc: func(evt event.UpdateEvent) bool {
		return checkForUpdateToMdef(evt)
	},
	DeleteFunc: func(evt event.DeleteEvent) bool {
		return false
	},
	CreateFunc: func(evt event.CreateEvent) bool {
		return csvFilter(evt.Object)
	},
	GenericFunc: func(evt event.GenericEvent) bool {
		return false
	},
}

func (r *ClusterServiceVersionReconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&olmv1alpha1.ClusterServiceVersion{}, builder.WithPredicates(clusterServiceVersionPredictates)).
		Watches(
			&marketplacev1beta1.MeterDefinition{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &olmv1alpha1.ClusterServiceVersion{}),
		).
		Complete(r)
}
