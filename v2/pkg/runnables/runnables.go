package runnables

import (
	"github.com/google/wire"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Runnables []manager.Runnable

var RunnableSet = wire.NewSet(
	NewPodMonitor,
	ProvideRunnables,
)

func ProvideRunnables(
	podMonitor *PodMonitor,
) Runnables {
	return []manager.Runnable{
		podMonitor,
	}
}
