package testenv

import (
	"context"
	"time"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MeterbaseController", func() {

	const timeout = time.Second * 30
	const interval = time.Second * 1

	var (
		base             *v1alpha1.MeterBase
		kubeletMonitor   *monitoringv1.ServiceMonitor
		kubeStateMonitor *monitoringv1.ServiceMonitor
	)

	Context("MeterBase reconcile", func() {
		Context("creating a meterbase", func() {
			BeforeEach(func() {
				base = &v1alpha1.MeterBase{
					ObjectMeta: v1.ObjectMeta{
						Name:      "rhm-marketplaceconfig-meterbase",
						Namespace: "openshift-redhat-marketplace",
					},
					Spec: v1alpha1.MeterBaseSpec{
						Enabled: true,
						Prometheus: &v1alpha1.PrometheusSpec{
							Storage: v1alpha1.StorageSpec{
								Class: ptr.String("default"),
								Size:  resource.MustParse("20Gi"),
							},
						},
					},
				}

				kubeletMonitor = &monitoringv1.ServiceMonitor{
					ObjectMeta: v1.ObjectMeta{
						Name:      "kubelet",
						Namespace: "openshift-monitoring",
					},
					Spec: monitoringv1.ServiceMonitorSpec{
						Endpoints: []monitoringv1.Endpoint{
							{
								Port: "web",
							},
						},
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"foo": "bar",
							},
						},
					},
				}

				kubeStateMonitor = &monitoringv1.ServiceMonitor{
					ObjectMeta: v1.ObjectMeta{
						Name:      "kube-state-metrics",
						Namespace: "openshift-monitoring",
					},
					Spec: monitoringv1.ServiceMonitorSpec{
						Endpoints: []monitoringv1.Endpoint{
							{
								Port: "web",
							},
						},
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"foo": "bar",
							},
						},
					},
				}

				Expect(k8sClient.Create(context.Background(), kubeletMonitor)).Should(Succeed())
				Expect(k8sClient.Create(context.Background(), kubeStateMonitor)).Should(Succeed())
			})

			It("should create all assets", func() {
				cm := &corev1.ConfigMap{}
				deployment := &appsv1.Deployment{}
				service := &corev1.Service{}

				Expect(k8sClient.Create(context.Background(), base)).Should(Succeed())

				By("create prometheus operator")
				Eventually(func() bool {
					result, _ := cc.Do(
						context.Background(),
						GetAction(types.NamespacedName{Name: "operator-certs-ca-bundle", Namespace: namespace}, cm),
						GetAction(types.NamespacedName{Name: "prometheus-operator", Namespace: namespace}, deployment),
						GetAction(types.NamespacedName{Name: "prometheus-operator", Namespace: namespace}, service),
					)
					return result.Is(Continue)
				}, timeout, interval).Should(BeTrue())

				By("creating metric-state")

				deployment = &appsv1.Deployment{}
				service = &corev1.Service{}
				serviceMonitor := &monitoringv1.ServiceMonitor{}

				Eventually(func() bool {
					result, _ := cc.Do(
						context.Background(),
						GetAction(types.NamespacedName{Name: "rhm-metric-state", Namespace: namespace}, deployment),
						GetAction(types.NamespacedName{Name: "rhm-metric-state-service", Namespace: namespace}, service),
						GetAction(types.NamespacedName{Name: "rhm-metric-state", Namespace: namespace}, serviceMonitor),
					)
					return result.Is(Continue)
				}, timeout, interval).Should(BeTrue())

				By("creating additional config secret")

				secret := &corev1.Secret{}

				Eventually(func() bool {
					result, _ := cc.Do(
						context.Background(),
						GetAction(types.NamespacedName{Name: "rhm-meterbase-additional-scrape-configs", Namespace: namespace}, secret),
					)
					return result.Is(Continue)
				}, timeout, interval).Should(BeTrue())

				By("finish installing")

				Eventually(func() bool {
					result, _ := cc.Do(
						context.Background(),
						GetAction(types.NamespacedName{Name: base.Name, Namespace: namespace}, base),
					)

					return result.Is(Continue) &&
						base.Status.Conditions.IsFalseFor(v1alpha1.ConditionInstalling) &&
						base.Status.Conditions.GetCondition(v1alpha1.ConditionInstalling).Reason == v1alpha1.ReasonMeterBaseFinishInstall
				}, timeout, interval)
			})
		})
	})
})
