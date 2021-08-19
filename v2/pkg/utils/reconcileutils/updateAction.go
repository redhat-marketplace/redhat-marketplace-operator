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
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type updateAction struct {
	*BaseAction
	updateObject runtime.Object
	updateActionOptions
}

//go:generate go-options -imports=github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch  -option UpdateActionOption -prefix Update updateActionOptions
type updateActionOptions struct {
	StatusOnly bool
	WithPatch  patch.PatchAnnotator
}

func UpdateAction(
	updateObject runtime.Object,
	updateOptions ...UpdateActionOption,
) *updateAction {
	opts, _ := newUpdateActionOptions(updateOptions...)
	return &updateAction{
		updateObject:        updateObject,
		updateActionOptions: opts,
		BaseAction:          NewBaseAction("UpdateAction"),
	}
}

func (a *updateAction) Bind(result *ExecResult) {
	a.LastResult = result
}

func (a *updateAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	updatedObject := a.updateObject
	reqLogger := a.GetReqLogger(c)

	if isNil(updatedObject) {
		err := emperrors.WithStack(ErrNilObject)
		reqLogger.Error(err, "updatedObject is nil")
		return NewExecResult(Error, reconcile.Result{}, a.BaseAction, err), err
	}

	key, _ := client.ObjectKeyFromObject(updatedObject)
	reqLogger = reqLogger.WithValues("requestType", fmt.Sprintf("%T", updatedObject), "key", key)
	var err error

	if a.StatusOnly {
		err = c.client.Status().Update(ctx, updatedObject)
	} else {
		if a.WithPatch != nil {
			if err := a.WithPatch.SetLastAppliedAnnotation(updatedObject); err != nil {
				reqLogger.Error(err, "failure creating patch")
				return NewExecResult(Error, reconcile.Result{}, a.BaseAction, err), emperrors.Wrap(err, "error with patch")
			}
		}

		err = c.client.Update(ctx, updatedObject)
	}

	if err != nil {
		reqLogger.Error(err, "error updating object")
		return NewExecResult(Error, reconcile.Result{}, a.BaseAction, err), emperrors.Wrap(err, "error applying update")
	}

	reqLogger.Info("updated object")
	return NewExecResult(Requeue, reconcile.Result{Requeue: true}, a.BaseAction, nil), nil
}

type updateStatusConditionAction struct {
	instance   runtime.Object
	conditions *status.Conditions
	condition  status.Condition
	*BaseAction
}

func UpdateStatusCondition(
	instance runtime.Object,
	conditions *status.Conditions,
	condition status.Condition,
) *updateStatusConditionAction {
	return &updateStatusConditionAction{
		instance:   instance,
		conditions: conditions,
		condition:  condition,

		BaseAction: NewBaseAction("UpdateStatusCondition"),
	}
}

func (u *updateStatusConditionAction) Bind(result *ExecResult) {
	u.LastResult = result
}

func (u *updateStatusConditionAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	reqLogger := u.GetReqLogger(c)

	if isNil(u.instance) {
		err := emperrors.WithStack(ErrNilObject)
		reqLogger.Error(err, "instance is nil")
		return NewExecResult(Error, reconcile.Result{}, u.BaseAction, err), err
	}

	if isNil(u.conditions) {
		err := emperrors.WithStack(ErrNilObject)
		reqLogger.Info("conditions cannot be nil")
		return NewExecResult(Error, reconcile.Result{}, u.BaseAction, err), emperrors.New("conditions cannot be nil")
	}

	key, _ := client.ObjectKeyFromObject(u.instance)
	reqLogger = reqLogger.WithValues("requestType", fmt.Sprintf("%T", u.instance), "key", key)

	if u.conditions.SetCondition(u.condition) {
		reqLogger.Info("updating condition")
		err := c.client.Status().Update(context.TODO(), u.instance)

		if err != nil {
			reqLogger.Error(err, "updating condition")
			return NewExecResult(Error, reconcile.Result{}, u.BaseAction, err), emperrors.Wrap(err, "error while updating status condition")
		}

		return NewExecResult(Requeue, reconcile.Result{Requeue: true}, u.BaseAction, nil), nil
	}

	reqLogger.Info("no update required")
	return NewExecResult(Continue, reconcile.Result{}, u.BaseAction, nil), nil
}

type updateWithPatchAction struct {
	*BaseAction
	object    runtime.Object
	patchData []byte
	patchType types.PatchType
	patcher   patch.Patcher
}

func UpdateWithPatchAction(
	object runtime.Object,
	patchType types.PatchType,
	patchData []byte,
) *updateWithPatchAction {
	return &updateWithPatchAction{
		BaseAction: NewBaseAction("updateWithPatchAction"),
		object:     object,
		patchData:  patchData,
		patchType:  patchType,
	}
}

func (a *updateWithPatchAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	reqLogger := a.GetReqLogger(c)

	reqLogger.Info("updating with patch", "patch", string(a.patchData))
	err := c.client.Patch(ctx, a.object, client.RawPatch(a.patchType, a.patchData))

	if err != nil {
		reqLogger.Error(err, "error updating with patch")
		return NewExecResult(Error, reconcile.Result{}, a.BaseAction, err), emperrors.Wrap(err, "error while updating with patch")
	}

	return NewExecResult(Requeue, reconcile.Result{Requeue: true}, a.BaseAction, nil), nil
}
