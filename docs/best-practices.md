# Best Practices


## Errors

All errors returned should be wrapped using the emperror/errors library. This creates very detailed trace logs of the failure like:

```
 Messages:        "" is invalid: metadata.name: Required value: name is required
                        failed to create new kube state serivce monitor
                        github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller/meterbase.(*ReconcileMeterBase).Reconcile
                                /Users/ztaylor/ibm/repos/redhat-marketplace/redhat-marketplace-operator/pkg/controller/meterbase/meterbase_controller.go:319
                        github.com/redhat-marketplace/redhat-marketplace-operator/test/rectest.(*ControllerReconcileStep).Test
                                /Users/ztaylor/ibm/repos/redhat-marketplace/redhat-marketplace-operator/test/rectest/testing.go:169
                        github.com/redhat-marketplace/redhat-marketplace-operator/test/rectest.(*ReconcilerTest).TestAll
                                /Users/ztaylor/ibm/repos/redhat-marketplace/redhat-marketplace-operator/test/rectest/testing.go:316
                        github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller/meterbase.glob..func1.3.1
                                /Users/ztaylor/ibm/repos/redhat-marketplace/redhat-marketplace-operator/pkg/controller/meterbase/meterbase_controller_test.go:130
                        github.com/onsi/ginkgo/internal/leafnodes.(*runner).runSync
                                /Users/ztaylor/ibm/go/pkg/mod/github.com/onsi/ginkgo@v1.13.0/internal/leafnodes/runner.go:113

```

Here's an example return:

```go
//in your imports
import (
	merrors "emperror.dev/errors"
)

// in your code
  err := someComputation()

  if err != nil {
			return merrors.Wrap(err, "failed to create new kube state service monitor")
  }
```
