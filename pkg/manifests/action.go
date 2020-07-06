package manifests

import (
	"context"

	emperrors "emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/patch"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type createOrUpdateFactoryItemAction struct {
	*BaseAction
	object         runtime.Object
	factoryFunc    func() (runtime.Object, error)
	owner          runtime.Object
	patchAnnotator patch.PatchAnnotator
	patchChecker   *PatchChecker
}

type CreateOrUpdateFactoryItemArgs struct {
	Owner          runtime.Object
	PatchAnnotator patch.PatchAnnotator
	PatchChecker   *PatchChecker
}

func CreateOrUpdateFactoryItemAction(
	newObj runtime.Object,
	factoryFunc func() (runtime.Object, error),
	args CreateOrUpdateFactoryItemArgs,
) *createOrUpdateFactoryItemAction {
	return &createOrUpdateFactoryItemAction{
		BaseAction:     NewBaseAction("createOrUpdateFactoryItem"),
		object:         newObj,
		factoryFunc:    factoryFunc,
		owner:          args.Owner,
		patchAnnotator: args.PatchAnnotator,
		patchChecker:   args.PatchChecker,
	}
}

func (a *createOrUpdateFactoryItemAction) Bind(result *ExecResult) {
	a.SetLastResult(result)
}

func (a *createOrUpdateFactoryItemAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	reqLogger := a.GetReqLogger(c)
	result, err := a.factoryFunc()

	if err != nil {
		reqLogger.Error(err, "failure creating factory obj")
		return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error with patch")
	}

	key, err := client.ObjectKeyFromObject(a.object)

	if err != nil {
		reqLogger.Error(err, "failure getting factory obj name")
		return NewExecResult(Error, reconcile.Result{}, err), emperrors.Wrap(err, "error with patch")
	}

	cmd := HandleResult(
		GetAction(key, a.object),
		OnNotFound(CreateAction(result, CreateWithAddOwner(a.owner), CreateWithPatch(a.patchAnnotator))),
		OnContinue(Call(func() (ClientAction, error) {
			update, err := a.patchChecker.CheckPatch(a.object, result)
			if err != nil {
				return nil, err
			}

			if !update {
				return nil, nil
			}

			return UpdateAction(result, UpdateWithPatch(a.patchAnnotator)), nil
		})))
	cmd.Bind(a.GetLastResult())
	return c.Do(ctx, cmd)
}
