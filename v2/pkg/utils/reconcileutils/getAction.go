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
	"fmt"

	emperrors "emperror.dev/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type getAction struct {
	*BaseAction
	NamespacedName types.NamespacedName
	Object         runtime.Object
	getActionOptions
}

//go:generate go-options -option GetActionOption -prefix GetWith getActionOptions
type getActionOptions struct {
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
		BaseAction: NewBaseAction("Get"),
	}
}

func (g *getAction) Bind(r *ExecResult) {
	g.LastResult = r
}

func (g *getAction) Exec(ctx context.Context, c *ClientCommand) (*ExecResult, error) {
	reqLogger := g.GetReqLogger(c)

	if isNil(g.Object) {
		err := emperrors.New("object to get is nil")
		reqLogger.Error(err, "updatedObject is nil")
		return NewExecResult(Error, reconcile.Result{Requeue: true}, g.BaseAction, err), err
	}

	reqLogger = reqLogger.WithValues("requestType", fmt.Sprintf("%T", g.Object), "key", g.NamespacedName)

	err := c.client.Get(ctx, g.NamespacedName, g.Object)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("not found")
			return NewExecResult(NotFound, reconcile.Result{}, g.BaseAction, err), nil
		}
		if err != nil {
			reqLogger.Error(err, "error getting")
			return NewExecResult(Error, reconcile.Result{Requeue: true}, g.BaseAction, err), emperrors.Wrap(err, "error during get")
		}
	}

	reqLogger.V(2).Info("found")
	return NewExecResult(Continue, reconcile.Result{}, g.BaseAction, err), nil
}
