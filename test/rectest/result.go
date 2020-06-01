package rectest

import (
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type AnyReconcileResult ReconcileResult

var AnyResult ReconcileResult = ReconcileResult(AnyReconcileResult{})
var DoneResult ReconcileResult = ReconcileResult{Result: reconcile.Result{}, Err: nil}
var RequeueResult ReconcileResult = ReconcileResult{Result: reconcile.Result{Requeue: true}, Err: nil}

func RangeReconcileResults(result ReconcileResult, n int) []ReconcileResult {
	arr := make([]ReconcileResult, n)

	for i := 0; i < n; i++ {
		arr[i] = result
	}

	return arr
}

func RequeueAfterResult(dur time.Duration) ReconcileResult {
	return ReconcileResult{
		Result: reconcile.Result{RequeueAfter: dur},
	}
}
