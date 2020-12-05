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

package controllers

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// blank assignment to verify that ReconcileNode implements reconcile.Reconciler
var _ reconcile.Reconciler = &RemoteResourceS3Reconciler{}

// ReconcileNode reconciles a Node object
type RemoteResourceS3Reconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

func (r *RemoteResourceS3Reconciler) SetupWithManager(mgr manager.Manager) error {
	labelPreds := []predicate.Predicate{
		predicate.Funcs{
			UpdateFunc: func(evt event.UpdateEvent) bool {
				return true
			},
			CreateFunc: func(evt event.CreateEvent) bool {
				return true
			},
			GenericFunc: func(evt event.GenericEvent) bool {
				return false
			},
			DeleteFunc: func(evt event.DeleteEvent) bool {
				return false
			},
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		Watches(&source.Kind{Type: &marketplacev1alpha1.RemoteResourceS3{}}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(labelPreds...)).
		Complete(r)
}

// Reconcile reads that state of the cluster for a Node object and makes changes based on the state read
// and what is in the Node.Spec
func (r *RemoteResourceS3Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Name", request.Name, "Request.Namespace", request.Namespace)
	reqLogger.Info("Reconciling RemoteResourceS3")

	// Fetch the Node instance
	instance := &marketplacev1alpha1.RemoteResourceS3{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Error(err, "remoteresources3 does not exist")
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get node")
		return reconcile.Result{}, err
	}

	if instance.Status.Touched == nil {
		instance.Status = marketplacev1alpha1.RemoteResourceS3Status{
			Touched: ptr.Bool(true),
		}
		err := r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		reqLogger.Info("updated remoteresources3")
	}

	reqLogger.Info("finished reconcile")
	return reconcile.Result{}, nil
}
