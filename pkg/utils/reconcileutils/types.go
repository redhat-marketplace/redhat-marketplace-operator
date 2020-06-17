package reconcileutils

import (
	"context"

	"github.com/operator-framework/operator-sdk/pkg/status"
	"k8s.io/apimachinery/pkg/runtime"
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

// baseAction is the struct that has common variables for all actions
type baseAction struct {
	lastResult *ExecResult
}

// ConditionFunc takes an execresult and returns a true/false if the condition matches.
type ConditionFunc = func(*ExecResult) bool

// UpdateStatusConditionFunc takes an ExecResult and err and returns true, plus a custom resource instance,
// it's conditions, and a new condition to add or update to it's status. This facilitates updates to
// the status's condition variable.
type UpdateStatusConditionFunc func(result *ExecResult, err error) (update bool, instance runtime.Object, conditions *status.Conditions, condition status.Condition)

// UpdateFunction returns an updatedObject
type UpdateFunction func() (updatedObject runtime.Object, err error)

// ConditionalUpdateFunction returns true and and updatedObject if there is an update, or false if
// there is a no change.
type ConditionalUpdateFunction func() (update bool, updatedObject runtime.Object, err error)
