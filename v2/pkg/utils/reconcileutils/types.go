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

	"github.com/go-logr/logr"
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/codelocation"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClientAction is the interface all actions must use in order to
// be able to be executed.
type ClientAction interface {
	// Exec is the logic being the action, running this function will
	// execute the action.
	Exec(context.Context, *ClientCommand) (*ExecResult, error)
	// Bind binds a previous result to the action, this is to provide it to
	// an action so it can chain commands together. Not all actions have to look
	// at the LastResult.
	Bind(*ExecResult)
}

// ClientCommandRunner provides a method of executing commands. Commands
// can be executed on their own but this interface should be used instead
// so mocking and context is preserved.
type ClientCommandRunner interface {
	// Do runs the commands one after the other. Do chains one command
	// result to another so if a command errors or is returned then it will
	// stop executing and return.
	Do(ctx context.Context, actions ...ClientAction) (*ExecResult, error)
	// Exec will run a single command. You can use Do instead but Do uses the Do
	// Command, where exec will only just call the action it is passed.
	Exec(ctx context.Context, action ClientAction) (*ExecResult, error)
}

// BaseAction is the struct that has common variables for all actions
type BaseAction struct {
	Name         string                    `json:"name"`
	LastResult   *ExecResult               `json:"-"`
	CodeLocation codelocation.CodeLocation `json:"code_location"`
}

func NewBaseAction(name string) *BaseAction {
	return &BaseAction{
		Name:         name,
		CodeLocation: codelocation.New(2),
	}
}

func (b *BaseAction) Bind(result *ExecResult) {
	b.SetLastResult(result)
}

func (b *BaseAction) GetReqLogger(c *ClientCommand) logr.Logger {
	return c.Log().WithValues("file", b.CodeLocation, "action", b.Name)
}

func (b *BaseAction) SetLastResult(a *ExecResult) {
	b.LastResult = a
}

func (b *BaseAction) GetLastResult() *ExecResult {
	return b.LastResult
}

type ClientActionBranch struct {
	Status ActionResultStatus
	Action ClientAction
	Any    bool
}

type ClientCommandRunnerProvider interface {
	NewCommandRunner(client client.Client, scheme *runtime.Scheme, log logr.Logger) ClientCommandRunner
}

type DefaultCommandRunnerProvider struct{}

func (d *DefaultCommandRunnerProvider) NewCommandRunner(client client.Client, scheme *runtime.Scheme, log logr.Logger) ClientCommandRunner {
	return NewClientCommand(client, scheme, log)
}

func ProvideDefaultCommandRunnerProvider() *DefaultCommandRunnerProvider {
	return &DefaultCommandRunnerProvider{}
}

var CommandRunnerProviderSet = wire.NewSet(
	ProvideDefaultCommandRunnerProvider,
	NewClientCommand,
	wire.Bind(new(ClientCommandRunnerProvider), new(*DefaultCommandRunnerProvider)),
)
