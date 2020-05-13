//go:generate go-options -imports=sigs.k8s.io/controller-runtime/pkg/reconcile,k8s.io/apimachinery/pkg/runtime -option ReconcileOption -prefix With reconcileUtilOptions

package reconcileutils

import (
	"context"
	"fmt"

	emperrors "emperror.dev/errors"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	"github.com/operator-framework/operator-sdk/pkg/status"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ReconcileUtilFunc func(runtime.Object) (ActionResult, reconcile.Result, error)
type CreateReconcileWithOptionsFunc func(runtime.Object, ...CreateActionOption) (ActionResult, reconcile.Result, error)
type UpdateReconcileWithOptionsFunc func(runtime.Object, ...UpdateActionOption) (ActionResult, reconcile.Result, error)
type PatchReconcileWithOptionsFunc func(runtime.Object) (ActionResult, reconcile.Result, error)
type ListReconcileWithOptionsFunc func(runtime.Object) (ActionResult, reconcile.Result, error)
type resultFunc func(runtime.Object, ActionResult, reconcile.Result, error) error
type resultFilterFunc func(runtime.Object, ActionResult, reconcile.Result, error) bool
type actionResultFilterFunc func(ActionResult) bool
type resultConditionFunc func(runtime.Object, ActionResult, reconcile.Result, error) status.Condition

type ActionResult *string

var (
	Continue ActionResult = ptr.String("continue")
	NotFound              = ptr.String("not_found")
	Requeue               = ptr.String("requeue")
	Error                 = ptr.String("error")
)

type ClientAction interface {
	Exec(context.Context, *ClientCommand, runtime.Object) (ActionResult, reconcile.Result, error)
}

type FilterAction struct {
	Action           ClientAction
	Options          []FilterActionOption
	prevResult       ActionResult
	prevActionResult reconcile.Result
	prevError        error
}

//go:generate go-options -option FilterActionOption -prefix FilterBy filterActionOptions
type filterActionOptions struct {
	ActionResult actionResultFilterFunc
	All          resultFilterFunc
}

func (g FilterAction) Exec(ctx context.Context, c *ClientCommand, obj runtime.Object) (ActionResult, reconcile.Result, error) {
	filterOpts, _ := newFilterActionOptions(g.Options...)

	if filterOpts.All != nil && filterOpts.All(obj, g.prevResult, g.prevActionResult, g.prevError) {
		return g.Action.Exec(ctx, c, obj)
	}

	if filterOpts.ActionResult != nil && filterOpts.ActionResult(g.prevResult) {
		return g.Action.Exec(ctx, c, obj)
	}

	return Continue, reconcile.Result{}, nil
}

type GetAction struct {
	NamespacedName types.NamespacedName
	Object         runtime.Object
	Options        []GetActionOption `options:"..."`
}

//go:generate go-options -option GetActionOption -prefix GetWith getActionOptions
type getActionOptions struct {
	IgnoreNotFound bool
}

func (g GetAction) Exec(ctx context.Context, c *ClientCommand, obj runtime.Object) (ActionResult, reconcile.Result, error) {
	getOpts, _ := newGetActionOptions(g.Options...)

	err := c.client.Get(ctx, g.NamespacedName, obj)
	if err != nil {
		err = emperrors.Wrap(err, "error during get")
		if errors.IsNotFound(err) {
			if getOpts.IgnoreNotFound {
				return Continue, reconcile.Result{}, err
			}

			return NotFound, reconcile.Result{}, err
		}
		if err != nil {
			return Error, reconcile.Result{}, err
		}
	}

	return Continue, reconcile.Result{}, err
}

type CreateAction struct {
	NewObject func() (runtime.Object, error)
	Options   []CreateActionOption
}

//go:generate go-options -option CreateActionOption -prefix CreateWith createActionOptions
type createActionOptions struct {
	Always    bool
	Patch     bool
	AddOwner  runtime.Object `options:",nil"`
	Condition resultConditionFunc
}

func (a CreateAction) Exec(ctx context.Context, c *ClientCommand, obj runtime.Object) (ActionResult, reconcile.Result, error) {
	reqLogger := c.log.WithValues("requestType", fmt.Sprintf("%T", obj))
	createOpts, _ := newCreateActionOptions(a.Options...)
	newObj, err := a.NewObject()

	if err != nil {
		reqLogger.Error(err, "failed to create")
		return Error, reconcile.Result{}, err
	}

	if createOpts.Patch {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(newObj); err != nil {
			reqLogger.Error(err, "failure creating patch")
			return Error, reconcile.Result{}, err
		}
	}

	reqLogger.Info("Creating")
	err = c.client.Create(ctx, obj)
	if err != nil {
		c.log.Error(err, "Failed to create.", "obj", newObj)
		return Error, reconcile.Result{}, err
	}

	if createOpts.AddOwner != nil {
		if err := controllerutil.SetControllerReference(createOpts.AddOwner(), obj, c.scheme); err != nil {
			return Error, reconcile.Result{}, err
		}
	}

	return Requeue, reconcile.Result{Requeue: true}, nil
}


//go:generate go-options -option UpdateActionOption -prefix UpdateWith updateActionOptions
type updateActionOptions struct {
	Patch     bool
	Condition resultConditionFunc
}

type ClientCommand struct {
	client client.Client
	scheme *runtime.Scheme
	log    logr.Logger
}

func NewClientCommand(
	client client.Client,
	scheme *runtime.Scheme,
	log logr.Logger,
) *ClientCommand {
	return &ClientCommand{
		client: client,
		scheme: scheme,
		log:    log,
	}
}

func (c *ClientCommand) Execute(
	actions []ClientAction,
	conditionFunc resultConditionFunc,
) (ActionResult, reconcile.Result, error) {
	var obj runtime.Object
	var result ActionResult
	var reconcileResult reconcile.Result
	var err error

	for _, action := range actions {
		if filterType, ok := action.(FilterAction); ok {
			filterType.prevResult = result
			filterType.prevActionResult = reconcileResult
			filterType.prevError = err
		}

		result, reconcileResult, err = action.Exec(obj)
		conditionFunc(obj, result, reconcileResult, err)

		if result != Continue {
			return result, reconcileResult, err
		}
	}

	return result, reconcileResult, err
}
