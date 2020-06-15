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
	"time"

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
				return okOperator && operatorTagLabel == "true"
			},
			CreateFunc: func(evt event.CreateEvent) bool {
				operatorTagLabel, ok := evt.Meta.GetLabels()[operatorTag]
				return ok && operatorTagLabel == "true"
			},
			DeleteFunc: func(evt event.DeleteEvent) bool {
				operatorTagLabel, ok := evt.Meta.GetLabels()[operatorTag]
				return ok && operatorTagLabel == "true"
			},
			GenericFunc: func(evt event.GenericEvent) bool {
				operatorTagLabel, ok := evt.Meta.GetLabels()[operatorTag]
				return ok && operatorTagLabel == "true"
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

	// check for uninstall label and delete the components
	if instance.ObjectMeta.Labels[uninstallTag] == "true" {
		if instance.Status.State == olmv1alpha1.SubscriptionStateUpgradePending {
			if instance.Spec.InstallPlanApproval == olmv1alpha1.ApprovalManual {
				installPlan, err := r.getInstallPlanForSubscription(instance, request)
				if err != nil {
					return reconcile.Result{}, err
				}

				if installPlan.Status.Phase != olmv1alpha1.InstallPlanPhaseRequiresApproval {
					return reconcile.Result{Requeue: true, RequeueAfter: time.Duration(5 * time.Second)}, nil
				}
			} else {
				return reconcile.Result{Requeue: true, RequeueAfter: time.Duration(5 * time.Second)}, nil
			}
		}

		if err := r.deleteSubscription(instance); err != nil {
			reqLogger.Info("error deleting subscription", err)
			if errors.IsNotFound(err) {
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, err
		}

		if err := r.deleteCSVInstance(instance, request); err != nil {
			reqLogger.Info("error deleting clusterserviceversion", err)
			if !errors.IsNotFound(err) {
				// Request object not found, could have been deleted after reconcile request.
				// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
				// Return and don't requeue
				return reconcile.Result{}, nil
			}
			// Error reading the object - requeue the request.
			return reconcile.Result{}, err
		}

		reqLogger.Info("subscription deletion successful")
		return reconcile.Result{}, nil
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

func (r *ReconcileSubscription) deleteCSVInstance(instance *olmv1alpha1.Subscription, request reconcile.Request) error {
	csvInstance := &olmv1alpha1.ClusterServiceVersion{}
	t := &reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      instance.Status.CurrentCSV,
			Namespace: request.NamespacedName.Namespace,
		},
	}
	if err := r.client.Get(context.TODO(), t.NamespacedName, csvInstance); err != nil {
		return err
	}

	return r.client.Delete(context.TODO(), csvInstance)
}

func (r *ReconcileSubscription) deleteSubscription(instance *olmv1alpha1.Subscription) error {
	return r.client.Delete(context.TODO(), instance)
}

func (r *ReconcileSubscription) getInstallPlanForSubscription(instance *olmv1alpha1.Subscription, request reconcile.Request) (*olmv1alpha1.InstallPlan, error) {
	installPlanInstance := &olmv1alpha1.InstallPlan{}
	t2 := &reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      instance.Status.InstallPlanRef.Name,
			Namespace: request.NamespacedName.Namespace,
		},
	}
	if err := r.client.Get(context.TODO(), t2.NamespacedName, installPlanInstance); err != nil {
		return nil, err
	}

	return installPlanInstance, nil
	//return r.client.Delete(context.TODO(), installPlanInstance)
}
