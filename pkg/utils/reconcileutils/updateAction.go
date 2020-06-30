package reconcileutils

import (
	"context"
	"fmt"

	emperrors "emperror.dev/errors"
	"github.com/operator-framework/operator-sdk/pkg/status"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/codelocation"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/patch"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type updateAction struct {
	baseAction
	updateObject runtime.Object
	updateActionOptions
}

//go:generate go-options -option UpdateActionOption -prefix Update updateActionOptions
type updateActionOptions struct {
}

type PatchChecker struct {
	patchMaker patch.PatchMaker
}

func NewPatchChecker(p patch.PatchMaker) *PatchChecker {
	return &PatchChecker{
		patchMaker: p,
	}
}

func (p *PatchChecker) CheckPatch(
	originalObject runtime.Object,
	updatedObject runtime.Object,
) (update bool, err error) {
	update = false

	patchResult, err := p.patchMaker.Calculate(originalObject, updatedObject)

	if err != nil {
		return
	}

	if patchResult.IsEmpty() {
		return
	}

	update = true
	return
}

func UpdateAction(
	updateObject runtime.Object,
	updateOptions ...UpdateActionOption,
) *updateAction {
	opts, _ := newUpdateActionOptions(updateOptions...)
	return &updateAction{
		updateObject:        updateObject,
		updateActionOptions: opts,
		baseAction: baseAction{
			codelocation: codelocation.New(1),
		},
	}
}

func (a *updateAction) Bind(result *ExecResult) {
	a.lastResult = result
}

func (a *updateAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	updatedObject := a.updateObject
	reqLogger := c.log.WithValues("file", a.codelocation, "action", "UpdateAction")

	if isNil(updatedObject) {
		err := emperrors.WithStack(ErrNilObject)
		reqLogger.Error(err, "updatedObject is nil")
		return NewExecResult(Error, reconcile.Result{}, err), err
	}

	key, _ := client.ObjectKeyFromObject(updatedObject)
	reqLogger = reqLogger.WithValues("requestType", fmt.Sprintf("%T", updatedObject), "key", key)
	err := c.client.Update(ctx, updatedObject)

	if err != nil {
		reqLogger.Error(err, "error updating object")
		return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error applying update")
	}

	reqLogger.Info("updated object")
	return NewExecResult(Requeue, reconcile.Result{Requeue: true}, nil), nil
}

type updateStatusConditionAction struct {
	instance   runtime.Object
	conditions *status.Conditions
	condition  status.Condition
	baseAction
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

		baseAction: baseAction{
			codelocation: codelocation.New(1),
		},
	}
}

func (u *updateStatusConditionAction) Bind(result *ExecResult) {
	u.lastResult = result
}

func (u *updateStatusConditionAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	reqLogger := c.log.WithValues("file", u.codelocation, "action", "UpdateStatusConditionAction")

	if isNil(u.instance) {
		err := emperrors.WithStack(ErrNilObject)
		reqLogger.Error(err, "instance is nil")
		return NewExecResult(Error, reconcile.Result{}, err), err
	}

	if isNil(u.conditions) {
		reqLogger.Info("setting default conditions")
		u.conditions = &status.Conditions{}
	}


	key, _ := client.ObjectKeyFromObject(u.instance)
	reqLogger = reqLogger.WithValues("requestType", fmt.Sprintf("%T", u.instance), "key", key)

	if u.conditions.SetCondition(u.condition) {
		reqLogger.Info("updating condition")
		err := c.client.Status().Update(context.TODO(), u.instance)

		if err != nil {
			reqLogger.Error(err, "updating condition")
			return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error while updating status condition")
		}

		return NewExecResult(Requeue, reconcile.Result{Requeue: true}, nil), nil
	}

	reqLogger.Info("no update required")
	return NewExecResult(Continue, reconcile.Result{}, nil), nil
}
