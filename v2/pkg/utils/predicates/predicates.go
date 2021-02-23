package predicates

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func NamespacePredicate(targetNamespace string) predicate.Funcs {
	return predicate.Funcs{
		// Ensures MarketPlaceConfig reconciles only within namespace
		GenericFunc: func(e event.GenericEvent) bool {
			return e.Meta.GetNamespace() == targetNamespace
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.MetaOld.GetNamespace() == targetNamespace && e.MetaNew.GetNamespace() == targetNamespace
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Meta.GetNamespace() == targetNamespace
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Meta.GetNamespace() == targetNamespace
		},
	}
}
