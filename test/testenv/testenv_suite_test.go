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

package testenv

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	// register tests
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/testenv/apis"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/testenv/controller"
)

const (
	timeout   = time.Second * 180
	interval  = time.Second * 1
	Namespace = "openshift-redhat-marketplace"
)

var (
	cfg        *rest.Config
	k8sClient  client.Client
	k8sManager ctrl.Manager
	testEnv    *envtest.Environment
	cc         reconcileutils.ClientCommandRunner
	stop       chan struct{}
	toDelete   []runtime.Object
	base       = &v1alpha1.MeterBase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rhm-marketplaceconfig-meterbase",
			Namespace: "openshift-redhat-marketplace",
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
	marketplaceCfg = &v1alpha1.MarketplaceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.MARKETPLACECONFIG_NAME,
			Namespace: Namespace,
		},
		Spec: v1alpha1.MarketplaceConfigSpec{
			ClusterUUID:             "fooCluster",
			RhmAccountID:            "fooID",
			EnableMetering:          ptr.Bool(true),
			InstallIBMCatalogSource: ptr.Bool(false),
		},
	}
)

func TestEnv(t *testing.T) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller EnvTest Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	By("setting up env")
	Expect(os.Setenv("TEST_ASSET_KUBE_APISERVER", "../../testbin/kube-apiserver")).To(Succeed())
	Expect(os.Setenv("TEST_ASSET_ETCD", "../../testbin/etcd")).To(Succeed())
	Expect(os.Setenv("TEST_ASSET_KUBECTL", "../../testbin/kubectl")).To(Succeed())
	Expect(os.Setenv("WATCH_NAMESPACE", "")).To(Succeed())

	By("bootstrapping test environment")
	t := true
	if os.Getenv("TEST_USE_EXISTING_CLUSTER") == "true" {
		testEnv = &envtest.Environment{
			UseExistingCluster: &t,
		}
	} else {
		testEnv = &envtest.Environment{
			CRDDirectoryPaths: []string{
				filepath.Join("..", "..", "deploy", "crds"),
				filepath.Join(".", "testdata"),
			},
		}
	}

	// apply roles, sa
	tag := os.Getenv("OPERATOR_IMAGE_TAG")
	if tag == "" {
		By(fmt.Sprintf("building tag %s", tag))
		os.Setenv("OPERATOR_IMAGE_TAG", fmt.Sprintf("%s-%s", "test", rand.String(6)))
		command := exec.Command("make", "build-and-load-kind")
		command.Stderr = GinkgoWriter
		command.Stdout = GinkgoWriter
		command.Dir = "../.."
		command.Env = os.Environ()
		err := command.Run()
		out, _ := command.CombinedOutput()
		Expect(err).ShouldNot(HaveOccurred(), string(out))
	}

	tag = os.Getenv("OPERATOR_IMAGE_TAG")
	By(fmt.Sprintf("testing with tag %s", tag))

	command := exec.Command("make", "clean", "helm")
	command.Stderr = GinkgoWriter
	command.Stdout = GinkgoWriter
	command.Dir = "../.."
	command.Env = os.Environ()
	err := command.Run()
	out, _ := command.CombinedOutput()
	Expect(err).ShouldNot(HaveOccurred(), string(out))

	operatorDepl := &appsv1.Deployment{}
	dat, err := ioutil.ReadFile("../../deploy/operator.yaml")
	Expect(err).ShouldNot(HaveOccurred())
	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(dat)), 100).Decode(operatorDepl)
	Expect(err).ShouldNot(HaveOccurred())

	for _, env := range operatorDepl.Spec.Template.Spec.Containers[0].Env {
		os.Setenv(env.Name, env.Value)
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	ctrlMain, err := initializeMainCtrl(cfg)
	Expect(err).ToNot(HaveOccurred())
	Expect(ctrlMain).ToNot(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: ctrlMain.Manager.GetScheme()})
	Expect(err).NotTo(HaveOccurred())

	k8sManager = ctrlMain.Manager

	Expect(k8sClient).ToNot(BeNil())

	additionalNamespaces := []string{
		"openshift-monitoring",
		"openshift-config-managed",
		"openshift-config",
		Namespace,
	}

	for _, namespace := range additionalNamespaces {
		Expect(k8sClient.Create(context.TODO(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		})).To(SucceedOrAlreadyExist)
	}

	home := os.Getenv("HOME")
	pullSecret := &corev1.Secret{}
	err = k8sClient.Get(context.TODO(),
		k8stypes.NamespacedName{
			Name:      "local-pull-secret",
			Namespace: Namespace,
		}, pullSecret)

	if errors.IsNotFound(err) {
		command = exec.Command("kubectl", "create", "secret", "generic",
			"local-pull-secret",
			"-n", Namespace,
			fmt.Sprintf("--from-file=.dockerconfigjson=%s/.docker/config.json", home),
			"--type=kubernetes.io/dockerconfigjson")
		command.Stderr = GinkgoWriter
		command.Stdout = GinkgoWriter
		command.Dir = "../.."
		err = command.Run()
		out, _ = command.CombinedOutput()
		Expect(err).ShouldNot(HaveOccurred(), string(out))
	}

	By("loading files")
	loadFiles()

	By("adding certs")
	addCerts()
	CreatePrereqs()
	cc = reconcileutils.NewLoglessClientCommand(k8sClient, k8sManager.GetScheme())

	apis.K8sClient = k8sClient
	controller.K8sClient = k8sClient
	controller.CC = cc

	stop = make(chan struct{})

	go func() {
		ctrlMain.Run(stop)
	}()

	Expect(ctrlMain.Manager.GetCache().WaitForCacheSync(stop)).Should(BeTrue())

	time.Sleep(10 * time.Second)

	By("create prereqs")
	Expect(k8sClient.Create(context.TODO(), marketplaceCfg)).Should(SucceedOrAlreadyExist)

	By("wait for marketplaceconfig")
	Eventually(func() bool {
		err := k8sClient.Get(context.TODO(), k8stypes.NamespacedName{Name: base.Name, Namespace: Namespace}, base)

		if err != nil {
			return false
		}

		return true
	}, timeout).Should(BeTrue())

	By("update meterbase")
	base.Spec = v1alpha1.MeterBaseSpec{
		Enabled: true,
		Prometheus: &v1alpha1.PrometheusSpec{
			Storage: v1alpha1.StorageSpec{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: "",
				},
				Size: resource.MustParse("1Gi"),
			},
		},
	}
	By("update meterbase")
	Expect(k8sClient.Update(context.TODO(), base)).Should(Succeed())
	By("wait for meterbase")
	WaitForMeterBaseToDeploy(base)
})

var _ = AfterSuite(func() {
	By("delete prereqs")
	k8sClient.Delete(context.TODO(), marketplaceCfg)
	k8sClient.Delete(context.TODO(), base)
	time.Sleep(30 * time.Second)

	pullSecret := &corev1.Secret{}
	err := k8sClient.Get(context.TODO(),
		k8stypes.NamespacedName{
			Name:      "local-pull-secret",
			Namespace: Namespace,
		}, pullSecret)
	if err == nil {
		k8sClient.Delete(context.TODO(), pullSecret)
	}

	time.Sleep(10 * time.Second)

	By("clean up certs")
	cleanupCerts()
	DeletePrereqs()

	By("shutting down controller")
	close(stop)

	By("tearing down the test environment")
	gexec.KillAndWait(5 * time.Second)

	// cleanup created objects
	for _, obj := range toDelete {
		k8sClient.Delete(context.TODO(), obj)
	}

	if testEnv != nil {
		err := testEnv.Stop()
		Expect(err).ToNot(HaveOccurred())
	}
}, 60)

var SucceedOrAlreadyExist types.GomegaMatcher = SatisfyAny(
	Succeed(),
	WithTransform(errors.IsAlreadyExists, BeTrue()),
)

var IsNotFound types.GomegaMatcher = WithTransform(errors.IsNotFound, BeTrue())

func WaitForMeterBaseToDeploy(base *v1alpha1.MeterBase) {
	Eventually(func() map[string]interface{} {
		result, _ := cc.Do(
			context.TODO(),
			GetAction(k8stypes.NamespacedName{Name: base.Name, Namespace: Namespace}, base),
		)

		if !result.Is(Continue) || base.Status.Conditions == nil {
			return map[string]interface{}{
				"resultStatus": result.Status,
			}
		}

		cond := base.Status.Conditions.GetCondition(v1alpha1.ConditionInstalling)

		return map[string]interface{}{
			"resultStatus":      result.Status,
			"conditionStatus":   cond.Status,
			"reason":            cond.Reason,
			"availableReplicas": base.Status.AvailableReplicas,
		}
	}, timeout, interval).Should(
		MatchAllKeys(Keys{
			"resultStatus":      Equal(Continue),
			"conditionStatus":   Equal(corev1.ConditionFalse),
			"reason":            Equal(v1alpha1.ReasonMeterBaseFinishInstall),
			"availableReplicas": PointTo(BeNumerically("==", 2)),
		}),
	)
}

func WaitForMeterBaseToFinalize(base *v1alpha1.MeterBase) {
	Eventually(func() bool {
		err := k8sClient.Get(context.TODO(),
			k8stypes.NamespacedName{Name: base.Name, Namespace: Namespace}, base)

		if err == nil {
			return false
		}

		return errors.IsNotFound(err)
	}, timeout, interval).Should(BeTrue())
}

func loadFiles() {
	type runtimeFile struct {
		filename string
		findType func(string) runtime.Object
	}

	loadFiles := []runtimeFile{
		{"../../deploy/service_account.yaml", func(dat string) runtime.Object {
			sa := &corev1.ServiceAccount{}
			sa.Namespace = Namespace
			sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{
				Name: "local-pull-secret",
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

			Expect(k8sClient.Create(context.TODO(), obj)).To(SucceedOrAlreadyExist, rec.filename, " ", obj)
			toDelete = append(toDelete, obj)
		}
	}

}

// func debugPods(done chan interface{}) {
// 	w := GinkgoWriter
// 	output := func(s string) error {
// 		return io.WriteString(GinkgoWriter, s)
// 	}

// 	for {
// 		select {
// 		case <- done:
// 			return
// 		case
// 		}
// 	}
// }
