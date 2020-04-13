package testing

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ReconcilerTestValidationFunc func(*ReconcilerTest, *testing.T, runtime.Object)
type ReconcilerSetupFunc func(*ReconcilerTest) error

type ReconcilerTest struct {
	SetupFunc ReconcilerSetupFunc
	Reconciler reconcile.Reconciler
	Client     client.Client
}

func NewReconcilerTest(setup ReconcilerSetupFunc) *ReconcilerTest {
	return &ReconcilerTest{
		SetupFunc: setup,
	}

}

func Ignore(r *ReconcilerTest, t *testing.T, obj runtime.Object) {}

type ReconcilerTestCase struct {
	TestName       string
	Request        reconcile.Request
	NamespacedName types.NamespacedName
	TestObj        runtime.Object
	ExpectedResult reconcile.Result
	ExpectedError  error
	ValidationFunc ReconcilerTestValidationFunc
}

type ReconcilerTestCaseBuilder struct {
	tc ReconcilerTestCase
}

type ReconcilerTestCaseOption func(*ReconcilerTestCase)

func NewReconcilerTestCase(opts ...ReconcilerTestCaseOption) *ReconcilerTestCase {
	tc := &ReconcilerTestCase{
		NamespacedName: types.NamespacedName{},
		ExpectedResult: reconcile.Result{Requeue: true},
		ValidationFunc: Ignore,
	}
	for _, opt := range opts {
		opt(tc)
	}
	return tc
}

func WithTestName(name string) ReconcilerTestCaseOption {
	return func(tc *ReconcilerTestCase) {
		tc.TestName = name
	}
}

func WithRequest(r reconcile.Request) ReconcilerTestCaseOption {
	return func(tc *ReconcilerTestCase) {
		tc.Request = r
	}
}

func WithNamespacedName(n types.NamespacedName) ReconcilerTestCaseOption {
	return func(tc *ReconcilerTestCase) {
		tc.NamespacedName = n
	}
}

func WithName(n string) ReconcilerTestCaseOption {
	return func(tc *ReconcilerTestCase) {
		tc.NamespacedName.Name = n
	}
}

func WithNamespace(n string) ReconcilerTestCaseOption {
	return func(tc *ReconcilerTestCase) {
		tc.NamespacedName.Namespace = n
	}
}

func WithTestObj(o runtime.Object) ReconcilerTestCaseOption {
	return func(tc *ReconcilerTestCase) {
		tc.TestObj = o
	}
}

func WithExpectedResult(r reconcile.Result) ReconcilerTestCaseOption {
	return func(tc *ReconcilerTestCase) {
		tc.ExpectedResult = r
	}
}

func WithExpectedError(err error) ReconcilerTestCaseOption {
	return func(tc *ReconcilerTestCase) {
		tc.ExpectedError = err
	}
}

func WithValidationFunc(r ReconcilerTestValidationFunc) ReconcilerTestCaseOption {
	return func(tc *ReconcilerTestCase) {
		tc.ValidationFunc = r
	}
}

func WithNoObj() ReconcilerTestCaseOption {
	return func(tc *ReconcilerTestCase) {
		tc.TestObj = nil
	}
}

func (r *ReconcilerTest) ReconcilerTest(t *testing.T, testCase *ReconcilerTestCase) {
	//Reconcile again so Reconcile() checks for the OperatorSource
	res, err := r.Reconciler.Reconcile(testCase.Request)
	if testCase.ExpectedError != err {
		t.Errorf("%v reconcile result(%v) != expected(%v)", testCase.Request, err, testCase.ExpectedError)
	}

	if res != (testCase.ExpectedResult) {
		t.Errorf("%v reconcile result(%v) != expected(%v)", testCase.Request, res, testCase.ExpectedResult)
	}

	if testCase.TestObj != nil {
		err = r.Client.Get(context.TODO(), testCase.NamespacedName, testCase.TestObj)

		if err != nil {
			t.Errorf("get (%T): (%v)", testCase.TestObj, err)
		} else {
			testCase.ValidationFunc(r, t, testCase.TestObj)
		}
	}
}

func (r *ReconcilerTest) TestAll(t *testing.T, testCases []*ReconcilerTestCase) {

	if r.SetupFunc != nil {
		err := r.SetupFunc(r)

		if err != nil {
			t.Fatalf("failed to setup test %v", err)
		}
	}

	for i, testData := range testCases {
		testName := testData.TestName

		if testName == "" {
			testName = fmt.Sprintf("Step %v with name %s", i, testData.NamespacedName.Name)
		}
		t.Run(testName, func(t *testing.T) {
			r.ReconcilerTest(
				t,
				testData,
			)
		})
	}
}
