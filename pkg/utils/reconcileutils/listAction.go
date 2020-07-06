package reconcileutils

import (
	"context"

	emperrors "emperror.dev/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type listAction struct {
	list    runtime.Object
	filters []client.ListOption
	BaseAction
}

func ListAction(list runtime.Object, filters ...client.ListOption) *listAction {
	return &listAction{
		list:    list,
		filters: filters,
	}
}

func (l *listAction) Bind(result *ExecResult) {
	l.lastResult = result
}

func (l *listAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	err := c.client.List(ctx, l.list, l.filters...)

	if err != nil {
		return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error while listing")
	}

	return NewExecResult(Continue, reconcile.Result{}, nil), nil
}
