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
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"time"

	"github.com/caarlos0/env"
	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/pkg/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	timeout  = time.Second * 180
	interval = time.Second * 1
)

var (
	HarnessFeatures []FeatureFlag = []FeatureFlag{
		&addPullSecret{},
		&mockOpenShift{},
		&deployHelm{},
		&createMarketplaceConfig{},
	}
)

type TestHarness struct {
	logger  logr.Logger
	testEnv *envtest.Environment
	kscheme *runtime.Scheme
	context context.Context

	client.Client
	reconcileutils.ClientCommandRunner

	Config TestHarnessOptions

	cfg      *rest.Config
	stop     chan struct{}
	toDelete []runtime.Object

	features []FeatureFlag
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
	features := []FeatureFlag{}

	rootDir, _ := GetRootDirectory()

	fmt.Println(rootDir)
	t := true
	testEnv := envtest.Environment{
		UseExistingCluster: &t,
		CRDDirectoryPaths:  []string{
			//filepath.Join(rootDir, "deploy", "crds"),
			//filepath.Join(rootDir, "test", "testdata"),
		},
	}

bigloop:
	for _, name := range options.EnabledFeatures {
		if name == "all" {
			for _, f := range HarnessFeatures {
				features = append(features, f)
			}
			break bigloop
		}

		for _, f := range HarnessFeatures {
			if f.Name() == name {
				features = append(features, f)
				continue bigloop
			}
		}

		return nil, fmt.Errorf("failed find feature for %s", name)
	}

	return &TestHarness{
		testEnv:  &testEnv,
		logger:   logger,
		features: features,
		Config:   options,
		context:  context.Background(),
		stop:     make(chan struct{}),
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

	err = t.Setup()

	if err != nil {
		t.Stop()

		return t.context, err
	}

	return t.context, nil
}

func (t *TestHarness) Setup() error {
	err := env.Parse(t)

	if err != nil {
		return errors.Wrap(err, "fail")
	}

	os.Setenv("WATCH_NAMESPACE", t.Config.WatchNamespace)
	for _, feature := range t.features {
		t.logger.Info("loading env overrides", "feature", feature.Name())
		err := feature.Parse()

		if err != nil {
			return errors.Wrap(err, "fail")
		}
	}

	err = t.Upsert(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: t.Config.Namespace,
		},
	})

	if err != nil {
		return err
	}

	sort.Sort(ByPriority(t.features))

	for _, feature := range t.features {
		if f, ok := feature.(SetupFunc); ok {
			t.logger.Info("setting up harness feature", "feature", feature.Name())
			err := f.Setup(t)

			if err != nil {
				t.logger.Error(err, "failed to setup")
				return errors.Wrap(err, "fail")
			}
		}
	}

	return nil
}

func (t *TestHarness) BeforeAll() error {
	sort.Sort(ByPriority(t.features))

	for _, feature := range t.features {
		if f, ok := feature.(BeforeFunc); ok {
			t.logger.Info("executing before", "feature", feature.Name())
			err := f.Before(t)

			if err != nil {
				return errors.Wrap(err, "fail")
			}
		}
	}

	return nil
}

func (t *TestHarness) AfterAll() error {
	sort.Sort(sort.Reverse(ByPriority(t.features)))

	for _, feature := range t.features {
		if f, ok := feature.(AfterFunc); ok {
			t.logger.Info("executing after", "feature", feature.Name())
			err := f.After(t)

			if err != nil {
				return errors.Wrap(err, "fail")
			}
		}
	}

	return nil
}

func (t *TestHarness) Stop() error {
	if t.Client != nil {
		sort.Sort(sort.Reverse(ByPriority(t.features)))

		for _, feature := range t.features {
			if f, ok := feature.(HasCleanup); ok {
				t.logger.Info("cleaning up for feature", "feature", feature.Name())
				arr := f.HasCleanup()

				for _, obj := range arr {
					t.Client.Delete(context.TODO(), obj)
				}
			}
			if f, ok := feature.(TeardownFunc); ok {
				t.logger.Info("tearing down for feature", "feature", feature.Name())
				err := f.Teardown(t)

				if err != nil {
					t.logger.Error(err, "failed to tear down")
				}
			}
		}

		close(t.stop)
	}

	if t.testEnv != nil {
		t.testEnv.Stop()
	}

	return nil
}

func (t *TestHarness) Upsert(ctx context.Context, obj runtime.Object) error {
	err := t.Create(ctx, obj)

	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			oldObj := obj.DeepCopyObject()
			key, _ := client.ObjectKeyFromObject(oldObj)
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

type FeatureFlag interface {
	Name() string
	Parse() error
}

type SetupFunc interface {
	Setup(h *TestHarness) error
}

type TeardownFunc interface {
	Teardown(h *TestHarness) error
}

type BeforeFunc interface {
	Before(h *TestHarness) error
}

type AfterFunc interface {
	After(h *TestHarness) error
}

type HasCleanup interface {
	HasCleanup() []runtime.Object
}

type ByPriority []FeatureFlag

func (a ByPriority) Len() int           { return len(a) }
func (a ByPriority) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByPriority) Less(i, j int) bool { return getPriority(a[i]) < getPriority(a[j]) }

func getPriority(f FeatureFlag) int {
	for i, f2 := range HarnessFeatures {
		if f.Name() == f2.Name() {
			return i
		}
	}

	return 0
}

type TestHarnessFeatures struct {
	Certs  bool
	Deploy bool
}

const FeatureAddPullSecret string = "AddPullSecret"

type addPullSecret struct {
	PullSecretName string `env:"PULL_SECRET_NAME" envDefault:"local-pull-secret"`
	DockerAuthFile string `env:"DOCKER_AUTH_FILE" envDefault:"${HOME}/.docker/config.json" envExpand:"true"`

	pullSecret *v1.Secret
}

func (e *addPullSecret) Name() string {
	return FeatureAddPullSecret
}

func (e *addPullSecret) Parse() error {
	return env.Parse(e)
}

func (e *addPullSecret) Setup(h *TestHarness) error {
	data, err := ioutil.ReadFile(e.DockerAuthFile)
	if err != nil {
		return errors.Wrap(err, "failed to read docker auth file")
	}

	os.Setenv("PULL_SECRET_NAME", e.PullSecretName)

	pullSecret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      e.PullSecretName,
			Namespace: h.Config.Namespace,
		},
		Type: v1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			v1.DockerConfigJsonKey: data,
		},
	}

	h.Upsert(context.TODO(), &pullSecret)

	e.pullSecret = &pullSecret

	return nil
}

func (e *addPullSecret) HasCleanup() []runtime.Object {
	return []runtime.Object{e.pullSecret}
}

const FeatureCreateMarketplaceConfig string = "CreateMarketplaceConfig"

type createMarketplaceConfig struct {
	Namespace string `env:"NAMESPACE" envDefault:"openshift-redhat-marketplace"`

	marketplaceCfg *v1alpha1.MarketplaceConfig
	base           *v1alpha1.MeterBase
}

func (d *createMarketplaceConfig) Name() string {
	return FeatureCreateMarketplaceConfig
}

func (d *createMarketplaceConfig) Parse() error {
	return env.Parse(d)
}

func (d *createMarketplaceConfig) Setup(t *TestHarness) error {
	d.base = &v1alpha1.MeterBase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rhm-marketplaceconfig-meterbase",
			Namespace: d.Namespace,
		},
		Spec: v1alpha1.MeterBaseSpec{
			Enabled: true,
			Prometheus: &v1alpha1.PrometheusSpec{
				Storage: v1alpha1.StorageSpec{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: "",
					},
					Size: resource.MustParse("1Gi"),
				},
			},
		},
	}
	d.marketplaceCfg = &v1alpha1.MarketplaceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.MARKETPLACECONFIG_NAME,
			Namespace: d.Namespace,
		},
		Spec: v1alpha1.MarketplaceConfigSpec{
			ClusterUUID:             "fooCluster",
			RhmAccountID:            "fooID",
			EnableMetering:          ptr.Bool(true),
			InstallIBMCatalogSource: ptr.Bool(false),
		},
	}

	return nil
}

func (d *createMarketplaceConfig) Before(t *TestHarness) error {
	if err := t.Setup(); err != nil {
		return err
	}

	err := t.Create(context.TODO(), d.marketplaceCfg)

	if err != nil {
		return err
	}

	By("wait for marketplaceconfig")
	Eventually(func() bool {
		err := t.Get(context.TODO(), k8stypes.NamespacedName{Name: d.base.Name, Namespace: d.Namespace}, d.base)

		if err != nil {
			return false
		}

		return true
	}, timeout).Should(BeTrue())

	By("update meterbase")

	By("wait for meterbase")
	Eventually(func() map[string]interface{} {
		err := t.Get(context.TODO(), k8stypes.NamespacedName{Name: d.base.Name, Namespace: d.Namespace}, d.base)

		if err != nil {
			return map[string]interface{}{
				"err": err,
			}
		}

		if d.base.Status.Conditions == nil {
			return map[string]interface{}{
				"err":             err,
				"conditionStatus": "",
			}
		}

		cond := d.base.Status.Conditions.GetCondition(v1alpha1.ConditionInstalling)

		return map[string]interface{}{
			"err":               err,
			"conditionStatus":   cond.Status,
			"reason":            cond.Reason,
			"availableReplicas": d.base.Status.AvailableReplicas,
		}
	}, timeout, interval).Should(
		MatchAllKeys(Keys{
			"err":               Succeed(),
			"conditionStatus":   Equal(corev1.ConditionFalse),
			"reason":            Equal(v1alpha1.ReasonMeterBaseFinishInstall),
			"availableReplicas": PointTo(BeNumerically("==", 2)),
		}),
	)

	return nil
}

func (d *createMarketplaceConfig) After(t *TestHarness) error {
	t.Delete(context.TODO(), d.marketplaceCfg)
	t.Delete(context.TODO(), d.base)
	Eventually(func() bool {
		err := t.Get(context.TODO(),
			k8stypes.NamespacedName{Name: d.base.Name, Namespace: d.Namespace}, d.base)

		if err == nil {
			return false
		}

		return k8serrors.IsNotFound(err)
	}, timeout, interval).Should(BeTrue())

	return nil
}
