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
