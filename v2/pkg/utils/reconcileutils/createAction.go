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

package reconcileutils

import (
	"context"
	"fmt"

	emperrors "emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type createAction struct {
	*BaseAction
	newObject client.Object
	createActionOptions
}

//go:generate go-options -option CreateActionOption -imports=sigs.k8s.io/controller-runtime/pkg/client,github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch -prefix Create createActionOptions
type createActionOptions struct {
	WithPatch         patch.PatchAnnotator
	WithAddOwner      client.Object `options:",nil"`
	WithAddController client.Object `options:",nil"`
}

func CreateAction(
	newObj client.Object,
	opts ...CreateActionOption,
) *createAction {
	createOpts, _ := newCreateActionOptions(opts...)
	return &createAction{
		newObject:           newObj,
		createActionOptions: createOpts,
		BaseAction:          NewBaseAction("Create"),
	}
}

func (a *createAction) Bind(result *ExecResult) {
	a.LastResult = result
}

func (a *createAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	reqLogger := a.GetReqLogger(c)

	if isNil(a.newObject) {
		err := emperrors.WithStack(ErrNilObject)
		reqLogger.Error(err, "newObject is nil")
		return NewExecResult(Error, reconcile.Result{}, a.BaseAction, err), err
	}

	key := client.ObjectKeyFromObject(a.newObject)
	reqLogger = reqLogger.WithValues("requestType", fmt.Sprintf("%T", a.newObject), "key", key)

	if a.WithAddController != nil {
		if meta, ok := a.newObject.(metav1.Object); ok {
			if err := controllerutil.SetControllerReference(
				a.WithAddController.(metav1.Object),
				meta,
				c.scheme); err != nil {
				c.log.Error(err, "Failed to create.", "obj", a.newObject)
				return NewExecResult(Error, reconcile.Result{}, a.BaseAction, err), emperrors.Wrap(err, "error adding owner")
			}
		}
	} else {
		if a.WithAddOwner != nil {
			if meta, ok := a.newObject.(metav1.Object); ok {
				if err := controllerutil.SetOwnerReference(
					a.WithAddOwner.(metav1.Object),
					meta,
					c.scheme); err != nil {
					c.log.Error(err, "Failed to create.", "obj", a.newObject)
					return NewExecResult(Error, reconcile.Result{}, a.BaseAction, err), emperrors.Wrap(err, "error adding owner")
				}
			}
		}
	}

	if a.WithPatch != nil {
		if err := a.WithPatch.SetLastAppliedAnnotation(a.newObject); err != nil {
			reqLogger.Error(err, "failure creating patch")
			return NewExecResult(Error, reconcile.Result{}, a.BaseAction, err), emperrors.Wrap(err, "error with patch")
		}
	}

	reqLogger.Info("Creating object", "object", a.newObject)
	err := c.client.Create(ctx, a.newObject)
	if err != nil {
		c.log.Error(err, "Failed to create.", "obj", a.newObject)
		return NewExecResult(Error, reconcile.Result{Requeue: true}, a.BaseAction, err), emperrors.Wrap(err, "error with create")
	}

	return NewExecResult(Requeue, reconcile.Result{Requeue: true}, a.BaseAction, nil), nil
}
