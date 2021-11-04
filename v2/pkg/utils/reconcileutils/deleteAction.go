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
	"context"

	emperrors "emperror.dev/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type deleteAction struct {
	obj client.Object
	BaseAction
	deleteActionOptions
}

//go:generate go-options -option DeleteActionOption -imports=sigs.k8s.io/controller-runtime/pkg/client -prefix Delete deleteActionOptions
type deleteActionOptions struct {
	WithDeleteOptions []client.DeleteOption `options:"..."`
}

func DeleteAction(
	obj client.Object,
	opts ...DeleteActionOption,
) *deleteAction {
	deleteOpts, _ := newDeleteActionOptions(opts...)
	return &deleteAction{
		obj:                 obj,
		deleteActionOptions: deleteOpts,
	}
}

func (d *deleteAction) Bind(result *ExecResult) {
	d.LastResult = result
}

func (d *deleteAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	if isNil(d.obj) {
		err := emperrors.New("object to delete is nil")
		return NewExecResult(Error, reconcile.Result{}, &d.BaseAction, err), err
	}

	err := c.client.Delete(ctx, d.obj, d.WithDeleteOptions...)

	if err != nil {
		return NewExecResult(Error, reconcile.Result{}, &d.BaseAction, err), emperrors.Wrap(err, "error while deleting")
	}

	return NewExecResult(Continue, reconcile.Result{}, &d.BaseAction, nil), nil
}
