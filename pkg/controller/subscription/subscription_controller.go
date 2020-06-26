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

package subscription

import (
	"context"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_olm_subscription_watcher")

const operatorTag = "marketplace.redhat.com/operator"
const uninstallTag = "marketplace.redhat.com/uninstall"

// Add creates a new Subscription Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSubscription{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("subscription-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	labelPreds := []predicate.Predicate{
		predicate.Funcs{
			UpdateFunc: func(evt event.UpdateEvent) bool {
				operatorTagLabel, okOperator := evt.MetaNew.GetLabels()[operatorTag]
				uninstallTagLabel, okUninstall := evt.MetaNew.GetLabels()[uninstallTag]
				return (okOperator && operatorTagLabel == "true") || (okUninstall && uninstallTagLabel == "true")
			},
			CreateFunc: func(evt event.CreateEvent) bool {
				operatorTagLabel, okOperator := evt.Meta.GetLabels()[operatorTag]
				uninstallTagLabel, okUninstall := evt.Meta.GetLabels()[uninstallTag]
				return (okOperator && operatorTagLabel == "true") || (okUninstall && uninstallTagLabel == "true")
			},
			DeleteFunc: func(evt event.DeleteEvent) bool {
				operatorTagLabel, okOperator := evt.Meta.GetLabels()[operatorTag]
				uninstallTagLabel, okUninstall := evt.Meta.GetLabels()[uninstallTag]
				return (okOperator && operatorTagLabel == "true") || (okUninstall && uninstallTagLabel == "true")
			},
			GenericFunc: func(evt event.GenericEvent) bool {
				operatorTagLabel, okOperator := evt.Meta.GetLabels()[operatorTag]
				uninstallTagLabel, okUninstall := evt.Meta.GetLabels()[uninstallTag]
				return (okOperator && operatorTagLabel == "true") || (okUninstall && uninstallTagLabel == "true")
			},
		},
	}

	// Watch for changes to primary resource Subscription
	err = c.Watch(&source.Kind{Type: &olmv1alpha1.Subscription{}}, &handler.EnqueueRequestForObject{}, labelPreds...)
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileSubscription implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileSubscription{}

// ReconcileSubscription reconciles a Subscription object
type ReconcileSubscription struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Subscription object and makes changes based on the state read
// and what is in the Subscription.Spec
func (r *ReconcileSubscription) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Subscription")

	// Fetch the Subscription instance
	instance := &olmv1alpha1.Subscription{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// check for uninstall label and delete the resources
	if instance.ObjectMeta.Labels[uninstallTag] == "true" {
		return r.uninstall(instance)
	}

	groups := &olmv1.OperatorGroupList{}

	// find operator groups
	err = r.client.List(context.TODO(),
		groups,
		client.InNamespace(instance.GetNamespace()))

	if err != nil {
		return reconcile.Result{}, err
	}

	createList := []*olmv1.OperatorGroup{}
	deleteList := []*olmv1.OperatorGroup{}

	// if none exist, we'll create one
	if len(groups.Items) == 0 {
		reqLogger.Info("need to create an operator group")
		createList = append(createList, r.createOperatorGroup(instance))
	}

	if len(groups.Items) > 1 {
		reqLogger.Info("need to create an operator group")
		for _, og := range groups.Items {
			if val, ok := og.Labels[operatorTag]; ok && val == "true" {
				deleteList = append(deleteList, &og)
			}
		}
	}

	for _, og := range deleteList {
		reqLogger.Info("deleting operator group",
			"name", og.GetName(),
			"namespace", og.GetNamespace())

		err = r.client.Delete(context.TODO(), og)

		// nothing to create
		if err != nil {
			reqLogger.Error(err,
				"failed to delete",
				"generate-name", og.GetGenerateName(),
				"namespace", og.GetNamespace())
			return reconcile.Result{}, err
		}

		return reconcile.Result{Requeue: true}, nil
	}

	for _, og := range createList {
		reqLogger.Info("creating an operator group",
			"generate-name", og.GetGenerateName(),
			"namespace", og.GetNamespace())
		err = r.client.Create(context.TODO(), og)

		// nothing to create
		if err != nil {
			reqLogger.Error(err,
				"failed to create",
				"generate-name", og.GetGenerateName(),
				"namespace", og.GetNamespace())
			return reconcile.Result{}, err
		}

		reqLogger.Info("succesfully created",
			"generate-name", og.GetGenerateName(),
			"namespace", og.GetNamespace())
		return reconcile.Result{Requeue: true}, nil
	}

	reqLogger.Info("reconcilation complete")
	return reconcile.Result{}, nil
}

func (r *ReconcileSubscription) createOperatorGroup(instance *olmv1alpha1.Subscription) *olmv1.OperatorGroup {
	return &olmv1.OperatorGroup{
		ObjectMeta: v1.ObjectMeta{
			Namespace:    instance.Namespace,
			GenerateName: "redhat-marketplace-og-",
			Labels: map[string]string{
				operatorTag: "true",
			},
		},
		Spec: olmv1.OperatorGroupSpec{
			TargetNamespaces: []string{
				instance.Namespace,
			},
		},
	}
}

func (r *ReconcileSubscription) uninstall(sub *olmv1alpha1.Subscription) (reconcile.Result, error) {
	reqLogger := log.WithValues("Subscription.Namespace", sub.Namespace, "Subscription.Name", sub.Name, "Subscription.Spec.Package", sub.Spec.Package)
	reqLogger.Info("started to uninstall operator")

	// collect CSVs from installPlans
	csvList := make(map[string]struct{})
	installPlanList := &olmv1alpha1.InstallPlanList{}
	err := r.client.List(context.TODO(), installPlanList, client.InNamespace(sub.Namespace))
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "could not list installPlan")
	}
	if err == nil {
		for _, ip := range installPlanList.Items {
			if len(ip.OwnerReferences) > 0 && ip.OwnerReferences[0].Kind == sub.Kind && ip.OwnerReferences[0].Name == sub.Name {
				for _, csv := range ip.Spec.ClusterServiceVersionNames {
					csvList[csv] = struct{}{}
				}
			}
		}
	}

	// delete sub
	err = r.client.Delete(context.TODO(), sub)
	if err != nil && !errors.IsNotFound((err)) {
		reqLogger.Error(err, "could not delete sub")
	}

	// delete CSVs
	for csvName := range csvList {
		csvObj := &olmv1alpha1.ClusterServiceVersion{}
		csvNamespacedName := types.NamespacedName{
			Name:      csvName,
			Namespace: sub.Namespace,
		}
		err = r.client.Get(context.TODO(), csvNamespacedName, csvObj)
		if err != nil && !errors.IsNotFound((err)) {
			reqLogger.Error(err, "could not delete csv", "csv name", csvName)
		}
		if err == nil {
			err = r.client.Delete(context.TODO(), csvObj)
			if err != nil && !errors.IsNotFound((err)) {
				reqLogger.Error(err, "could not delete csv", "csv name", csvName)
			}
		}
	}

	reqLogger.Info("uninstalling operator complete")
	return reconcile.Result{}, nil
}
