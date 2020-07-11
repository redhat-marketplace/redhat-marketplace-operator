package reconcileutils

import (
	"context"
	"fmt"

	emperrors "emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/codelocation"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)


type getAction struct {
	BaseAction
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
		BaseAction: BaseAction{
			codelocation: codelocation.New(1),
		},
	}
}

func (g *getAction) Bind(r *ExecResult) {
	g.lastResult = r
}

func (g *getAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	reqLogger := c.log.WithValues("file", g.codelocation, "action", "GetAction")

	if isNil(g.Object) {
		err := emperrors.New("object to get is nil")
		reqLogger.Error(err, "updatedObject is nil")
		return NewExecResult(Error, reconcile.Result{}, err), err
	}

	reqLogger = reqLogger.WithValues("requestType", fmt.Sprintf("%T", g.Object), "key", g.NamespacedName)

	err := c.client.Get(ctx, g.NamespacedName, g.Object)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("not found")
			return NewExecResult(NotFound, reconcile.Result{}, err), nil
		}
		if err != nil {
			reqLogger.Error(err, "error getting")
			return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error during get")
		}
	}

	reqLogger.V(2).Info("found")
	return NewExecResult(Continue, reconcile.Result{}, err), nil
}
