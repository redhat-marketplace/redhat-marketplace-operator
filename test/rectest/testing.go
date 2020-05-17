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
	"context"
	"fmt"
	"sync"
	"testing"

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

func (r *ReconcilerTest) GetGetObjects() []runtime.Object {
	return r.runtimeObjs
}

type ReconcileResult struct {
	reconcile.Result
	Err error
}

var testSetupLock sync.Mutex

// NewReconcilerTest creates a new reconciler test with a setup func
// using the provided runtime objects to creat on the client.
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

type ControllerReconcileStep struct {
	stepOptions
	reconcileStepOptions
	*testLine
}

func ReconcileStep(
	stepOptions []StepOption,
	options ...ReconcileStepOption,
) *ControllerReconcileStep{
	stepOpts, _ := newStepOptions(stepOptions...)
	opts, _ := newReconcileStepOptions(options...)

	return &ControllerReconcileStep{
		testLine:             NewTestLine("reconcileStep failure", 3),
		reconcileStepOptions: opts,
		stepOptions:          stepOpts,
	}
}

func (tc *ControllerReconcileStep) GetStepName() string {
	if tc.StepName == "" {
		return "ReconcileStep"
	}
	return tc.StepName
}

func (tc *ControllerReconcileStep) Test(t *testing.T, r *ReconcilerTest) {
	//Reconcile again so Reconcile() checks for the OperatorSource

	if tc.UntilDone {
		tc.Max = 10
	}

	if tc.Max == 0 {
		tc.Max = len(tc.ExpectedResults)
	}

	for i := 0; i < tc.Max; i++ {
		exit := false
		t.Run(fmt.Sprintf("%v_of_%v_expresult", i+1, tc.Max), func(t *testing.T) {
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

func GetStep(
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
	if tc.StepName == "" {
		return "GetStep"
	}
	return tc.StepName
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
		tc.Obj,
	)

	require.NoErrorf(t, err, "get (%T): (%v); err=%+v",
		tc.Obj, err, tc.TestLineError(err))

	if !t.Run("check list result", func(t *testing.T) {
		t.Helper()
		tc.CheckResult(r, t, tc.Obj)
	}) {
		assert.FailNowf(t, "failed get check", "%+v", tc.testLine)
	}
}

type ClientListStep struct {
	*testLine
	stepOptions
	listStepOptions
}

func ListStep(
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
		return "ListStep"
	}
	return tc.StepName
}

func (tc *ClientListStep) Test(t *testing.T, r *ReconcilerTest) {
	t.Helper()
	//Reconcile again so Reconcile() checks for the OperatorSource
	var err error
	err = r.GetClient().List(context.TODO(),
		tc.Obj,
		tc.Filter...,
	)

	if err != nil {
		assert.FailNowf(t, "error encountered",
			"%+v", tc.TestLineError(errors.Errorf("get (%T): (%v)", tc.Obj, err)))
	}

	if !t.Run("check list result", func(t *testing.T) {
		t.Helper()
		tc.CheckResult(r, t, tc.Obj)
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
		testName := fmt.Sprintf("%v %v", testData.GetStepName(), i+1)

		if testName == "" {
			testName = fmt.Sprintf("Step %v", i+1)
		}

		success := t.Run(testName, func(t *testing.T) {
			t.Helper()
			testData.Test(t, r)
		})

		require.Truef(t, success, "Step %s failed", testName)
	}
}
