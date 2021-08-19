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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var NotListTypeErr = emperrors.New("type is not a list type")

type listAction struct {
	list    runtime.Object
	filters []client.ListOption
	*BaseAction
}

func ListAction(list runtime.Object, filters ...client.ListOption) *listAction {
	return &listAction{
		list:       list,
		filters:    filters,
		BaseAction: NewBaseAction("list"),
	}
}

func (l *listAction) Bind(result *ExecResult) {
	l.LastResult = result
}

func (l *listAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	err := c.client.List(ctx, l.list, l.filters...)

	if err != nil {
		return NewExecResult(Error, reconcile.Result{}, l.BaseAction, err), emperrors.Wrap(err, "error while listing")
	}

	return NewExecResult(Continue, reconcile.Result{}, l.BaseAction, nil), nil
}

type listAppendAction struct {
	list    runtime.Object
	filters []client.ListOption
	*BaseAction
}

func ListAppendAction(listType runtime.Object, filters ...client.ListOption) *listAppendAction {
	return &listAppendAction{
		list:       listType,
		filters:    filters,
		BaseAction: NewBaseAction("listAppend"),
	}
}

func (l *listAppendAction) Bind(result *ExecResult) {
	l.LastResult = result
}

func (l *listAppendAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	if !meta.IsListType(l.list) {
		return NewExecResult(Error, reconcile.Result{}, l.BaseAction, NotListTypeErr), emperrors.Wrap(NotListTypeErr, "invalid input type")
	}

	extractedList, err := meta.ExtractList(l.list)
	if err != nil {
		return NewExecResult(Error, reconcile.Result{}, l.BaseAction, err), emperrors.Wrap(err, "error extracting original list")
	}

	newList := l.list.DeepCopyObject()
	_ = meta.SetList(newList, []runtime.Object{})

	if err != nil {
		return NewExecResult(Error, reconcile.Result{}, l.BaseAction, err), emperrors.Wrap(err, "error while listing")
	}

	err = c.client.List(ctx, newList, l.filters...)

	if err != nil {
		return NewExecResult(Error, reconcile.Result{}, l.BaseAction, err), emperrors.Wrap(err, "error while listing")
	}

	if meta.LenList(newList) == 0 {
		return NewExecResult(Continue, reconcile.Result{}, l.BaseAction, nil), nil
	}

	newListSlice, err := meta.ExtractList(newList)

	if err != nil {
		return NewExecResult(Error, reconcile.Result{}, l.BaseAction, err), emperrors.Wrap(err, "error while extracting list")
	}

	for _, obj := range newListSlice {
		extractedList = append(extractedList, obj)
	}

	meta.SetList(l.list, extractedList)

	return NewExecResult(Continue, reconcile.Result{}, l.BaseAction, nil), nil
}
