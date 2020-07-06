package reconcileutils

import (
	"context"

	emperrors "emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/codelocation"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type call struct {
	BaseAction
	call func() (ClientAction, error)
}

func Call(callAction func() (ClientAction, error)) ClientAction {
	return &call{
		call: callAction,
		BaseAction: BaseAction{
			codelocation: codelocation.New(1),
		},
	}
}

func (i *call) Bind(result *ExecResult) {
	i.lastResult = result
}

func (i *call) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	logger := c.log.WithValues("file", i.codelocation, "action", "Call")
	action, err := i.call()

	if err != nil {
		logger.Error(err, "call action had an error")
		return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error on call")
	}

	if isNil(action) {
		logger.V(2).Info("call had no action to perform")
		return NewExecResult(Continue, reconcile.Result{}, nil), nil
	}

	action.Bind(i.lastResult)
	logger.V(4).Info("executing action")
	return action.Exec(ctx, c)
}

func Do(actions ...ClientAction) ClientAction {
	return &do{
		Actions: actions,
		BaseAction: BaseAction{
			codelocation: codelocation.New(1),
		},
	}
}

type do struct {
	BaseAction
	Actions []ClientAction
}

func (i *do) Bind(result *ExecResult) {
	i.lastResult = result
}

func (i *do) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	logger := c.log.WithValues("file", i.codelocation, "action", "Do")

	var err error
	result := i.lastResult
	for _, action := range i.Actions {
		action.Bind(result)
		result, err = action.Exec(ctx, c)

		if result == nil {
			err := emperrors.New("result should not be nil")
			logger.Error(err, "result should not be nil")
			return NewExecResult(Error, reconcile.Result{}, err), err
		}

		logger.V(2).Info("action returned result", "result", *result)
		switch result.Status {
		case Error:
			logger.Info("returning error", "err", err)
			return result, emperrors.Wrap(err, "error executing do")
		case Requeue:
			logger.Info("returning requeue")
			return result, nil
		}
	}

	logger.Info("final result occurred", "result", *result)
	return result, nil
}

type storeResult struct {
	BaseAction
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
	BaseAction
	Action   ClientAction
	Branches []ClientActionBranch
}

// HandleResult will return original results on
// error or requeue, but will return the handled action
// branch on NotFound and Continue.
func HandleResult(
	action ClientAction,
	branches ...ClientActionBranch,
) ClientAction {
	return &handleResult{
		Action:   action,
		Branches: branches,
		BaseAction: BaseAction{
			codelocation: codelocation.New(1),
		},
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
	logger := c.log.WithValues("file", r.codelocation, "action", "HandleResult")
	r.Action.Bind(r.lastResult)
	myVar, err := r.Action.Exec(ctx, c)

	for _, branch := range r.Branches {
		if myVar.Is(branch.Status) {
			logger.Info("branch matched", "status", branch.Status)
			var2, err := branch.Action.Exec(ctx, c)

			if myVar.Is(Error) {
				logger.Info("returning original error")
				return myVar, myVar.Err
			}

			if myVar.Is(Requeue) {
				logger.Info("returning original requeue")
				return myVar, nil
			}

			return var2, err
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

func (c *ClientCommand) Log() logr.Logger {
	return c.log
}
