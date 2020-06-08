package reconcileutils

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ActionResult interface {
	GetResult() ActionResultStatus
	Is(ActionResultStatus) bool
	GetReconcile() reconcile.Result
	GetError() error
	Return() (reconcile.Result, error)
	Object() runtime.Object
}

type ActionResultStatus string

var (
	Continue ActionResultStatus = "continue"
	NotFound ActionResultStatus = "not_found"
	Requeue  ActionResultStatus = "requeue"
	Error    ActionResultStatus = "error"
)

type ExecResult struct {
	Status          ActionResultStatus
	ResultObject    runtime.Object
	ReconcileResult reconcile.Result
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

func (e *ExecResult) Object() runtime.Object {
	return e.ResultObject
}

func NewExecResult(
	status ActionResultStatus,
	resultObject runtime.Object,
	reconcileResult reconcile.Result,
	err error) *ExecResult {
	return &ExecResult{
		Status:          status,
		ResultObject:    resultObject,
		ReconcileResult: reconcileResult,
		Err:             err,
	}
}
