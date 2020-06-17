package reconcileutils

import (
	"context"
	"fmt"

	emperrors "emperror.dev/errors"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type createAction struct {
	baseAction
	NewObject func() (runtime.Object, error)
	createActionOptions
}

//go:generate go-options -option CreateActionOption -imports=k8s.io/apimachinery/pkg/runtime,github.com/banzaicloud/k8s-objectmatcher/patch -prefix Create createActionOptions
type createActionOptions struct {
	WithPatch           *patch.Annotator
	WithAddOwner        runtime.Object `options:",nil"`
	WithStatusCondition UpdateStatusConditionFunc
}

func CreateAction(
	newObj func() (runtime.Object, error),
	opts ...CreateActionOption,
) *createAction {
	createOpts, _ := newCreateActionOptions(opts...)
	return &createAction{
		NewObject:           newObj,
		createActionOptions: createOpts,
	}
}

func (a *createAction) Bind(result *ExecResult) {
	a.lastResult = result
}

func (a *createAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	newObj, err := a.NewObject()
	reqLogger := c.log.WithValues("requestType", fmt.Sprintf("%T", newObj))

	withCondition := func(result *ExecResult, err error) (*ExecResult, error) {
		return result, err
	}

	if a.WithStatusCondition != nil {
		withCondition = func(result *ExecResult, err error) (*ExecResult, error) {
			statusUpdater := UpdateStatusCondition(a.WithStatusCondition)
			statusUpdater.Bind(result)
			statusUpdater.Exec(ctx, c)
			return result, err
		}
	}

	if err != nil {
		reqLogger.Error(err, "failed to create")
		return withCondition(NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error with generate new object"))
	}

	if newObj == nil {
		err := emperrors.New("object to create is nil")
		return withCondition(NewExecResult(Error, reconcile.Result{}, err), err)
	}

	if a.WithPatch != nil {
		if err := a.WithPatch.SetLastAppliedAnnotation(newObj); err != nil {
			reqLogger.Error(err, "failure creating patch")
			return withCondition(NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error with patch"))
		}
	}

	reqLogger.V(0).Info("Creating object")
	err = c.client.Create(ctx, newObj)
	if err != nil {
		c.log.Error(err, "Failed to create.", "obj", newObj)
		return withCondition(NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error with create"))
	}

	if a.WithAddOwner != nil {
		if err := controllerutil.SetControllerReference(
			a.WithAddOwner.(metav1.Object),
			newObj.(metav1.Object),
			c.scheme); err != nil {
			c.log.Error(err, "Failed to create.", "obj", newObj)
			return withCondition(NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error adding owner"))
		}
	}

	return withCondition(NewExecResult(Requeue, reconcile.Result{Requeue: true}, nil), nil)
}
