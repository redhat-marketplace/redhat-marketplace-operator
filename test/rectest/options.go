package rectest

import (
	"k8s.io/apimachinery/pkg/runtime"
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

//go:generate go-options -imports=k8s.io/apimachinery/pkg/runtime -option GetStepOption -prefix GetWith getStepOptions
type getStepOptions struct {
	NamespacedName struct {
		Name, Namespace string
	}
	Obj         runtime.Object
	Labels      map[string]string            `options:",map[string]string{}"`
	CheckResult ReconcilerTestValidationFunc `options:",Ignore"`
}

//go:generate go-options -imports=k8s.io/apimachinery/pkg/runtime,sigs.k8s.io/controller-runtime/pkg/client -option ListStepOption -prefix ListWith listStepOptions
type listStepOptions struct {
	Obj         runtime.Object
	Filter      []client.ListOption          `options:"..."`
	CheckResult ReconcilerTestValidationFunc `options:",Ignore"`
}
