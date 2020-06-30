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

package meterbase

import (
	"context"

	merrors "emperror.dev/errors"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/test/rectest"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("MeterbaseController", func() {
	var (
		name, namespace string
		meterbase       *marketplacev1alpha1.MeterBase
		ctrlScheme      *runtime.Scheme
		req             reconcile.Request
		options         []StepOption
		storageClass    *storagev1.StorageClass
	)

	BeforeEach(func() {
		namespace = "redhat-marketplace-operator"
		name = "rhm-marketplace"
		meterbase = &marketplacev1alpha1.MeterBase{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: marketplacev1alpha1.MeterBaseSpec{
				Enabled: true,
				Prometheus: &marketplacev1alpha1.PrometheusSpec{
					Storage: marketplacev1alpha1.StorageSpec{
						Size: resource.MustParse("30Gi"),
					},
				},
			},
		}
		req = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}
		options = []StepOption{
			WithRequest(req),
		}
		storageClass = &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default-storage",
				Namespace: "",
				Annotations: map[string]string{
					"storageclass.kubernetes.io/is-default-class": "true",
				},
			},
			Provisioner: "foo",
		}

		viper.Set("assets", "../../../assets")

		ctrlScheme = runtime.NewScheme()
		scheme.AddToScheme(ctrlScheme)
		olmv1alpha1.AddToScheme(ctrlScheme)
		monitoringv1.AddToScheme(ctrlScheme)
		ctrlScheme.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, meterbase)
	})

	Describe("check test flags", func() {
		It("should compile flags", func() {
			flagset := FlagSet()
			Expect(flagset.HasFlags()).To(BeTrue(), "no flags on the flagset")
		})
	})

	// Describe("when kubelet and kubestate missing", func() {
	// 	var (
	// 		client   client.Client
	// 		ctrl     *ReconcileMeterBase
	// 		test     *ReconcilerTest
	// 		ctx      context.Context
	// 		mockCtrl *gomock.Controller
	// 	)

	// 	BeforeEach(func() {
	// 		ctrl = &ReconcileMeterBase{
	// 			client:       client,
	// 			scheme:       ctrlScheme,
	// 			ccprovider:   &reconcileutils.DefaultCommandRunnerProvider{},
	// 			patchChecker: reconcileutils.NewPatchChecker(utils.RhmPatchMaker),
	// 			opts: &MeterbaseOpts{
	// 				PullPolicy: v1.PullAlways,
	// 				AssetPath:  "../../../assets",
	// 			},
	// 		}

	// 		client = ClientErrorStub(mockCtrl,
	// 			fake.NewFakeClientWithScheme(ctrlScheme, meterbase, storageClass, kubelet, kubestate),
	// 			mockErr)

	// 		test = NewReconcilerTestSimple(ctrl, client)
	// 		ctx = context.TODO()

	// 		test.TestAll(GinkgoT(),
	// 			ReconcileStep(options,
	// 				ReconcileWithUntilDone(true),
	// 			),
	// 		)
	// 	})

	// 	ExpectGetObject := func(name, namespace string, obj runtime.Object) Assertion {
	// 		return Expect(client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, obj))
	// 	}

	// 	It("should create a prometheus sub", func() {
	// 		ExpectGetObject(name, namespace, &olmv1alpha1.Subscription{}).To(Succeed())
	// 	})
	// 	It("should create a prometheus obj", func() {
	// 		ExpectGetObject(name, namespace, &monitoringv1.Prometheus{}).To(Succeed())
	// 	})
	// 	It("should create a prometheus service", func() {
	// 		ExpectGetObject(name, namespace, &corev1.Service{}).To(Succeed())
	// 	})
	// 	It("should not create service monitors", func() {
	// 		ExpectGetObject("rhm-kube-state-metrics", namespace, &monitoringv1.ServiceMonitor{}).To(HaveOccurred())
	// 		ExpectGetObject("rhm-kubelet", namespace, &monitoringv1.ServiceMonitor{}).To(HaveOccurred())
	// 	})
	// })

	Describe("when the client errors", func() {
		var (
			client             client.Client
			ctrl               *ReconcileMeterBase
			test               *ReconcilerTest
			mockCtrl           *gomock.Controller
			ctx                context.Context
			kubelet, kubestate *monitoringv1.ServiceMonitor
			mockErr            error
		)

		AfterEach(func() {
			mockCtrl.Finish()
		})

		BeforeEach(func() {
			logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))
			mockErr = merrors.New("mock error")
			mockCtrl = gomock.NewController(GinkgoT())
			kubelet = &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubelet",
					Namespace: "openshift-monitoring",
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					NamespaceSelector: monitoringv1.NamespaceSelector{
						MatchNames: []string{"a", "b"},
					},
				},
			}
			kubestate = &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-state-metrics",
					Namespace: "openshift-monitoring",
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					NamespaceSelector: monitoringv1.NamespaceSelector{
						MatchNames: []string{"a", "b"},
					},
				},
			}
		})

		ExpectGetObject := func(name, namespace string, obj runtime.Object) Assertion {
			return Expect(client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, obj))
		}

		Context("when kubestate and kubelet don't exists", func() {
			BeforeEach(func() {
				client = ClientErrorStub(mockCtrl,
					fake.NewFakeClientWithScheme(ctrlScheme, meterbase, storageClass),
					mockErr)

				ctrl = &ReconcileMeterBase{
					client:       client,
					scheme:       ctrlScheme,
					ccprovider:   &reconcileutils.DefaultCommandRunnerProvider{},
					patchChecker: reconcileutils.NewPatchChecker(utils.RhmPatchMaker),
					opts: &MeterbaseOpts{
						PullPolicy: v1.PullAlways,
						AssetPath:  "../../../assets",
					},
				}

				test = NewReconcilerTestSimple(ctrl, client)
				ctx = context.TODO()

				test.TestAll(GinkgoT(),
					ReconcileStep(options,
						ReconcileWithUntilDone(true),
						ReconcileWithIgnoreError(true),
					),
				)
			})

			It("should create service monitors", func() {
				ExpectGetObject(name, namespace, &olmv1alpha1.Subscription{}).To(Succeed())
				ExpectGetObject(name, namespace, &monitoringv1.Prometheus{}).To(Succeed())
				ExpectGetObject(name, namespace, &corev1.Service{}).To(Succeed())
				ExpectGetObject("rhm-kube-state-metrics", namespace, &monitoringv1.ServiceMonitor{}).To(HaveOccurred())
				ExpectGetObject("rhm-kubelet", namespace, &monitoringv1.ServiceMonitor{}).To(HaveOccurred())
			})

		})

		Context("when kubestate and kubelet are present", func() {
			BeforeEach(func() {
				client = ClientErrorStub(mockCtrl,
					fake.NewFakeClientWithScheme(ctrlScheme, meterbase, storageClass, kubelet, kubestate),
					mockErr)

				ctrl = &ReconcileMeterBase{
					client:       client,
					scheme:       ctrlScheme,
					ccprovider:   &reconcileutils.DefaultCommandRunnerProvider{},
					patchChecker: reconcileutils.NewPatchChecker(utils.RhmPatchMaker),
					opts: &MeterbaseOpts{
						PullPolicy: v1.PullAlways,
						AssetPath:  "../../../assets",
					},
				}

				test = NewReconcilerTestSimple(ctrl, client)
				ctx = context.TODO()

				test.TestAll(GinkgoT(),
					ReconcileStep(options,
						ReconcileWithUntilDone(true),
						ReconcileWithIgnoreError(true),
					),
				)
			})

			It("should create service monitors", func() {
				ExpectGetObject(name, namespace, &olmv1alpha1.Subscription{}).To(Succeed())
				ExpectGetObject(name, namespace, &monitoringv1.Prometheus{}).To(Succeed())
				ExpectGetObject(name, namespace, &corev1.Service{}).To(Succeed())
				ExpectGetObject("rhm-kube-state-metrics", namespace, &monitoringv1.ServiceMonitor{}).To(Succeed())
				ExpectGetObject("rhm-kubelet", namespace, &monitoringv1.ServiceMonitor{}).To(Succeed())
			})
		})

		Context("when updates happen to prometheus", func() {
			BeforeEach(func() {
				prom := &monitoringv1.Prometheus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
					},
					Spec: monitoringv1.PrometheusSpec{
						ServiceMonitorNamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"foo": "bar",
							},
						},
					},
				}
				client = ClientErrorStub(mockCtrl,
					fake.NewFakeClientWithScheme(ctrlScheme, meterbase, storageClass, kubelet, kubestate, prom),
					mockErr)

				ctrl = &ReconcileMeterBase{
					client:       client,
					scheme:       ctrlScheme,
					ccprovider:   &reconcileutils.DefaultCommandRunnerProvider{},
					patchChecker: reconcileutils.NewPatchChecker(utils.RhmPatchMaker),
					opts: &MeterbaseOpts{
						PullPolicy: v1.PullAlways,
						AssetPath:  "../../../assets",
					},
				}

				test = NewReconcilerTestSimple(ctrl, client)
				ctx = context.TODO()

				test.TestAll(GinkgoT(),
					ReconcileStep(options,
						ReconcileWithUntilDone(true),
						ReconcileWithIgnoreError(true),
					),
				)
			})

			It("should update prometheus to default", func() {
				ExpectGetObject(name, namespace, &olmv1alpha1.Subscription{}).To(Succeed())
				ExpectGetObject(name, namespace, &monitoringv1.Prometheus{}).To(Succeed())
				ExpectGetObject("rhm-kube-state-metrics", namespace, &monitoringv1.ServiceMonitor{}).To(Succeed())
				ExpectGetObject("rhm-kubelet", namespace, &monitoringv1.ServiceMonitor{}).To(Succeed())
				ExpectGetObject(name, namespace, &corev1.Service{}).To(Succeed())
			})
		})
	})

})
