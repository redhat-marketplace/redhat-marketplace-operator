package harness

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/caarlos0/env"
	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	"github.com/pkg/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	timeout  = time.Second * 60
	interval = time.Second * 1
)

var IsNotFound types.GomegaMatcher = WithTransform(k8serrors.IsNotFound, BeTrue())

var SucceedOrAlreadyExist types.GomegaMatcher = SatisfyAny(
	Succeed(),
	WithTransform(k8serrors.IsAlreadyExists, BeTrue()),
)

type TestHarness struct {
	logger  logr.Logger
	testEnv *envtest.Environment
	kscheme *runtime.Scheme

	client.Client

	config TestHarnessOptions

	cfg      *rest.Config
	stop     chan struct{}
	toDelete []runtime.Object

	features []FeatureFlag
}

type TestHarnessOptions struct {
	EnabledFeatures []string `env:"FEATURES" envSeparator:"," envDefault:"all"`
	Namespace       string   `env:"NAMESPACE" envDefault:"openshift-redhat-marketplace"`
	WatchNamespace  string   `env:"WATCH_NAMESPACE" envDefault:""`
}

func NewTestHarness(
	kscheme *runtime.Scheme,
	testEnv *envtest.Environment,
	options TestHarnessOptions,
) (*TestHarness, error) {
	logger := logf.Log.WithName("test-harness-logger")
	features := []FeatureFlag{}

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
		logger:   logger,
		kscheme:  kscheme,
		testEnv:  testEnv,
		features: features,
		config:   options,
	}, nil
}

func (t *TestHarness) Start() error {
	var err error
	t.stop = make(chan struct{})
	t.Client, err = client.New(t.testEnv.Config, client.Options{Scheme: t.kscheme})

	if err != nil {
		return errors.Wrap(err, "fail")
	}

	err = t.Setup()

	if err != nil {
		return errors.Wrap(err, "fail")
	}

	return nil
}

func (t *TestHarness) Setup() error {
	err := env.Parse(t)

	if err != nil {
		return errors.Wrap(err, "fail")
	}

	os.Setenv("WATCH_NAMESPACE", t.config.WatchNamespace)
	t.testEnv = &envtest.Environment{
		UseExistingCluster: ptr.Bool(true),
	}

	for _, feature := range t.features {
		t.logger.Info("loading env overrides", "feature", feature.Name())
		err := feature.Parse()

		if err != nil {
			return errors.Wrap(err, "fail")
		}
	}

	err = t.Client.Create(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: t.config.Namespace,
		},
	})

	Expect(err).To(SucceedOrAlreadyExist)

	sort.Sort(ByPriority(t.features))

	for _, feature := range t.features {
		if f, ok := feature.(SetupFunc); ok {
			t.logger.Info("setting up harness feature", "feature", feature.Name())
			err := f.Setup(t)

			if err != nil {
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
	sort.Sort(sort.Reverse(ByPriority(t.features)))

	for _, feature := range t.features {
		if f, ok := feature.(HasCleanup); ok {
			arr := f.HasCleanup()

			if len(arr) > 0 {
				for _, obj := range arr {
					t.Client.Delete(context.TODO(), obj)
				}
			}
		}
	}

	close(t.stop)

	return nil
}

type FeatureFlag interface {
	Name() string
	Parse() error
}

type SetupFunc interface {
	Setup(h *TestHarness) error
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

var (
	HarnessFeatures []FeatureFlag = []FeatureFlag{
		&addPullSecret{},
		&mockOpenShift{},
		&deployLocal{},
		&createMarketplaceConfig{},
	}
)

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

type addPullSecret struct {
	PullSecretName string `env:"PULL_SECRET_NAME" envDefault:"local-pull-secret"`
	DockerAuthFile string `env:"DOCKER_AUTH_FILE" envDefault:"${HOME}/.docker/config.json" envExpand:"true"`

	pullSecret *v1.Secret
}

func (e *addPullSecret) Name() string {
	return "AddPullSecret"
}

func (e *addPullSecret) Parse() error {
	return env.Parse(e)
}

func (e *addPullSecret) Setup(h *TestHarness) error {
	data, err := ioutil.ReadFile(e.DockerAuthFile)
	if err != nil {
		return errors.Wrap(err, "failed to read docker auth file")
	}

	pullSecret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      e.PullSecretName,
			Namespace: h.config.Namespace,
		},
		Type: v1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			v1.DockerConfigJsonKey: data,
		},
	}

	Expect(h.Client.Create(context.TODO(), &pullSecret)).To(SucceedOrAlreadyExist)

	e.pullSecret = &pullSecret

	return nil
}

func (e *addPullSecret) HasCleanup() []runtime.Object {
	return []runtime.Object{e.pullSecret}
}

type deployLocal struct {
	PullSecretName string `env:"PULL_SECRET_NAME" envDefault:"local-pull-secret"`
	Namespace      string `env:"NAMESPACE" envDefault:"openshift-redhat-marketplace"`

	cleanup []runtime.Object
}

func (d *deployLocal) Name() string {
	return "DeployLocal"
}

func (d *deployLocal) Parse() error {
	return env.Parse(d)
}

func (d *deployLocal) HasCleanup() []runtime.Object {
	return d.cleanup
}

func (d *deployLocal) Setup(h *TestHarness) error {
	d.cleanup = []runtime.Object{}
	command := exec.Command("make", "clean", "helm")
	command.Dir = "../.."
	command.Env = os.Environ()
	err := command.Run()
	out, _ := command.CombinedOutput()

	if err != nil {
		fmt.Println(string(out))
		return errors.Wrap(err, "fail")
	}

	operatorDepl := &appsv1.Deployment{}
	dat, err := ioutil.ReadFile("../../deploy/operator.yaml")

	if err != nil {
		return errors.Wrap(err, "fail")
	}

	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(dat)), 100).Decode(operatorDepl)

	if err != nil {
		return errors.Wrap(err, "fail")
	}

	for _, env := range operatorDepl.Spec.Template.Spec.Containers[0].Env {
		os.Setenv(env.Name, env.Value)
	}

	type runtimeFile struct {
		filename string
		findType func(string) runtime.Object
	}

	loadFiles := []runtimeFile{
		{"../../deploy/service_account.yaml", func(dat string) runtime.Object {
			sa := &corev1.ServiceAccount{}
			sa.Namespace = d.Namespace
			sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{
				Name: d.PullSecretName,
			})
			return sa
		}},
		{"../../deploy/role.yaml", func(dat string) runtime.Object {
			switch {
			case strings.Contains(dat, "kind: Role"):
				GinkgoWriter.Write([]byte("adding kind Role\n"))
				obj := &rbacv1.Role{}
				return obj
			case strings.Contains(dat, "kind: ClusterRole"):
				GinkgoWriter.Write([]byte("adding kind ClusterRole\n"))
				obj := &rbacv1.ClusterRole{}
				return obj
			default:
				GinkgoWriter.Write([]byte("type not found\n"))
				return nil
			}
		}},
		{"../../deploy/role_binding.yaml", func(dat string) runtime.Object {
			switch {
			case strings.Contains(dat, "kind: RoleBinding"):
				obj := &rbacv1.RoleBinding{}
				GinkgoWriter.Write([]byte("adding kind RoleBinding\n"))
				return obj
			case strings.Contains(dat, "kind: ClusterRoleBinding"):
				obj := &rbacv1.ClusterRoleBinding{}
				GinkgoWriter.Write([]byte("adding kind ClusterRoleBinding\n"))
				return obj
			default:
				GinkgoWriter.Write([]byte("type not found\n"))
				return nil
			}
		}},
	}

	for _, rec := range loadFiles {
		dat, err := ioutil.ReadFile(rec.filename)
		Expect(err).ShouldNot(HaveOccurred())

		datArray := strings.Split(string(dat), "---")
		for _, dat := range datArray {
			obj := rec.findType(dat)

			if obj == nil || len(dat) == 0 {
				continue
			}

			err := yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(dat)), 100).Decode(obj)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(h.Create(context.TODO(), obj)).To(SucceedOrAlreadyExist, rec.filename, " ", obj)
			d.cleanup = append(d.cleanup, obj)
		}
	}

	return nil
}

type createMarketplaceConfig struct {
	Namespace string `env:"NAMESPACE" envDefault:"openshift-redhat-marketplace"`

	marketplaceCfg *v1alpha1.MarketplaceConfig
	base           *v1alpha1.MeterBase
}

func (d *createMarketplaceConfig) Name() string {
	return "CreateMarketplaceConfig"
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

	Expect(t.Create(context.TODO(), d.marketplaceCfg)).Should(SucceedOrAlreadyExist)

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
