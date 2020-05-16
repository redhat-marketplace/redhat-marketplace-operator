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

package testing

import (
	"context"
	"fmt"
	"io"
	gruntime "runtime"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// - interfaces -
type ReconcilerTestValidationFunc func(*ReconcilerTest, *testing.T, runtime.Object)
type ReconcilerSetupFunc func(*ReconcilerTest) error

type TestCaseStep interface {
	GetStepName() string
	Test(t *testing.T, reconcilerTest *ReconcilerTest)
}

// - end of interfaces -

// ReconcilerTest is the major test driver, create one of these for each test
type ReconcilerTest struct {
	runtimeObjs []runtime.Object
	SetupFunc   ReconcilerSetupFunc
	Reconciler  reconcile.Reconciler
	Client      client.Client
}

func (r *ReconcilerTest) SetReconciler(re reconcile.Reconciler) {
	r.Reconciler = re
}

func (r *ReconcilerTest) SetClient(c client.Client) {
	r.Client = c
}

func (r *ReconcilerTest) GetReconciler() reconcile.Reconciler {
	return r.Reconciler
}

func (r *ReconcilerTest) GetClient() client.Client {
	return r.Client
}

func (r *ReconcilerTest) GetRuntimeObjects() []runtime.Object {
	return r.runtimeObjs
}

//go:generate go-options -imports=sigs.k8s.io/controller-runtime/pkg/reconcile -option StepOption -prefix With stepOptions
type stepOptions struct {
	StepName string
	Request  reconcile.Request
}

//go:generate go-options -option ReconcileStepOption -prefix With reconcileStepOptions
type reconcileStepOptions struct {
	ExpectedResults []ReconcileResult `options:"..."`
	UntilDone       bool
	Max             int
}

type ReconcileResult struct {
	reconcile.Result
	Err error
}

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

//go:generate go-options -imports=k8s.io/apimachinery/pkg/runtime -option GetStepOption -prefix With getStepOptions
type getStepOptions struct {
	NamespacedName struct {
		Name, Namespace string
	}
	RuntimeObj     runtime.Object
	Labels         map[string]string            `options:",map[string]string{}"`
	CheckGetResult ReconcilerTestValidationFunc `options:",Ignore"`
}

//go:generate go-options -imports=k8s.io/apimachinery/pkg/runtime,sigs.k8s.io/controller-runtime/pkg/client -option ListStepOption -prefix With listStepOptions
type listStepOptions struct {
	ListObj         runtime.Object
	ListOptions     []client.ListOption          `options:"..."`
	CheckListResult ReconcilerTestValidationFunc `options:",Ignore"`
}

type testLine struct {
	msg   string
	stack errors.StackTrace
	err   error
}

func (t *testLine) Error() string {
	return t.msg
}

func NewTestLine(message string, up int) *testLine {
	pc := make([]uintptr, 1)
	n := gruntime.Callers(up, pc)

	if n == 0 {
		return &testLine{msg: message}
	}

	trace := make(errors.StackTrace, len(pc))

	for i, ptr := range pc {
		trace[i] = errors.Frame(ptr)
	}

	return &testLine{msg: message, stack: trace}
}

func (t *testLine) TestLineError(err error) error {
	if err == nil {
		return nil
	}

	t.err = err
	return t
}

func (t *testLine) Unwrap() error { return t.err }

func (t *testLine) Cause() error { return t.err }

func (t *testLine) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%+v", t.Cause())
			t.stack.Format(s, verb)
			return
		}
		fallthrough
	case 's':
		io.WriteString(s, t.Error())
	case 'q':
		fmt.Fprintf(s, "%q", t.Error())
	}
}

var testSetupLock sync.Mutex

// NewReconcilerTest creates a new reconciler test with a setup func
func NewReconcilerTest(setup ReconcilerSetupFunc, predefinedObjs ...runtime.Object) *ReconcilerTest {
	testSetupLock.Lock()
	defer testSetupLock.Unlock()
	myObjs := []runtime.Object{}

	for _, obj := range predefinedObjs {
		myObjs = append(myObjs, obj.DeepCopyObject())
	}

	return &ReconcilerTest{
		runtimeObjs: myObjs,
		SetupFunc:   setup,
	}
}

func Ignore(r *ReconcilerTest, t *testing.T, obj runtime.Object) {}

type ReconcileStep struct {
	stepOptions
	reconcileStepOptions
	*testLine
}

func NewReconcileStep(
	stepOptions []StepOption,
	options ...ReconcileStepOption,
) *ReconcileStep {
	stepOpts, _ := newStepOptions(stepOptions...)
	opts, _ := newReconcileStepOptions(options...)

	return &ReconcileStep{
		testLine:             NewTestLine("reconcileStep failure", 3),
		reconcileStepOptions: opts,
		stepOptions:          stepOpts,
	}
}

func (tc *ReconcileStep) GetStepName() string {
	if tc.StepName == "" {
		return "Reconcile"
	}
	return tc.StepName
}

func (tc *ReconcileStep) Test(t *testing.T, r *ReconcilerTest) {
	//Reconcile again so Reconcile() checks for the OperatorSource

	if tc.UntilDone {
		tc.Max = 10
	}

	if tc.Max == 0 {
		tc.Max = len(tc.ExpectedResults)
	}

	for i := 0; i < tc.Max; i++ {
		exit := false
		t.Run(fmt.Sprintf("reconcileResult/%v_of_%v", i+1, tc.Max), func(t *testing.T) {
			res, err := r.Reconciler.Reconcile(tc.Request)
			result := ReconcileResult{res, err}

			expectedResult := AnyResult

			if i < len(tc.ExpectedResults) {
				expectedResult = tc.ExpectedResults[i]
			}

			if expectedResult != AnyResult {
				assert.Equalf(t, expectedResult, result,
					"%+v", tc.TestLineError(fmt.Errorf("incorrect expected result")))
			} else {
				// stop if done or if there was an error
				if result == DoneResult {
					if len(tc.ExpectedResults) != 0 && i > len(tc.ExpectedResults)-1 && !tc.UntilDone {
						assert.Equalf(t, len(tc.ExpectedResults)-1, i,
							"%+v", tc.TestLineError(fmt.Errorf("expected reconcile count did not match")))
					}
					t.Logf("reconcile completed in %v turns", i+1)
					exit = true
				}

				if err != nil {
					assert.Equalf(t, DoneResult, result,
						"%+v", tc.TestLineError(fmt.Errorf("error while reconciling")))
					exit = true
				}

				if i == tc.Max-1 {
					assert.Equalf(t, DoneResult, result,
						"%+v", tc.TestLineError(fmt.Errorf("did not successfully reconcile")))
					exit = true
				}
			}
		})
		if exit {
			break
		}
	}
}

type ClientGetStep struct {
	*testLine
	stepOptions
	getStepOptions
}

func NewClientGetStep(
	stepOptions []StepOption,
	options ...GetStepOption,
) *ClientGetStep {
	stepOpts, _ := newStepOptions(stepOptions...)
	getOpts, _ := newGetStepOptions(options...)
	return &ClientGetStep{
		testLine:       NewTestLine("failed client get step", 3),
		stepOptions:    stepOpts,
		getStepOptions: getOpts,
	}
}

func (tc *ClientGetStep) GetStepName() string {
	nsn := tc.getStepOptions.NamespacedName
	if tc.StepName == "" {
		if nsn.Namespace == "" {
			return nsn.Name
		}
		return "ClientGetStep/" + nsn.Namespace + "/" + nsn.Name
	}
	return "ClientGetStep/" + tc.StepName
}

func (tc *ClientGetStep) Test(t *testing.T, r *ReconcilerTest) {
	//Reconcile again so Reconcile() checks for the OperatorSource
	t.Helper()
	var err error
	err = r.GetClient().Get(
		context.TODO(),
		types.NamespacedName{
			Name:      tc.NamespacedName.Name,
			Namespace: tc.NamespacedName.Namespace,
		},
		tc.RuntimeObj,
	)

	require.NoErrorf(t, err, "get (%T): (%v); err=%+v",
		tc.RuntimeObj, err, tc.TestLineError(err))

	if !t.Run("check list result", func(t *testing.T) {
		t.Helper()
		tc.CheckGetResult(r, t, tc.RuntimeObj)
	}) {
		assert.FailNowf(t, "failed get check", "%+v", tc.testLine)
	}
}

type ClientListStep struct {
	*testLine
	stepOptions
	listStepOptions
}

func NewClientListStep(
	stepOptions []StepOption,
	options ...ListStepOption,
) *ClientListStep {
	stepOpts, _ := newStepOptions(stepOptions...)
	listOpts, _ := newListStepOptions(options...)
	return &ClientListStep{
		testLine:        NewTestLine("failed client list step", 3),
		stepOptions:     stepOpts,
		listStepOptions: listOpts,
	}
}

func (tc *ClientListStep) GetStepName() string {
	if tc.StepName == "" {
		return "ClientListStep"
	}
	return "ClientListStep/" + tc.StepName
}

func (tc *ClientListStep) Test(t *testing.T, r *ReconcilerTest) {
	t.Helper()
	//Reconcile again so Reconcile() checks for the OperatorSource
	var err error
	err = r.GetClient().List(context.TODO(),
		tc.ListObj,
		tc.ListOptions...,
	)

	if err != nil {
		assert.FailNowf(t, "error encountered",
			"%+v", tc.TestLineError(errors.Errorf("get (%T): (%v)", tc.ListObj, err)))
	}

	if !t.Run("check list result", func(t *testing.T) {
		t.Helper()
		tc.CheckListResult(r, t, tc.ListObj)
	}) {
		assert.FailNowf(t, "failed check list", "%+v", tc.testLine)
	}
}

var testAllMutex sync.Mutex

func (r *ReconcilerTest) TestAll(t *testing.T, testCases ...TestCaseStep) {
	t.Helper()
	if r.SetupFunc != nil {
		testAllMutex.Lock()
		err := r.SetupFunc(r)
		testAllMutex.Unlock()

		if err != nil {
			t.Fatalf("failed to setup test %v", err)
		}
	}

	for i, testData := range testCases {
		testName := fmt.Sprintf("%v %v", testData.GetStepName(), i)

		if testName == "" {
			testName = fmt.Sprintf("Step %v", i)
		}

		success := t.Run(testName, func(t *testing.T) {
			t.Helper()
			testData.Test(t, r)
		})

		require.Truef(t, success, "Step %s failed", testName)
	}
}
