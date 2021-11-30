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

package authchecker

import (
	"context"
	"net/http"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type AuthChecker struct {
	Logger    logr.Logger
	RetryTime time.Duration
	Namespace string
	Podname   string
	Client    client.Client
	Checker   *AuthCheckChecker
}

type AuthCheckChecker struct {
	Err   error
	mutex sync.Mutex
}

var a *AuthCheckChecker = &AuthCheckChecker{}
var _ healthz.Checker = a.Check

func (c *AuthCheckChecker) SetErr(err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Err = err
}

func (c *AuthCheckChecker) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Err = nil
}

func (c *AuthCheckChecker) Check(_ *http.Request) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.Err != nil {
		return c.Err
	}

	return nil
}

func (a *AuthChecker) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Pod{}, builder.WithPredicates(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return e.Object.GetName() == a.Podname
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					return e.ObjectNew.GetName() == a.Podname
				},
				DeleteFunc: func(event.DeleteEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			})).
		Complete(a)
}

func (a *AuthChecker) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	pod := &v1.Pod{}
	secrets := &v1.SecretList{}
	err := a.Client.Get(context.TODO(), types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, pod)

	if err != nil {
		a.Logger.Error(err, "failed to get pod")
		a.Checker.SetErr(err)
		return reconcile.Result{}, err
	}

	err = a.Client.List(context.TODO(), secrets)

	if err != nil {
		a.Logger.Error(err, "failed to list secrets")
		a.Checker.SetErr(err)
		return reconcile.Result{}, err
	}

volume:
	for _, vol := range pod.Spec.Volumes {
		if vol.Secret == nil {
			continue
		}

		if vol.Secret.Optional != nil && *vol.Secret.Optional {
			continue
		}

		for _, secret := range secrets.Items {
			if vol.Secret.SecretName == secret.Name {
				a.Logger.V(4).Info("found secret", "secret", secret.Name)
				continue volume
			}
		}

		err := errors.NewWithDetails("secret missing", "pod", pod.Name, "secret", vol.Secret.SecretName)
		a.Logger.Error(err, "failed to get secret")
		a.Checker.SetErr(err)
		return reconcile.Result{RequeueAfter: a.RetryTime}, nil
	}

	// ensure the checker is clear
	a.Checker.Clear()
	return reconcile.Result{RequeueAfter: a.RetryTime}, nil
}
