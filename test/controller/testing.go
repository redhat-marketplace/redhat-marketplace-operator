//go:generate go-options -imports=sigs.k8s.io/controller-runtime/pkg/reconcile,k8s.io/apimachinery/pkg/runtime -option TestCaseOption -prefix With testOptions

package testing

import (
	"context"
	"fmt"
	"sync"
	"testing"

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

type testOptions struct {
	StepName       string
	Request        reconcile.Request
	ExpectedResult reconcile.Result `options:",reconcile.Result{Requeue: true}"`
	ExpectedError  error
	Name           string
	Namespace      string
	TestObj        runtime.Object
	AfterFunc      ReconcilerTestValidationFunc `options:"After,Ignore"`
	Labels         map[string]string            `options:",map[string]string{}"`
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
	StepName       string
	Request        reconcile.Request
	ExpectedResult reconcile.Result
	ExpectedError  error
}

func NewReconcileStep(options ...TestCaseOption) *ReconcileStep {
	cfg, _ := newTestOptions(options...)
	return &ReconcileStep{
		StepName:       cfg.StepName,
		ExpectedResult: cfg.ExpectedResult,
		Request:        cfg.Request,
		ExpectedError:  cfg.ExpectedError,
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
	res, err := r.Reconciler.Reconcile(tc.Request)
	if tc.ExpectedError != err {
		t.Errorf("%v reconcile result(%v) != expected(%v)", tc.Request, err, tc.ExpectedError)
	}
	if res != (tc.ExpectedResult) {
		t.Errorf("%v reconcile result(%v) != expected(%v)", tc.Request, res, tc.ExpectedResult)
	}
}

type ClientGetStep struct {
	StepName       string
	NamespacedName types.NamespacedName `options:"NamespacedName,types.NamespacedName{}"`
	TestObj        runtime.Object
	Labels         map[string]string            `options:",map[string]string{}"`
	AfterFunc      ReconcilerTestValidationFunc `options:"After,Ignore"`
}

func NewClientGetStep(options ...TestCaseOption) *ClientGetStep {
	cfg, _ := newTestOptions(options...)
	return &ClientGetStep{
		StepName:       cfg.StepName,
		AfterFunc:      cfg.AfterFunc,
		TestObj:        cfg.TestObj,
		NamespacedName: types.NamespacedName{Name: cfg.Name, Namespace: cfg.Namespace},
		Labels:         cfg.Labels,
	}
}

func (tc *ClientGetStep) GetStepName() string {
	if tc.StepName == "" {
		if tc.NamespacedName.Namespace == "" {
			return tc.NamespacedName.Name
		}
		return tc.NamespacedName.Namespace + "/" + tc.NamespacedName.Name
	}
	return tc.StepName
}

func (tc *ClientGetStep) Test(t *testing.T, r *ReconcilerTest) {
	//Reconcile again so Reconcile() checks for the OperatorSource
	var err error
	err = r.GetClient().Get(context.TODO(), tc.NamespacedName, tc.TestObj)

	if err != nil {
		t.Errorf("get (%T): (%v)", tc.TestObj, err)
	} else {
		tc.AfterFunc(r, t, tc.TestObj)
	}
}

type ClientListStep struct {
	StepName       string
	NamespacedName types.NamespacedName `options:"NamespacedName,types.NamespacedName{}"`
	TestObj        runtime.Object
	Labels         map[string]string            `options:",map[string]string{}"`
	AfterFunc      ReconcilerTestValidationFunc `options:"After,Ignore"`
}

func NewClientListStep(options ...TestCaseOption) *ClientListStep {
	cfg, _ := newTestOptions(options...)
	return &ClientListStep{
		StepName:       cfg.StepName,
		AfterFunc:      cfg.AfterFunc,
		TestObj:        cfg.TestObj,
		NamespacedName: types.NamespacedName{Name: cfg.Name, Namespace: cfg.Namespace},
		Labels:         cfg.Labels,
	}
}

func (tc *ClientListStep) GetStepName() string {
	if tc.StepName == "" {
		if tc.NamespacedName.Namespace == "" {
			return tc.NamespacedName.Name
		}
		return tc.NamespacedName.Namespace + "/" + tc.NamespacedName.Name
	}
	return tc.StepName
}

func (tc *ClientListStep) ListStepName() string {
	if tc.StepName == "" {
		if tc.NamespacedName.Namespace == "" {
			return tc.NamespacedName.Name
		}
		return tc.NamespacedName.Namespace + "/" + tc.NamespacedName.Name
	}
	return tc.StepName
}

func (tc *ClientListStep) Test(t *testing.T, r *ReconcilerTest) {
	//Reconcile again so Reconcile() checks for the OperatorSource
	var err error
	err = r.GetClient().List(context.TODO(),
		tc.TestObj,
		client.InNamespace(tc.NamespacedName.Namespace),
		client.MatchingLabels(tc.Labels),
	)

	if err != nil {
		t.Errorf("get (%T): (%v)", tc.TestObj, err)
	} else {
		tc.AfterFunc(r, t, tc.TestObj)
	}
}

type ReconcilerTestCase struct {
	StepName       string
	Request        reconcile.Request
	ExpectedResult reconcile.Result
	ExpectedError  error
	NamespacedName types.NamespacedName `options:"NamespacedName,types.NamespacedName{}"`
	TestObj        runtime.Object
	AfterFunc      ReconcilerTestValidationFunc `options:"After,Ignore"`
	Labels         map[string]string            `options:",map[string]string{}"`
}

func NewReconcilerTestCase(options ...TestCaseOption) *ReconcilerTestCase {
	cfg, _ := newTestOptions(options...)
	return &ReconcilerTestCase{
		StepName:       cfg.StepName,
		AfterFunc:      cfg.AfterFunc,
		TestObj:        cfg.TestObj,
		NamespacedName: types.NamespacedName{Name: cfg.Name, Namespace: cfg.Namespace},
		ExpectedResult: cfg.ExpectedResult,
		Request:        cfg.Request,
		ExpectedError:  cfg.ExpectedError,
		Labels:         cfg.Labels,
	}
}

type ReconcilerTestCaseBuilder struct {
	tc ReconcilerTestCase
}

func (tc *ReconcilerTestCase) GetStepName() string {
	if tc.StepName == "" {
		if tc.NamespacedName.Namespace == "" {
			return tc.NamespacedName.Name
		}
		return tc.NamespacedName.Namespace + "/" + tc.NamespacedName.Name
	}
	return tc.StepName
}

func (tc *ReconcilerTestCase) Test(t *testing.T, r *ReconcilerTest) {
	//Reconcile again so Reconcile() checks for the OperatorSource
	res, err := r.Reconciler.Reconcile(tc.Request)
	if tc.ExpectedError != err {
		t.Errorf("%v reconcile result(%v) != expected(%v)", tc.Request, err, tc.ExpectedError)
	}

	if res != (tc.ExpectedResult) {
		t.Errorf("%v reconcile result(%v) != expected(%v)", tc.Request, res, tc.ExpectedResult)
	}

	if tc.TestObj != nil {
		if len(tc.Labels) > 0 {
			err = r.GetClient().List(context.TODO(),
				tc.TestObj,
				client.InNamespace(tc.NamespacedName.Namespace),
				client.MatchingLabels(tc.Labels),
			)
		} else {
			err = r.GetClient().Get(context.TODO(), tc.NamespacedName, tc.TestObj)
		}

		if err != nil {
			t.Errorf("get (%T): (%v)", tc.TestObj, err)
		} else {
			tc.AfterFunc(r, t, tc.TestObj)
		}
	}
}

var testAllMutex sync.Mutex

func (r *ReconcilerTest) TestAll(t *testing.T, testCases []TestCaseStep) {
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

		t.Run(testName, func(t *testing.T) {
			testData.Test(t, r)
		})
	}
}
