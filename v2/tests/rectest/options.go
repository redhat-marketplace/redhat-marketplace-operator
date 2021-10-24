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

package rectest

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

//go:generate go-options -imports=sigs.k8s.io/controller-runtime/pkg/reconcile -option StepOption -prefix With stepOptions
type stepOptions struct {
	StepName string
	Request  reconcile.Request
}

//go:generate go-options -option ReconcileStepOption -prefix ReconcileWith reconcileStepOptions
type reconcileStepOptions struct {
	ExpectedResults []ReconcileResult `options:"..."`
	UntilDone       bool
	Max             int
	IgnoreError     bool
}

//go:generate go-options -imports=sigs.k8s.io/controller-runtime/pkg/client -option GetStepOption -prefix GetWith getStepOptions
type getStepOptions struct {
	NamespacedName struct {
		Name, Namespace string
	}
	Obj         client.Object
	Labels      map[string]string            `options:",map[string]string{}"`
	CheckResult ReconcilerTestValidationFunc `options:",Ignore"`
}

//go:generate go-options -imports=sigs.k8s.io/controller-runtime/pkg/client,sigs.k8s.io/controller-runtime/pkg/client -option ListStepOption -prefix ListWith listStepOptions
type listStepOptions struct {
	Obj         client.ObjectList
	Filter      []client.ListOption              `options:"..."`
	CheckResult ReconcilerTestListValidationFunc `options:",IgnoreList"`
}
