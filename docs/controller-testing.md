# Controller testing

To ease testing, there is a mini-framework available in the test/rectest package.
This framework provides 3 primitives for testing controllers although more can easily be added.

The 4 primitives is list, get, and reconcile. Using the framework allows you to define tests that are easy to reason with.

The problem with writing tests for the reconciler is you lose track of where exactly you are and why you're their. The simple flow is:

1. look for something (configmap, pod, deployment)
1. if not found create that missing thing and requeue
1. if found, update it to match the desired state
1. update status

These 4 steps can be performed over and over. So knowing exactly where you are and why you're there can be hard sometimes. The higher up you can see the problem the easier its is.

## Features

- Simple primitives to make testing easier.
- Option flows to enable advanced testing but has sensible defaults.
- Errors that can point you to our test definitions.
- Quickly write parallel test to speed up our unit test suite.

## Example

If you have a controller that creates kube resources, maybe our reconcile loop looks like this example.

```go

// WARNING: pseudocode not meant to be real golang

cm := getConfigMap()
if cm == nil {
  createConfigmap()
  return reconcile.Result{Requeue: true}, nil
}

dep := getDeployment()
if depl == nil {
  createDeployment()
  return reconcile.Result{Requeue: true}, nil
}

return reconcile.Result{}, nil
```

Then we would construct a test like this:

```go
// Step Options holds our reconcile request.
opts = []StepOption{
  WithRequest(req),
}

reconcilerTest := NewReconcilerTest(setup, marketplaceconfig)
	reconcilerTest.TestAll(t,
    // We expect to have 2 good reconcile results and then we're done
		ReconcileStep(opts, ReconcileWithExpectedResults(
			append(RangeReconcileResults(RequeueResult, 2), DoneResult)...,
		)),
		GetStep(opts,
			GetWithNamespacedName("mycm", namespace),
			GetWithObj(&corev1.ConfigMap{}),
		),
		GetStep(opts,
			GetWithNamespacedName("mydep", namespace),
			GetWithObj(&appsv1.Deployment{}),
		),
	)
```

## Debugging tests

The test framework provides tools to help you find problems in your test when they occur.

When an error occurs in a test step, you'll see output like this:

```shell
--- FAIL: TestRazeeDeployController (0.00s)
    --- FAIL: TestRazeeDeployController/Test_Clean_Install (0.01s)
        --- FAIL: TestRazeeDeployController/Test_Clean_Install/ReconcileStep_4 (0.00s)
            --- FAIL: TestRazeeDeployController/Test_Clean_Install/ReconcileStep_4/1_of_1_expresult (0.00s)
                testing.go:149:
                    	Error Trace:	testing.go:149
                    	Error:      	Not equal:
                    	            	expected: rectest.ReconcileResult{Result:reconcile.Result{Requeue:true, RequeueAfter:0}, Err:error(nil)}
                    	            	actual  : rectest.ReconcileResult{Result:reconcile.Result{Requeue:false, RequeueAfter:0}, Err:error(nil)}

                    	            	Diff:
                    	            	--- Expected
                    	            	+++ Actual
                    	            	@@ -2,3 +2,3 @@
                    	            	  Result: (reconcile.Result) {
                    	            	-  Requeue: (bool) true,
                    	            	+  Requeue: (bool) false,
                    	            	   RequeueAfter: (time.Duration) 0s
                    	Test:       	TestRazeeDeployController/Test_Clean_Install/ReconcileStep_4/1_of_1_expresult
                    	Messages:   	incorrect expected result
                    	            	github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller/razeedeployment.testCleanInstall
                    	            		/Users/ztaylor/ibm/go/src/github.ibm.com/symposium/redhat-marketplace-operator/pkg/controller/razeedeployment/razeedeployment_controller_test.go:210
        razeedeployment_controller_test.go:156:
            	Error Trace:	testing.go:305
            	Error:      	Should be true
            	Test:       	TestRazeeDeployController/Test_Clean_Install
            	Messages:   	Step ReconcileStep 4 failed
```

### Finding the test name

```
FAIL: TestRazeeDeployController/Test_Clean_Install/ReconcileStep_4/1_of_1_expresult (0.00s)
```

The first fail line shows the place where the error occured. You can walk down the stack of names in your file. It would be the subtest `Test Clean Install`, the 4th test, reconcileResult step, and the first expected result.

### Finding the cause

```
Messages:   	incorrect expected result
github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller/razeedeployment.testCleanInstall
/Users/ztaylor/ibm/go/src/github.ibm.com/symposium/redhat-marketplace-operator/pkg/controller/razeedeployment/razeedeployment_controller_test.go:210
```

The messages output will have the line where the ReconcileStep is defined at for this failure.

### Still lost

Step names can be renamed with the `WithStepName` option under the first options block. You'd have to define each one individually if you wanted specific names to look for.

```
ReconcileStep(append(opts, WithStepName("yourspecificstepname")),
  ReconcileWithExpectedResults(
    append(RangeReconcileResults(RequeueResult, 2), DoneResult)...,
  )),
```

## Extending

New Steps can be defined locally just by extending the Step interface. You can use this to define a repeatable test that can easily be used throughout your code.
