package reconcileutils

import (
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

func SetFinalizer(instance runtime.Object, finalizer string) func() (ClientAction, error) {
	return func() (ClientAction, error) {
		accessor, err := meta.Accessor(instance)
		if err != nil {
			return nil, err
		}

		isMarkedForDeletion := accessor.GetDeletionTimestamp() != nil
		if utils.Contains(accessor.GetFinalizers(), finalizer) ||
			isMarkedForDeletion {
			return nil, nil
		}

		accessor.SetFinalizers(append(accessor.GetFinalizers(), finalizer))
		return UpdateAction(instance), nil
	}
}

func RunFinalizer(
	instance runtime.Object,
	finalizer string,
	actions ...ClientAction,
) func() (ClientAction, error) {
	return func() (ClientAction, error) {
		accessor, err := meta.Accessor(instance)
		if err != nil {
			return nil, err
		}

		isMarkedForDeletion := accessor.GetDeletionTimestamp() != nil

		if !isMarkedForDeletion {
			return nil, nil
		}

		if !utils.Contains(accessor.GetFinalizers(), finalizer) {
			//return final result here
			return ReturnFinishedResult(), nil
		}

		accessor.SetFinalizers(utils.RemoveKey(accessor.GetFinalizers(), finalizer))

		return Do(
			Do(actions...),
			UpdateAction(instance),
		), nil
	}
}
