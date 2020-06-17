package reconcileutils

import (
	"context"

	emperrors "emperror.dev/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type deleteAction struct {
	obj           runtime.Object
	baseAction
	deleteActionOptions
}

//go:generate go-options -option DeleteActionOption -imports=sigs.k8s.io/controller-runtime/pkg/client -prefix Delete deleteActionOptions
type deleteActionOptions struct {
	WithStatusCondition UpdateStatusConditionFunc
	WithDeleteOptions []client.DeleteOption `options:"..."`
}

func DeleteAction(
	obj runtime.Object,
	opts ...DeleteActionOption,
) *deleteAction {
	deleteOpts, _ := newDeleteActionOptions(opts...)
	return &deleteAction{
		obj:           obj,
		deleteActionOptions: deleteOpts,
	}
}

func (d *deleteAction) Bind(result *ExecResult) {
	d.lastResult = result
}

func (d *deleteAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	withCondition := func(result *ExecResult, err error) (*ExecResult, error) {
		return result, err
	}

	if d.WithStatusCondition != nil {
		withCondition = func(result *ExecResult, err error) (*ExecResult, error) {
			statusUpdater := UpdateStatusCondition(d.WithStatusCondition)
			statusUpdater.Bind(result)
			statusUpdater.Exec(ctx, c)
			return result, err
		}
	}

	if d.obj == nil {
		err := emperrors.New("object to delete is nil")
		return withCondition(NewExecResult(Error, reconcile.Result{}, err), err)
	}

	err := c.client.Delete(ctx, d.obj, d.WithDeleteOptions...)

	if err != nil {
		return withCondition(NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error while deleting"))
	}

	return withCondition(NewExecResult(Continue, reconcile.Result{}, nil), nil)
}
