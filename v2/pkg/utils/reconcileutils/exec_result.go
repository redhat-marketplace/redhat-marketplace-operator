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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ActionResultStatus string

var (
	Return   ActionResultStatus = "return"
	Continue ActionResultStatus = "continue"
	NotFound ActionResultStatus = "not_found"
	Requeue  ActionResultStatus = "requeue"
	Error    ActionResultStatus = "error"
	Noop     ActionResultStatus = "noop"
)

type ExecResult struct {
	Status          ActionResultStatus
	ReconcileResult reconcile.Result
	Action          *BaseAction
	Err             error
}

func (e *ExecResult) GetResult() ActionResultStatus {
	return e.Status
}

func (e *ExecResult) Is(comp ActionResultStatus) bool {
	return e.Status == comp
}

func (e *ExecResult) GetReconcile() reconcile.Result {
	return e.ReconcileResult
}

func (e *ExecResult) GetError() error {
	return e.Err
}

func (e *ExecResult) Return() (reconcile.Result, error) {
	return e.ReconcileResult, e.Err
}

func (e *ExecResult) ReturnWithError(err error) (reconcile.Result, error) {
	return e.ReconcileResult, err
}

func (e *ExecResult) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return ""
}

func NewExecResult(
	status ActionResultStatus,
	reconcileResult reconcile.Result,
	action *BaseAction,
	err error) *ExecResult {
	return &ExecResult{
		Status:          status,
		ReconcileResult: reconcileResult,
		Action:          action,
		Err:             err,
	}
}
