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

package harness

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	timeout  = time.Second * 180
	interval = time.Second * 1
)

type TestHarness struct {
	logger  logr.Logger
	testEnv *envtest.Environment
	kscheme *runtime.Scheme
	context context.Context

	client.Client
	reconcileutils.ClientCommandRunner

	Config TestHarnessOptions

	cfg  *rest.Config
	stop chan struct{}
}

type TestHarnessOptions struct {
	EnabledFeatures []string `env:"FEATURES" envSeparator:"," envDefault:"all"`
	Namespace       string   `env:"NAMESPACE" envDefault:"openshift-redhat-marketplace"`
	WatchNamespace  string   `env:"WATCH_NAMESPACE" envDefault:""`

	ProvideScheme func(cfg *rest.Config) (*runtime.Scheme, error)
}

func NewTestHarness(
	options TestHarnessOptions,
) (*TestHarness, error) {
	logger := logf.Log.WithName("test-harness-logger")

	t := true
	testEnv := envtest.Environment{
		UseExistingCluster: &t,
	}

	return &TestHarness{
		testEnv: &testEnv,
		logger:  logger,
		Config:  options,
		context: context.Background(),
		stop:    make(chan struct{}),
	}, nil
}

func (t *TestHarness) Start() (context.Context, error) {
	var err error

	cfg, err := t.testEnv.Start()
	if err != nil {
		return t.context, errors.Wrap(err, "failed to start testenv")
	}

	t.kscheme, err = t.Config.ProvideScheme(cfg)
	if err != nil {
		return t.context, err
	}

	t.Client, err = client.New(cfg, client.Options{Scheme: t.kscheme})

	if err != nil {
		return t.context, err
	}

	t.ClientCommandRunner = reconcileutils.NewClientCommand(t.Client, t.kscheme, t.logger)
	return t.context, nil
}

func (t *TestHarness) Stop() error {

	if t.testEnv != nil {
		t.testEnv.Stop()
	}

	return nil
}

func (t *TestHarness) Upsert(ctx context.Context, obj client.Object) error {
	err := t.Create(ctx, obj)

	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			oldObj := obj.DeepCopyObject()
			key := client.ObjectKeyFromObject(&oldObj)
			err := t.Get(ctx, key, oldObj)

			if err != nil {
				return err
			}

			acc1, _ := meta.Accessor(obj)
			acc2, _ := meta.Accessor(oldObj)

			acc1.SetUID(acc2.GetUID())
			acc1.SetGeneration(acc2.GetGeneration())
			acc1.SetResourceVersion(acc2.GetResourceVersion())

			return t.Update(ctx, obj)
		}

		return err
	}

	return nil
}
