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
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"k8s.io/apimachinery/pkg/types"
)

type PodMonitor struct {
	Logger logr.Logger
	Client client.Client
	cfg    *config.OperatorConfig
}

func (r *PodMonitor) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (r *PodMonitor) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	r.cfg = cfg
	return nil
}

func (a *PodMonitor) SetupWithManager(mgr ctrl.Manager) error {
	namespacePredicate := predicates.NamespacePredicate(a.cfg.DeployedNamespace)

	return ctrl.NewControllerManagedBy(mgr).
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

func (a *PodMonitor) Reconcile(req reconcile.Request) (reconcile.Result, error) {
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
