package reconcileutils

import (
	"context"

	emperrors "emperror.dev/errors"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type mapAction struct {
	Results []*ExecResult
	Actions []ClientAction
	baseAction
}

type call struct {
	baseAction
	call func() (ClientAction, error)
}

func Call(callAction func() (ClientAction, error)) ClientAction {
	return &call{
		call: callAction,
	}
}

func (i *call) Bind(result *ExecResult) {
	i.lastResult = result
}

func (i *call) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	c.log.V(0).Info("entering call")
	action, err := i.call()

	if err != nil {
		return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error on call")
	}

	if isNil(action) {
		return NewExecResult(Continue, reconcile.Result{}, nil), nil
	}

	action.Bind(i.lastResult)
	return action.Exec(ctx, c)
}

func Do(actions ...ClientAction) ClientAction {
	return &do{
		Actions: actions,
	}
}

type do struct {
	baseAction
	Actions []ClientAction
}

func (i *do) Bind(result *ExecResult) {
	i.lastResult = result
}

func (i *do) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	c.log.V(0).Info("entering do")
	var err error
	result := i.lastResult
	for _, action := range i.Actions {
		action.Bind(result)
		result, err = action.Exec(ctx, c)

		if result != nil {
			switch result.Status {
			case Error:
				return result, emperrors.Wrap(err, "error executing do")
			case Requeue:
				return result, nil
			}
		}
	}
	c.log.V(0).Info("result is", "result", result)
	return result, nil
}

type storeResult struct {
	baseAction
	Var    *ExecResult
	Err    error
	Action ClientAction
}

func StoreResult(result *ExecResult, action ClientAction) ClientAction {
	return &storeResult{
		Var:    result,
		Action: action,
	}
}

func (r *storeResult) Bind(result *ExecResult) {
	r.lastResult = result
}

func (r *storeResult) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	r.Action.Bind(r.lastResult)
	myVar, err := r.Action.Exec(ctx, c)

	if r.Var == nil || myVar == nil {
		return NewExecResult(Error, reconcile.Result{}, nil), emperrors.New("vars are nil")
	}

	*r.Var = *myVar
	r.Err = err
	return myVar, err
}

type ClientCommand struct {
	client client.Client
	scheme *runtime.Scheme
	log    logr.Logger
}

func NewClientCommand(
	client client.Client,
	scheme *runtime.Scheme,
	log logr.Logger,
) *ClientCommand {
	return &ClientCommand{
		client: client,
		scheme: scheme,
		log:    log,
	}
}

func (c *ClientCommand) Do(
	ctx context.Context,
	actions ...ClientAction,
) (*ExecResult, error) {
	return Do(actions...).Exec(ctx, c)
}
