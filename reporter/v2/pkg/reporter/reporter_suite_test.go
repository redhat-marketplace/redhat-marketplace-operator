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

package reporter

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestReporter(t *testing.T) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reporter Suite")
}

var (
	cfg         *rest.Config
	operatorCfg *config.OperatorConfig
	k8sClient   client.Client
	k8sInter    kubernetes.Interface
	testEnv     *envtest.Environment
	k8sScheme   *runtime.Scheme
	factory     *manifests.Factory
	doneChan    chan struct{}

	eb      record.EventBroadcaster
	closeEB func()
)

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	os.Setenv("KUBEBUILDER_CONTROLPLANE_START_TIMEOUT", "2m")

	doneChan = make(chan struct{})
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "..", "v2", "config", "crd", "bases"),
			filepath.Join("..", "..", "..", "..", "tests", "v2", "testdata"),
		}, KubeAPIServerFlags: append(envtest.DefaultKubeAPIServerFlags, "--bind-address=127.0.0.1"),
	}

	k8sScheme = provideScheme()

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	k8sInter = kubernetes.NewForConfigOrDie(cfg)
	eb, closeEB = provideReporterEventBroadcaster(k8sInter)

	k8sClient, err = client.New(cfg, client.Options{Scheme: k8sScheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	k8sClient.Create(context.Background(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "openshift-redhat-marketplace",
		},
	})
})

var _ = AfterSuite(func() {
	close(doneChan)
	closeEB()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
