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
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	// register tests
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/testenv/apis"
)

const namespace = "openshift-redhat-marketplace"

var cfg *rest.Config
var k8sClient client.Client
var k8sManager ctrl.Manager
var testEnv *envtest.Environment
var cc reconcileutils.ClientCommandRunner
var stop chan struct{}

var toDelete []runtime.Object

func TestEnv(t *testing.T) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller EnvTest Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
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

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	ctrlMain, err := initializeMainCtrl(cfg)
	Expect(err).ToNot(HaveOccurred())
	Expect(ctrlMain).ToNot(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: ctrlMain.Manager.GetScheme()})
	Expect(err).NotTo(HaveOccurred())

	k8sManager = ctrlMain.Manager
	apis.K8sClient = k8sClient

	Expect(k8sClient).ToNot(BeNil())

	additionalNamespaces := []string{
		"openshift-monitoring",
		"openshift-config-managed",
		"openshift-config",
		namespace,
	}

	for _, namespace := range additionalNamespaces {
		Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		})).To(SucceedOrAlreadyExist)
	}

	// apply roles, sa
	command := exec.Command("make", "clean", "helm")
	command.Stderr = GinkgoWriter
	command.Stdout = GinkgoWriter
	command.Dir = "../.."
	err = command.Run()
	out, _ := command.CombinedOutput()
	Expect(err).ShouldNot(HaveOccurred(), string(out))

	type runtimeFile struct {
		filename string
		findType func(string) runtime.Object
	}

	loadFiles := []runtimeFile{
		{"../../deploy/service_account.yaml", func(dat string) runtime.Object {
			sa := &corev1.ServiceAccount{}
			sa.Namespace = namespace
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

			Expect(k8sClient.Create(context.Background(), obj)).To(SucceedOrAlreadyExist, rec.filename, " ", obj)
			toDelete = append(toDelete, obj)
		}
	}

	operatorDepl := &appsv1.Deployment{}
	dat, err := ioutil.ReadFile("../../deploy/operator.yaml")
	Expect(err).ShouldNot(HaveOccurred())
	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(dat)), 100).Decode(operatorDepl)
	Expect(err).ShouldNot(HaveOccurred())

	By("adding certs")
	addCerts()

	for _, env := range operatorDepl.Spec.Template.Spec.Containers[0].Env {
		if env.Value != "" {
			os.Setenv(env.Name, env.Value)
		}
	}

	cc = reconcileutils.NewLoglessClientCommand(k8sClient, k8sManager.GetScheme())

	stop = make(chan struct{})

	time.Sleep(time.Second * 5)

	go func() {
		ctrlMain.Run(stop)
	}()

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("clean up certs")
	cleanupCerts()

	By("shutting down controller")
	close(stop)

	By("tearing down the test environment")
	gexec.KillAndWait(5 * time.Second)

	// cleanup created objects
	for _, obj := range toDelete {
		k8sClient.Delete(context.Background(), obj)
	}

	if testEnv != nil {
		err := testEnv.Stop()
		Expect(err).ToNot(HaveOccurred())
	}
})

var SucceedOrAlreadyExist types.GomegaMatcher = SatisfyAny(
	Succeed(),
	WithTransform(errors.IsAlreadyExists, BeTrue()),
)


var IsNotFound types.GomegaMatcher = WithTransform(errors.IsNotFound, BeTrue())
