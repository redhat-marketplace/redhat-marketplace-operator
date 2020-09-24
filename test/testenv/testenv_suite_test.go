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
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"

	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	//"k8s.io/apimachinery/pkg/runtime"
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

func TestTestenv(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Testenv Suite")
}

const namespace = "openshift-redhat-marketplace"

var cfg *rest.Config
var k8sClient client.Client
var k8sManager ctrl.Manager
var testEnv *envtest.Environment
var cc reconcileutils.ClientCommandRunner
var stop chan struct{}

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

	stop = make(chan struct{})

	go func() {
		ctrlMain.Run(stop)
	}()

	k8sManager = ctrlMain.Manager
	apis.K8sClient = k8sClient

	Expect(k8sClient).ToNot(BeNil())
	Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	})).To(SatisfyAny(
		Succeed(),
		WithTransform(errors.IsAlreadyExists, BeTrue()),
	))
	Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "openshift-monitoring",
		},
	})).To(SucceedOrAlreadyExist)

	cc = reconcileutils.NewLoglessClientCommand(k8sClient, k8sManager.GetScheme())

	time.Sleep(time.Second * 5)

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("shutting down controller")
	close(stop)

	By("tearing down the test environment")
	gexec.KillAndWait(5 * time.Second)

	if testEnv != nil {
		err := testEnv.Stop()
		Expect(err).ToNot(HaveOccurred())
	}
})

var SucceedOrAlreadyExist types.GomegaMatcher = SatisfyAny(
	Succeed(),
	WithTransform(errors.IsAlreadyExists, BeTrue()),
)
