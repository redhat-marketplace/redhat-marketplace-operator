package reconcileutils

import (
	"context"
	"fmt"

	emperrors "emperror.dev/errors"
	"github.com/operator-framework/operator-sdk/pkg/status"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/patch"
	"k8s.io/apimachinery/pkg/runtime"
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
	}
}

func (a *updateAction) Bind(result *ExecResult) {
	a.lastResult = result
}

func (a *updateAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	updatedObject := a.updateObject

	if isNil(updatedObject) {
		err := emperrors.WithStack(ErrNilObject)
		return NewExecResult(Error, reconcile.Result{}, err), err
	}

	reqLogger := c.log.WithValues("requestType", fmt.Sprintf("%T", updatedObject))
	err := c.client.Update(ctx, updatedObject)

	if err != nil {
		reqLogger.Error(err, "error updating object")
		return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error applying update")
	}

	reqLogger.V(2).Info("updated object")
	return NewExecResult(Requeue, reconcile.Result{}, nil), nil
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
	}
}

func (u *updateStatusConditionAction) Bind(result *ExecResult) {
	u.lastResult = result
}

func (u *updateStatusConditionAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	if isNil(u.instance) {
		err := emperrors.WithStack(ErrNilObject)
		return NewExecResult(Error, reconcile.Result{}, err), err
	}

	if isNil(u.conditions) {
		u.conditions = &status.Conditions{}
	}

	if u.conditions.SetCondition(u.condition) {
		err := c.client.Status().Update(context.TODO(), u.instance)

		if err != nil {
			return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error while updating status condition")
		}

		return NewExecResult(Requeue, reconcile.Result{Requeue: true}, nil), nil
	}

	return NewExecResult(Continue, reconcile.Result{}, nil), nil
}
