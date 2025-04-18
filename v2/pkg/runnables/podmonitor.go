// Copyright 2021 IBM Corp.
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

package runnables

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// PodMonitor is responsible for deleting failing auth checker pods.
// Why authchecker? If you have an partial uninstall, for example,
// just delete and recreate a SA. The tokens originally associated to
// that SA are invalidated but the pods don't realize that and start throwing
// errors. Restarting the pod doesn't work because it doesn't get a new
// token on just a restart. The pod has to be deleted. Authchecker
// detects this scenario and deletes the pod for you.
type PodMonitor struct {
	Logger logr.Logger
	Client client.Client
	Cfg    *config.OperatorConfig
}

func (a *PodMonitor) SetupWithManager(mgr ctrl.Manager) error {
	namespacePredicate := predicates.NamespacePredicate(a.Cfg.DeployedNamespace)

	return ctrl.NewControllerManagedBy(mgr).
		Named("pod-monitor").
		For(&corev1.Pod{}, builder.WithPredicates(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					pod := e.ObjectNew.(*corev1.Pod)

					for _, status := range pod.Status.ContainerStatuses {
						if status.Name == "authcheck" && !status.Ready {
							if status.RestartCount > 2 {
								return true
							}
						}
					}

					return false
				},
				DeleteFunc: func(event.DeleteEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			})).
		WithEventFilter(namespacePredicate).
		Complete(a)
}

func (a *PodMonitor) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	pod := &corev1.Pod{}
	err := a.Client.Get(context.TODO(), types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, pod)

	if err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	err = a.Client.Delete(context.TODO(), pod)

	if err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
