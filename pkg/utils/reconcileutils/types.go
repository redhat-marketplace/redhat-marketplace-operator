package reconcileutils

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/codelocation"
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
	// at the lastResult.
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

// baseAction is the struct that has common variables for all actions
type baseAction struct {
	lastResult   *ExecResult
	codelocation codelocation.CodeLocation
}

type ClientActionBranch struct {
	Status ActionResultStatus
	Action ClientAction
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
