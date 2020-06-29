package reconcileutils

import (
	"context"

	emperrors "emperror.dev/errors"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

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

type handleResult struct {
	baseAction
	Action   ClientAction
	Branches []ClientActionBranch
}

func HandleResult(
	action ClientAction,
	branches ...ClientActionBranch,
) ClientAction {
	return &handleResult{
		Action:   action,
		Branches: branches,
	}
}

func OnRequeue(action ClientAction) ClientActionBranch {
	return ClientActionBranch{
		Status: Requeue,
		Action: action,
	}
}

func OnError(action ClientAction) ClientActionBranch {
	return ClientActionBranch{
		Status: Error,
		Action: action,
	}
}

func OnNotFound(action ClientAction) ClientActionBranch {
	return ClientActionBranch{
		Status: NotFound,
		Action: action,
	}
}

func OnContinue(action ClientAction) ClientActionBranch {
	return ClientActionBranch{
		Status: Continue,
		Action: action,
	}
}

func (r *handleResult) Bind(result *ExecResult) {
	r.lastResult = result
}

func (r *handleResult) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	r.Action.Bind(r.lastResult)
	myVar, err := r.Action.Exec(ctx, c)

	for _, branch := range r.Branches {
		if myVar.Is(branch.Status) {
			return branch.Action.Exec(ctx, c)
		}
	}

	return myVar, err
}

type ClientResultCollector struct {
	results map[string]*ExecResult
}

func NewCollector() *ClientResultCollector {
	return &ClientResultCollector{
		results: make(map[string]*ExecResult),
	}
}

func (r *ClientResultCollector) NextPointer(name string) *ExecResult {
	if val, ok := r.results[name]; ok {
		return val
	}

	result := &ExecResult{}
	r.results[name] = result
	return result
}

func (r *ClientResultCollector) Get(name string) *ExecResult {
	return r.results[name]
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
) ClientCommandRunner {
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

func (c *ClientCommand) Exec(
	ctx context.Context,
	action ClientAction,
) (*ExecResult, error) {
	return action.Exec(ctx, c)
}
