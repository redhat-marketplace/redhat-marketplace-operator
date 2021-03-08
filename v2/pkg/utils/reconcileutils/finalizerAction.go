// Copyright 2020 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reconcileutils

import (
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
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
			HandleResult(
				Do(actions...),
				OnNotFound(UpdateAction(instance))),
		), nil
	}
}
