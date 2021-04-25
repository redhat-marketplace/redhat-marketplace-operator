// Copyright 2020 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reconcileutils

import (
	"context"
	"time"

	emperrors "emperror.dev/errors"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type call struct {
	*BaseAction
	call func() (ClientAction, error)
}

func Call(callAction func() (ClientAction, error)) ClientAction {
	return &call{
		call:       callAction,
		BaseAction: NewBaseAction("Call"),
	}
}

func (i *call) Bind(result *ExecResult) {
	i.LastResult = result
}

func (i *call) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	logger := c.log.WithValues("file", i.CodeLocation, "action", "Call")
	action, err := i.call()

	if err != nil {
		logger.Error(err, "call action had an error")
		return NewExecResult(Error, reconcile.Result{}, i.BaseAction, err), emperrors.Wrap(err, "error on call")
	}

	if isNil(action) {
		logger.V(2).Info("call had no action to perform")
		return NewExecResult(Continue, reconcile.Result{}, i.BaseAction, nil), nil
	}

	action.Bind(i.LastResult)
	logger.V(4).Info("executing action")
	return action.Exec(ctx, c)
}

func Do(actions ...ClientAction) ClientAction {
	return &do{
		Actions:    actions,
		BaseAction: NewBaseAction("Do"),
	}
}

func internalDo(actions ...ClientAction) ClientAction {
	return &do{
		Actions:    actions,
		BaseAction: NewBaseAction("Do"),
	}
}

type do struct {
	*BaseAction
	Actions []ClientAction
}

func (i *do) Bind(result *ExecResult) {
	i.LastResult = result
}

func (i *do) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	logger := c.log.WithValues("file", i.CodeLocation, "action", "Do")

	if len(i.Actions) == 0 {
		return NewExecResult(Continue, reconcile.Result{}, i.BaseAction, nil), nil
	}

	var err error
	result := i.LastResult
	for _, action := range i.Actions {
		action.Bind(result)
		result, err = action.Exec(ctx, c)

		if err != nil {
			logger.Error(err, "error from action")
			return NewExecResult(Error, reconcile.Result{}, i.BaseAction, err), err
		}

		if result == nil {
			err = emperrors.New("result should not be nil")
			return NewExecResult(Error, reconcile.Result{}, i.BaseAction, err), err
		}

		switch result.Status {
		case Error:
			logger.Error(err, "action errored")
			return result, emperrors.Wrap(err, "error executing do")
		case Requeue:
			logger.V(4).Info("returning requeue")
			return result, nil
		}
	}

	if result != nil {
		logger.V(4).Info("action returned result", "result", *result)
	}
	return result, nil
}

func RetryConflict(actions ...ClientAction) ClientAction {
	return &retryAction{
		BaseAction: NewBaseAction("RetryConflict"),
		Actions:    actions,
		Backoff:    retry.DefaultRetry,
		RetryError: errors.IsConflict,
	}
}

func Retry(
	backoff wait.Backoff,
	retryError func(error) bool,
	actions ...ClientAction) ClientAction {
	return &retryAction{
		BaseAction: NewBaseAction("Retry"),
		Actions:    actions,
		Backoff:    backoff,
		RetryError: retryError,
	}
}

type retryAction struct {
	*BaseAction
	Actions    []ClientAction
	RetryError func(err error) bool
	Backoff    wait.Backoff
}

func (i *retryAction) Bind(result *ExecResult) {
	i.LastResult = result
}

func (i *retryAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	var result *ExecResult
	var err error
	err = retry.OnError(
		retry.DefaultBackoff,
		i.RetryError,
		func() error {
			var err error
			result, err = internalDo(i.Actions...).Exec(ctx, c)
			if result.Is(Error) && err != nil {
				return err
			}
			return nil
		},
	)

	if err != nil {
		return result, nil
	}

	return NewExecResult(Error, reconcile.Result{}, nil, err), err
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
	r.LastResult = result
}

func (r *storeResult) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	r.Action.Bind(r.LastResult)
	myVar, err := r.Action.Exec(ctx, c)

	if r.Var == nil || myVar == nil {
		return NewExecResult(Error, reconcile.Result{}, nil, nil), emperrors.New("vars are nil")
	}

	*r.Var = *myVar
	r.Err = err
	return myVar, err
}

type ReturnResponse struct {
	*BaseAction
	*ExecResult
}

func (r *ReturnResponse) Bind(result *ExecResult) {
	r.LastResult = result
}

func (r *ReturnResponse) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	r.ExecResult.Action = r.BaseAction
	return r.ExecResult, nil
}

func ReturnFinishedResult() *ReturnResponse {
	action := NewBaseAction("returnFinishedResult")
	return &ReturnResponse{
		BaseAction: action,
		ExecResult: NewExecResult(Return, reconcile.Result{}, action, nil),
	}
}

func RequeueResponse() *ReturnResponse {
	action := NewBaseAction("requeueReponse")
	return &ReturnResponse{
		BaseAction: NewBaseAction("requeueReponse"),
		ExecResult: NewExecResult(Requeue, reconcile.Result{Requeue: true}, action, nil),
	}
}

func RequeueAfterResponse(d time.Duration) *ReturnResponse {
	action := NewBaseAction("requeueReponse")
	return &ReturnResponse{
		BaseAction: action,
		ExecResult: NewExecResult(Requeue, reconcile.Result{RequeueAfter: d}, nil, nil),
	}
}

func ReturnWithError(err error) *ReturnResponse {
	action := NewBaseAction("errorReponse")
	return &ReturnResponse{
		BaseAction: action,
		ExecResult: NewExecResult(Error, reconcile.Result{}, nil, err),
	}
}

func ContinueResponse() *ReturnResponse {
	action := NewBaseAction("continueReponse")
	return &ReturnResponse{
		BaseAction: action,
		ExecResult: NewExecResult(Continue, reconcile.Result{}, nil, nil),
	}
}

type handleResult struct {
	*BaseAction
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
		Action:     action,
		Branches:   branches,
		BaseAction: NewBaseAction("HandleResult"),
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

func OnAny(action ClientAction) ClientActionBranch {
	return ClientActionBranch{
		Any:    true,
		Action: action,
	}
}

func (r *handleResult) Bind(result *ExecResult) {
	r.LastResult = result
}

func (r *handleResult) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	logger := r.GetReqLogger(c)
	r.Action.Bind(r.LastResult)
	myVar, err := r.Action.Exec(ctx, c)

	for _, branch := range r.Branches {
		if myVar.Is(branch.Status) {
			logger.V(2).Info("branch matched", "status", branch.Status)

			if branch.Action == nil || branch.Any {
				return myVar, err
			}

			var2, err := branch.Action.Exec(ctx, c)

			if err != nil {
				logger.Error(err, "error occurred on branch")
				return var2, err
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

func NewLoglessClientCommand(
	client client.Client,
	scheme *runtime.Scheme,
) ClientCommandRunner {
	return &ClientCommand{
		client: client,
		scheme: scheme,
		log:    logf.Log.WithName("clientcommand"),
	}
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
	return internalDo(actions...).Exec(ctx, c)
}

func (c *ClientCommand) Exec(
	ctx context.Context,
	action ClientAction,
) (*ExecResult, error) {
	return action.Exec(ctx, c)
}

func (c *ClientCommand) GetScheme() *runtime.Scheme {
	return c.scheme
}

func (c *ClientCommand) Log() logr.Logger {
	return c.log
}
