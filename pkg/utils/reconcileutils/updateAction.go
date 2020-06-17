package reconcileutils

import (
	"context"
	"fmt"

	emperrors "emperror.dev/errors"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/operator-framework/operator-sdk/pkg/status"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type updateAction struct {
	baseAction
	conditionalUpdate ConditionalUpdateFunction
	updateActionOptions
}

//go:generate go-options -option UpdateActionOption -prefix Update updateActionOptions
type updateActionOptions struct {
	WithStatusCondition UpdateStatusConditionFunc
}

func newPatcher(originalObject runtime.Object,
	patchMaker *patch.PatchMaker,
	updatedObjectFunction UpdateFunction,
) ConditionalUpdateFunction {
	return func() (update bool, updatedObject runtime.Object, err error) {
		new, err := updatedObjectFunction()
		updatedObject = new
		update = false

		if err != nil {
			return
		}

		patchResult, err := patchMaker.Calculate(originalObject, new)

		if err != nil {
			return
		}

		if patchResult.IsEmpty() {
			return
		}

		update = true
		return
	}
}

func UpdateWithPatch(
	originalObject runtime.Object,
	patcher *patch.PatchMaker,
	updateFunc UpdateFunction,
	updateOptions ...UpdateActionOption,
) *updateAction {
	opts, _ := newUpdateActionOptions(updateOptions...)
	return &updateAction{
		conditionalUpdate:   newPatcher(originalObject, patcher, updateFunc),
		updateActionOptions: opts,
	}
}

func UpdateAction(
	updateFunc ConditionalUpdateFunction,
	updateOptions ...UpdateActionOption,
) *updateAction {
	opts, _ := newUpdateActionOptions(updateOptions...)
	return &updateAction{
		conditionalUpdate:   updateFunc,
		updateActionOptions: opts,
	}
}

func (a *updateAction) Bind(result *ExecResult) {
	a.lastResult = result
}

func (a *updateAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	update, updatedObject, err := a.conditionalUpdate()

	withCondition := func(result *ExecResult, err error) (*ExecResult, error) {
		return result, err
	}

	if a.WithStatusCondition != nil {
		withCondition = func(result *ExecResult, err error) (*ExecResult, error) {
			statusUpdater := UpdateStatusCondition(a.WithStatusCondition)
			statusUpdater.Bind(result)
			statusUpdater.Exec(ctx, c)
			return result, err
		}
	}

	if updatedObject == nil {
		err := emperrors.New("object to update is nil")
		return NewExecResult(Error, reconcile.Result{}, err), err
	}

	if err != nil {
		c.log.Error(err, "failed to get object for update")
		return withCondition(NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error updating object"))
	}

	reqLogger := c.log.WithValues("requestType", fmt.Sprintf("%T", updatedObject))

	if update {
		err := c.client.Update(ctx, updatedObject)

		if err != nil {
			reqLogger.Error(err, "error updating object")
			return withCondition(NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error applying update"))
		}

		reqLogger.V(2).Info("updated object")
		return withCondition(NewExecResult(Requeue, reconcile.Result{}, nil), nil)
	}

	return withCondition(NewExecResult(Continue, reconcile.Result{}, nil), nil)
}

type updateStatusConditionAction struct {
	updateCondition UpdateStatusConditionFunc
	baseAction
}

func UpdateStatusCondition(
	updateCondition UpdateStatusConditionFunc,
) *updateStatusConditionAction {
	return &updateStatusConditionAction{
		updateCondition: updateCondition,
	}
}

func (u *updateStatusConditionAction) Bind(result *ExecResult) {
	u.lastResult = result
}

func (u *updateStatusConditionAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	update, instance, conditions, condition := u.updateCondition(u.lastResult, u.lastResult.Err)

	if update {
		if instance == nil {
			err := emperrors.New("instance is nil")
			return NewExecResult(Error, reconcile.Result{}, err), err
		}
		if conditions == nil {
			conditions = &status.Conditions{}
		}
		if conditions.SetCondition(condition) {
			err := c.client.Status().Update(context.TODO(), instance)

			if err != nil {
				return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error while updating status condition")
			}

			return NewExecResult(Requeue, reconcile.Result{Requeue: true}, nil), nil
		}
	}

	return NewExecResult(Continue, reconcile.Result{}, nil), nil
}
