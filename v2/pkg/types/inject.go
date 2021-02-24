package types

import (
	ctrl "sigs.k8s.io/controller-runtime"
)

type SetupWithManager interface {
	SetupWithManager(mgr ctrl.Manager) error
}

type Injectable interface {
	SetCustomFields(i interface{}) error
}
