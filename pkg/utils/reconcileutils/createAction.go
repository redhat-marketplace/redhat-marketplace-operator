package reconcileutils

import (
	"context"
	"fmt"

	emperrors "emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/codelocation"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type createAction struct {
	BaseAction
	newObject runtime.Object
	createActionOptions
}

//go:generate go-options -option CreateActionOption -imports=k8s.io/apimachinery/pkg/runtime,github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/patch -prefix Create createActionOptions
type createActionOptions struct {
	WithPatch           patch.PatchAnnotator
	WithAddOwner        runtime.Object `options:",nil"`
}

func CreateAction(
	newObj runtime.Object,
	opts ...CreateActionOption,
) *createAction {
	createOpts, _ := newCreateActionOptions(opts...)
	return &createAction{
		newObject:           newObj,
		createActionOptions: createOpts,
		BaseAction: BaseAction{
			codelocation: codelocation.New(1),
		},
	}
}

func (a *createAction) Bind(result *ExecResult) {
	a.lastResult = result
}

func (a *createAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	reqLogger := c.log.WithValues("file", a.codelocation, "action", "CreateAction")

	if isNil(a.newObject) {
		err := emperrors.WithStack(ErrNilObject)
		reqLogger.Error(err, "newObject is nil")
		return NewExecResult(Error, reconcile.Result{}, err), err
	}

	key, _ := client.ObjectKeyFromObject(a.newObject)
	reqLogger = reqLogger.WithValues("requestType", fmt.Sprintf("%T", a.newObject), "key", key)

	if a.WithPatch != nil {
		if err := a.WithPatch.SetLastAppliedAnnotation(a.newObject); err != nil {
			reqLogger.Error(err, "failure creating patch")
			return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error with patch")
		}
	}

	reqLogger.V(0).Info("Creating object", "object", a.newObject)
	err := c.client.Create(ctx, a.newObject)
	if err != nil {
		c.log.Error(err, "Failed to create.", "obj", a.newObject)
		return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error with create")
	}

	if a.WithAddOwner != nil {
		if meta, ok := a.newObject.(metav1.Object); ok {
			if err := controllerutil.SetControllerReference(
				a.WithAddOwner.(metav1.Object),
				meta,
				c.scheme); err != nil {
				c.log.Error(err, "Failed to create.", "obj", a.newObject)
				return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error adding owner")
			}
		}
	}

	return NewExecResult(Requeue, reconcile.Result{Requeue: true}, nil), nil
}
