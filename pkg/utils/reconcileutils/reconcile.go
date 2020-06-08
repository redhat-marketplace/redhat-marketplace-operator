package reconcileutils

import (
	"context"
	"fmt"

	emperrors "emperror.dev/errors"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ResultFunc func(runtime.Object, ActionResult) bool

// ClientAction is the interface all actions must use in order to
// be able to be executed.
type ClientAction interface {
	Exec(context.Context, *ClientCommand) ActionResult
}

type If struct {
	If   func() bool
	Then ClientAction
	Else ClientAction
}

func (i *If) Exec(ctx context.Context, c *ClientCommand) ActionResult {
	if i.If != nil && i.If() {
		c.log.V(4).Info("executing if")
		return i.Then.Exec(ctx, c)
	}

	if i.Else != nil {
		c.log.V(4).Info("executing else")
		return i.Else.Exec(ctx, c)
	}

	return nil
}

type Result struct {
	Var    ActionResult
	Action ClientAction
}

func (r *Result) Exec(ctx context.Context, c *ClientCommand) ActionResult {
	r.Var = r.Action.Exec(ctx, c)
	return r.Var
}

type getAction struct {
	NamespacedName types.NamespacedName
	Object         runtime.Object
	getActionOptions
}

//go:generate go-options -option GetActionOption -prefix GetWith getActionOptions
type getActionOptions struct {
	IgnoreNotFound bool
}

func GetAction(
	namespacedName types.NamespacedName,
	object runtime.Object,
	options ...GetActionOption,
) *getAction {
	opts, _ := newGetActionOptions(options...)
	return &getAction{
		NamespacedName:   namespacedName,
		Object:           object,
		getActionOptions: opts,
	}
}

func (g *getAction) Exec(ctx context.Context, c *ClientCommand) ActionResult {
	err := c.client.Get(ctx, g.NamespacedName, g.Object)
	if err != nil {
		err = emperrors.Wrap(err, "error during get")
		if errors.IsNotFound(err) {
			if g.IgnoreNotFound {
				return NewExecResult(Continue, g.Object, reconcile.Result{}, err)
			}

			return NewExecResult(NotFound, g.Object, reconcile.Result{}, err)
		}
		if err != nil {
			return NewExecResult(Error, g.Object, reconcile.Result{}, err)
		}
	}

	return NewExecResult(Continue, g.Object, reconcile.Result{}, err)
}

type createAction struct {
	NewObject func() (runtime.Object, error)
	createActionOptions
	lastResult ExecResult
}

//go:generate go-options -option CreateActionOption -imports=k8s.io/apimachinery/pkg/runtime -prefix Create createActionOptions
type createActionOptions struct {
	IfLastResult func(ExecResult) bool
	WithPatch    bool
	WithAddOwner runtime.Object `options:",nil"`
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

func (a *createAction) Exec(ctx context.Context, c *ClientCommand) ActionResult {
	newObj, err := a.NewObject()
	reqLogger := c.log.WithValues("requestType", fmt.Sprintf("%T", newObj))

	if err != nil {
		reqLogger.Error(err, "failed to create")
		return NewExecResult(Error, newObj, reconcile.Result{}, err)
	}

	if a.WithPatch {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(newObj); err != nil {
			reqLogger.Error(err, "failure creating patch")
			return NewExecResult(Error, newObj, reconcile.Result{}, err)
		}
	}

	reqLogger.Info("Creating")
	err = c.client.Create(ctx, newObj)
	if err != nil {
		c.log.Error(err, "Failed to create.", "obj", newObj)
		return NewExecResult(Error, newObj, reconcile.Result{}, err)
	}

	if a.WithAddOwner != nil {
		if err := controllerutil.SetControllerReference(
			a.WithAddOwner.(metav1.Object),
			newObj.(metav1.Object),
			c.scheme); err != nil {
			c.log.Error(err, "Failed to create.", "obj", newObj)
			return NewExecResult(Error, newObj, reconcile.Result{}, err)
		}
	}

	return NewExecResult(Requeue, newObj, reconcile.Result{Requeue: true}, nil)
}

type updateAction struct {
	originalObject runtime.Object
	updateObject   func() (bool, runtime.Object, error)
	updateActionOptions
}

//go:generate go-options -option UpdateActionOption -prefix UpdateWith updateActionOptions
type updateActionOptions struct {
	Patch bool
}

func UpdateAction(
	originalObject runtime.Object,
	updateObjectFunc func() (bool, runtime.Object, error),
	updateOptions ...UpdateActionOption,
) *updateAction {
	opts, _ := newUpdateActionOptions(updateOptions...)

	return &updateAction{
		originalObject: originalObject,
		updateObject:   updateObjectFunc,
		updateActionOptions:  opts,
	}
}

func (a *updateAction) Exec(ctx context.Context, c *ClientCommand) ActionResult {
	ogObject := a.originalObject
	update, updatedObject, err := a.updateObject()

	if err != nil {
		c.log.Error(err, "failed to get object for update")
		return NewExecResult(Error, ogObject, reconcile.Result{}, err)
	}

	reqLogger := c.log.WithValues("requestType", fmt.Sprintf("%T", updatedObject))

	if a.Patch {
		patchResult, err := utils.RhmPatchMaker.Calculate(ogObject, updatedObject)

		if err != nil {
			reqLogger.Error(err, "Failed to compare patches")
			return NewExecResult(Error, ogObject, reconcile.Result{}, err)
		}

		if !patchResult.IsEmpty() {
			reqLogger.V(2).Info("patch result is not empty")
			update = true
		} else {
			reqLogger.V(2).Info("patch result is empty")
			update = false
		}
	}

	if update {
		err := c.client.Update(ctx, updatedObject)

		if err != nil {
			reqLogger.Error(err, "error updating object")
			return NewExecResult(Error, updatedObject, reconcile.Result{}, err)
		}

		reqLogger.V(2).Info("updated object")
		return NewExecResult(Requeue, updatedObject, reconcile.Result{}, nil)
	}

	return NewExecResult(Continue, ogObject, reconcile.Result{}, nil)
}

type ListAction struct {
	listActionOptions
}

//go:generate go-options -option ListActionOption -prefix ListWith listActionOptions
type listActionOptions struct {
	Obj    runtime.Object
	Filter []client.ListOption `options:"..."`
}

// type DeleteAction struct {
// 	deleteActionOptions
// }

// func DeleteAction(
// 	options ...DeleteActionOption,
// ) {
//}

//go:generate go-options -option DeleteActionOption -prefix DeleteWith deleteActionOptions
type deleteActionOptions struct {
	Obj runtime.Object
}

type statusConditionAction struct {
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
	ctx context.Context,
	actions ...ClientAction,
) (ActionResult, []ActionResult) {
	var result ActionResult
	results := make([]ActionResult, len(actions), 0)

	for _, action := range actions {
		result = action.Exec(ctx, c)
		results = append(results, result)

		if !result.Is(Continue) {
			return result, results
		}
	}

	return result, results
}
