// Copyright 2021 IBM Corp.
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

package razeeengine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/InVisionApp/go-health/v2"
	"github.com/go-logr/logr"
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/razee"
	pkgtypes "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Engine struct {
	store      *razee.RazeeStore
	namespaces pkgtypes.Namespaces
	kubeClient clientset.Interface
	runnables  Runnables
	log        logr.Logger
	health     *health.Health

	mainContext  *context.Context
	localContext *context.Context
	cancelFunc   context.CancelFunc
}

func ProvideEngine(
	store *razee.RazeeStore,
	namespaces pkgtypes.Namespaces,
	log logr.Logger,
	kubeClient clientset.Interface,
	runnables Runnables,
) *Engine {
	h := health.New()
	return &Engine{
		store:      store,
		log:        log,
		namespaces: namespaces,
		kubeClient: kubeClient,
		runnables:  runnables,
		health:     h,
	}
}

func (e *Engine) Start(ctx context.Context) error {
	if e.cancelFunc != nil {
		e.mainContext = nil
		e.localContext = nil
		e.cancelFunc()
	}

	localCtx, cancel := context.WithCancel(ctx)
	e.mainContext = &ctx
	e.localContext = &localCtx
	e.cancelFunc = cancel

	for i := range e.runnables {
		runnable := e.runnables[i]
		e.log.Info("starting runnable", "runnable", fmt.Sprintf("%T", runnable))
		wg := sync.WaitGroup{}

		wg.Add(1)

		go func() {
			wg.Done()
			runnable.Start(localCtx)
		}()

		wg.Wait()
	}

	return e.health.Start()
}

type reflectorConfig struct {
	expectedType runtime.Object
	lister       func(string) cache.ListerWatcher

	startContext context.Context

	cancelFunc context.CancelFunc
}

type ListerRunnable struct {
	reflectorConfig
	namespaces pkgtypes.Namespaces

	Store cache.Store
}

func (p *ListerRunnable) Start(ctx context.Context) error {
	if p.Store == nil {
		return errors.New("cache is nil")
	}

	if p.startContext == nil {
		p.startContext = ctx
	}

	var localCtx context.Context
	localCtx, p.cancelFunc = context.WithCancel(p.startContext)

	for i := range p.namespaces {
		localNS := p.namespaces[i]
		lister := p.lister(localNS)
		reflector := cache.NewReflector(lister, p.expectedType, p.Store, 0)
		go reflector.Run(localCtx.Done())
	}

	return nil
}

type StoreRunnable struct {
	Store      cache.Store
	Reflectors Runnables
	ResyncTime time.Duration

	startContext context.Context
	log          logr.Logger

	cancelFunc context.CancelFunc
}

func (p *StoreRunnable) Start(ctx context.Context) error {
	if p.startContext == nil {
		p.startContext = ctx
	}

	var localCtx context.Context
	localCtx, p.cancelFunc = context.WithCancel(p.startContext)

	for i := range p.Reflectors {
		p.log.Info(fmt.Sprintf("starting reflector %T\n", p.Reflectors[i]))
		err := p.Reflectors[i].Start(localCtx)

		if err != nil {
			return err
		}
	}

	if p.ResyncTime != 0 {
		ticker := time.NewTicker(p.ResyncTime)

		go func() {
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					p.Store.Resync()
				case <-localCtx.Done():
					return
				}
			}
		}()
	}

	return nil
}

type RazeeStoreRunnable struct {
	StoreRunnable
}

func ProvideRazeeStoreRunnable(
	kubeClient clientset.Interface,
	nses pkgtypes.Namespaces,
	store *razee.RazeeStore,
	log logr.Logger,
) *RazeeStoreRunnable {
	return &RazeeStoreRunnable{
		StoreRunnable: StoreRunnable{
			Store:      store.DeltaStore(), //delta implements the Store
			ResyncTime: 0,                  //1*60*time.Second,
			log:        log.WithName("razee"),
			Reflectors: []Runnable{
				provideNodeLister(kubeClient, store.DeltaStore()),                  //delta implements the Store
				provideServiceListerRunnable(kubeClient, nses, store.DeltaStore()), //delta implements the Store
			},
		},
	}
}

type NodeListerRunnable struct {
	ListerRunnable
}

func provideNodeLister(
	kubeClient clientset.Interface,
	store cache.Store,
) *NodeListerRunnable {
	return &NodeListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &corev1.Node{},
				lister:       CreateNodeListWatch(kubeClient),
			},
			Store: store,
		},
	}
}

type ServiceListerRunnable struct {
	ListerRunnable
}

func provideServiceListerRunnable(
	kubeClient clientset.Interface,
	nses pkgtypes.Namespaces,
	store cache.Store,
) *ServiceListerRunnable {
	return &ServiceListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &corev1.Service{},
				lister:       CreateServiceListWatch(kubeClient),
			},
			namespaces: nses,
			Store:      store,
		},
	}
}

var EngineSet = wire.NewSet(
	ProvideEngine,
	razee.NewRazeeStore,
)
