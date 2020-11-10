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
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"

	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	// register tests
	"github.com/onsi/ginkgo/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/testenv/apis"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/testenv/controller"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/testenv/harness"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	timeout   = time.Second * 300
	interval  = time.Second * 1
	Namespace = "openshift-redhat-marketplace"
)

var (
	cfg         *rest.Config
	testHarness *harness.TestHarness
	testEnv     *envtest.Environment
	cc          reconcileutils.ClientCommandRunner
	stop        chan struct{}
)

func TestEnv(t *testing.T) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))
	RegisterFailHandler(PodFailHandler)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller EnvTest Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	By("setting up env")

	t := true
	// testEnv = &envtest.Environment{
	// 	UseExistingCluster: &t,
	// }
	testEnv = &envtest.Environment{
		UseExistingCluster: &t,
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "deploy", "crds"),
			filepath.Join(".", "testdata"),
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())

	scheme, err := initializeScheme(cfg)
	Expect(err).To(Succeed())

	testHarness, err = harness.NewTestHarness(scheme, testEnv, harness.TestHarnessOptions{
		EnabledFeatures: []string{"all"},
		Namespace:       "openshift-redhat-marketplace",
		WatchNamespace:  "",
	})
	Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%v+", err))

	err = testHarness.Start()
	Expect(err).ToNot(HaveOccurred())

	controller.TestHarness = testHarness
	controller.K8sClient = testHarness.Client
	apis.K8sClient = testHarness.Client
	cc = reconcileutils.NewLoglessClientCommand(testHarness.Client, scheme)
	controller.CC = cc

	stop = make(chan struct{})

	mainCtrl, err := initializeMainCtrl(cfg)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		mainCtrl.Run(stop)
	}()
})

var _ = AfterSuite(func() {
	if CurrentGinkgoTestDescription().Failed {
		printDebug()
	}

	close(stop)

	By("tearing down the test environment")
	gexec.KillAndWait(5 * time.Second)

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

func PodFailHandler(message string, callerSkip ...int) {
	printDebug()
	Fail(message, callerSkip...)
}

func printDebug() {
	lists := []runtime.Object{
		&corev1.PodList{},
		&appsv1.DeploymentList{},
		&appsv1.StatefulSetList{},
		&v1alpha1.MeterBaseList{},
		&v1alpha1.RazeeDeploymentList{},
		&v1alpha1.MarketplaceConfigList{},
	}

	filters := []func(runtime.Object) bool{
		func(obj runtime.Object) bool {
			pod, ok := obj.(*corev1.Pod)

			if !ok {
				return false
			}

			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.ContainersReady {
					if cond.Status == corev1.ConditionTrue {
						return true
					}
				}
			}
			return false
		},
	}

	for _, list := range lists {
		testHarness.List(context.TODO(), list, client.InNamespace(Namespace))
		printList(list, filters)
	}
}

func printList(list runtime.Object, filters []func(runtime.Object) bool) {
	preamble := "\x1b[1mDEBUG %T\x1b[0m"
	if config.DefaultReporterConfig.NoColor {
		preamble = "DEBUG %T"
	}

	extractedList, err := meta.ExtractList(list)

	if err != nil {
		return
	}

printloop:
	for _, item := range extractedList {
		for _, filter := range filters {
			if filter(item) {
				continue printloop
			}
		}

		typePre := fmt.Sprintf(preamble, item)
		access, _ := meta.Accessor(item)

		access.SetManagedFields([]metav1.ManagedFieldsEntry{})
		data, _ := json.MarshalIndent(item, "", "  ")

		fmt.Fprintf(GinkgoWriter,
			"%s: %s/%s debug output: %s\n", typePre, access.GetName(), access.GetNamespace(), string(data))
	}
}
