package server

import (
	"runtime"
	"time"
)

type finalizer struct {
	ch  chan time.Time
	ref *finalizerRef
}

type finalizerRef struct {
	parent *finalizer
}

func finalizerHandler(f *finalizerRef) {
	select {
	case f.parent.ch <- time.Time{}:
	default:
	}

	runtime.SetFinalizer(f, finalizerHandler)
}

func NewTicker() *finalizer {
	f := &finalizer{
		ch: make(chan time.Time, 1),
	}

	f.ref = &finalizerRef{parent: f}
	runtime.SetFinalizer(f.ref, finalizerHandler)
	f.ref = nil
	return f
}

// func SetGOGC() {
// 	var stats runtime.MemStats
// 	runtime.ReadMemStats(&stats)

// }
