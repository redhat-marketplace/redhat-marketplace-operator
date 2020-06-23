package reconcileutils

import (
	"context"
	"fmt"

	emperrors "emperror.dev/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type createAction struct {
	baseAction
	NewObject runtime.Object
	createActionOptions
}

//go:generate go-options -option CreateActionOption -imports=k8s.io/apimachinery/pkg/runtime -prefix Create createActionOptions
type createActionOptions struct {
	WithPatch           PatchAnnotator
	WithAddOwner        runtime.Object `options:",nil"`
}

func CreateAction(
	newObj runtime.Object,
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

	if isNil(a.NewObject) {
		err := emperrors.WithStack(ErrNilObject)
		return NewExecResult(Error, reconcile.Result{}, err), err
	}

	reqLogger := c.log.WithValues("requestType", fmt.Sprintf("%T", a.NewObject))

	if a.WithPatch != nil {
		if err := a.WithPatch.SetLastAppliedAnnotation(a.NewObject); err != nil {
			reqLogger.Error(err, "failure creating patch")
			return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error with patch")
		}
	}

	reqLogger.V(0).Info("Creating object")
	err := c.client.Create(ctx, a.NewObject)
	if err != nil {
		c.log.Error(err, "Failed to create.", "obj", a.NewObject)
		return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error with create")
	}

	if a.WithAddOwner != nil {
		if meta, ok := a.NewObject.(metav1.Object); ok {
			if err := controllerutil.SetControllerReference(
				a.WithAddOwner.(metav1.Object),
				meta,
				c.scheme); err != nil {
				c.log.Error(err, "Failed to create.", "obj", a.NewObject)
				return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error adding owner")
			}
		}
	}

	return NewExecResult(Requeue, reconcile.Result{Requeue: true}, nil), nil
}
