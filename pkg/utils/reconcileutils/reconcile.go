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

func (m *mapAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	c.log.V(0).Info("entering do")
	result := m.lastResult

	for _, action := range m.Actions {
		action.Bind(result)
		result, _ = action.Exec(ctx, c)
		m.Results = append(m.Results, result)
	}

	return NewExecResult(Continue, reconcile.Result{}, nil), nil
}

func (m *mapAction) Bind(result *ExecResult) {
	m.lastResult = result
}

func Map(results []*ExecResult, actions ...ClientAction) ClientAction {
	return &mapAction{
		Results: results,
		Actions: actions,
	}
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
	result := i.lastResult
	var err error

	for _, action := range i.Actions {
		action.Bind(result)
		result, err = action.Exec(ctx, c)

		switch result.Status {
		case Error:
			return result, emperrors.Wrap(err, "error executing do")
		case Requeue:
			return result, nil
		}
	}
	c.log.V(0).Info("result is", "result", result)
	return result, nil
}


func If(condition ConditionFunc, actions ...ClientAction) ClientAction {
	return &IfConditional{
		Condition: condition,
		Then:      actions,
	}
}

type IfConditional struct {
	baseAction
	Condition ConditionFunc
	Then      []ClientAction
}

func (i *IfConditional) Bind(result *ExecResult) {
	i.lastResult = result
}

func (i *IfConditional) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	c.log.V(0).Info("entering if")
	if i.Condition != nil && i.Condition(i.lastResult) {
		c.log.V(0).Info("executing if")
		return Do(i.Then...).Exec(ctx, c)
	}

	c.log.V(0).Info("no op")
	return NewExecResult(Continue, reconcile.Result{}, nil), nil
}

type StoreResult struct {
	baseAction
	Var    *ExecResult
	Err    error
	Action ClientAction
}

func (r *StoreResult) Bind(result *ExecResult) {
	r.lastResult = result
}

func (r *StoreResult) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	r.Action.Bind(r.lastResult)
	myVar, err := r.Action.Exec(ctx, c)
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
