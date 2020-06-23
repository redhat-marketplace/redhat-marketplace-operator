package reconcileutils

import (
	"context"

	emperrors "emperror.dev/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)


type getAction struct {
	baseAction
	NamespacedName types.NamespacedName
	Object         runtime.Object
	getActionOptions
}

//go:generate go-options -option GetActionOption -prefix GetWith getActionOptions
type getActionOptions struct {
}

func GetAction(
	namespacedName types.NamespacedName,
	object runtime.Object,
	options ...GetActionOption,
) *getAction {
	opts, _ := newGetActionOptions(options...)
	return &getAction{
		NamespacedName:   namespacedName,
		Object:           object,
		getActionOptions: opts,
	}
}

func (g *getAction) Bind(r *ExecResult) {
	g.lastResult = r
}

func (g *getAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {

	if isNil(g.Object) {
		err := emperrors.New("object to get is nil")
		return NewExecResult(Error, reconcile.Result{}, err), err
	}

	err := c.client.Get(ctx, g.NamespacedName, g.Object)
	if err != nil {
		if errors.IsNotFound(err) {
			return NewExecResult(NotFound, reconcile.Result{}, err), nil
		}
		if err != nil {
			return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error during get")
		}
	}

	return NewExecResult(Continue, reconcile.Result{}, err), nil
}
