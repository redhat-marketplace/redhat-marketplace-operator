package metric_generator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"emperror.dev/errors"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/client"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	collectorRuntime = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "rhm_meter_collector_runtime",
		Help: "Elapsed time of the collector query",
	})
)

type StopCollector chan struct{}

type Collector struct {
	cc      ClientCommandRunner
	refresh time.Duration
	cache   cache.Cache

	stop        StopCollector
	podListener []chan *corev1.Pod
}

type CollectorRefreshRate time.Duration

func NewMeterCollector(
	cc ClientCommandRunner,
	refresh CollectorRefreshRate,
	cache cache.Cache,
	stop StopCollector,
) *Collector {
	return &Collector{
		cc:          cc,
		refresh:     time.Duration(refresh),
		cache:       cache,
		stop:        stop,
		podListener: []chan *corev1.Pod{},
	}
}

func (c *Collector) Register() error {
	err := rhmclient.AddGVKIndexer(c.cache)
	if err != nil {
		return err
	}

	return nil
}

func (c *Collector) RegisterPodListener(podChan chan *corev1.Pod) {
	c.podListener = append(c.podListener, podChan)
}

func (c *Collector) Run() {
	ticker := time.Tick(c.refresh)
	for {
		select {
		case <-c.stop:
			return
		case <-ticker:
			ctx := context.Background()
			err := c.Collect(ctx)
			if err != nil {
				log.Error(err, "error running collector")
			}
		}
	}
}

func (c *Collector) Collect(ctx context.Context) error {
	timer := prometheus.NewTimer(collectorRuntime)
	defer timer.ObserveDuration()

	meterDefs, err := c.collectMeterDefs(ctx)

	if err != nil {
		return err
	}

	for _, meterDef := range meterDefs {
		err := c.processPods(ctx, &meterDef)

		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Collector) processPods(ctx context.Context, meterDef *marketplacev1alpha1.MeterDefinition) error {
	pods, err := c.collectPods(ctx, meterDef)

	if err != nil {
		return err
	}

	for _, pod := range pods {
		pod.ObjectMeta.Annotations["meter_def_domain"] = meterDef.Spec.Group
		pod.ObjectMeta.Annotations["meter_def_kind"] = meterDef.Spec.Kind
		pod.ObjectMeta.Annotations["meter_def_version"] = meterDef.Spec.Version

		for _, ch := range c.podListener {
			ch <- &pod
		}
	}

	return nil
}

func (c *Collector) collectMeterDefs(ctx context.Context) ([]marketplacev1alpha1.MeterDefinition, error) {
	namespaces := &corev1.NamespaceList{}
	list := &marketplacev1alpha1.MeterDefinitionList{}

	result, _ := c.cc.Do(
		ctx,
		ListAction(namespaces),
		Call(func() (ClientAction, error) {
			actions := []ClientAction{}

			for _, ns := range namespaces.Items {
				actions = append(actions, ListAppendAction(list, client.InNamespace(ns.GetName())))
			}

			return Do(actions...), nil
		}),
	)

	if result.Is(Error) {
		return []marketplacev1alpha1.MeterDefinition{}, errors.Wrap(result.Err, "failed to list meterdef")
	}

	return list.Items, nil
}

func (c *Collector) collectPods(ctx context.Context, instance *marketplacev1alpha1.MeterDefinition) ([]corev1.Pod, error) {
	cc := c.cc

	gvkStr := strings.ToLower(fmt.Sprintf("%s.%s.%s", instance.Spec.Kind, instance.Spec.Version, instance.Spec.Group))

	podList := &corev1.PodList{}
	replicaSetList := &appsv1.ReplicaSetList{}
	deploymentList := &appsv1.DeploymentList{}
	statefulsetList := &appsv1.StatefulSetList{}
	daemonsetList := &appsv1.DaemonSetList{}
	podLookupStrings := []string{gvkStr}
	serviceMonitors := &monitoringv1.ServiceMonitorList{}

	result, _ := cc.Do(ctx,
		HandleResult(
			ListAction(deploymentList, client.MatchingFields{rhmclient.OwnerRefContains: gvkStr}),
			OnContinue(Call(func() (ClientAction, error) {
				actions := []ClientAction{}

				for _, depl := range deploymentList.Items {
					actions = append(actions, ListAppendAction(replicaSetList, client.MatchingField(rhmclient.OwnerRefContains, string(depl.UID))))
				}
				return Do(actions...), nil
			})),
		),
		HandleResult(
			Do(
				ListAppendAction(replicaSetList, client.MatchingField(rhmclient.OwnerRefContains, gvkStr)),
				ListAction(statefulsetList, client.MatchingField(rhmclient.OwnerRefContains, gvkStr)),
				ListAction(daemonsetList, client.MatchingField(rhmclient.OwnerRefContains, gvkStr)),
				ListAction(serviceMonitors, client.MatchingField(rhmclient.OwnerRefContains, gvkStr)),
			),
			OnContinue(Call(func() (ClientAction, error) {
				for _, rs := range replicaSetList.Items {
					podLookupStrings = append(podLookupStrings, string(rs.UID))
				}

				for _, item := range statefulsetList.Items {
					podLookupStrings = append(podLookupStrings, string(item.UID))
				}

				for _, item := range daemonsetList.Items {
					podLookupStrings = append(podLookupStrings, string(item.UID))
				}

				actions := []ClientAction{}

				for _, lookup := range podLookupStrings {
					actions = append(actions, ListAppendAction(podList, client.MatchingFields{rhmclient.OwnerRefContains: lookup}))
				}

				return Do(actions...), nil
			}))),
	)

	if result.Is(Error) {
		log.Error(result, "failed to get lists")
		return []corev1.Pod{}, result.Err
	}

	return podList.Items, nil
}

func (c *Collector) processServices() {

}

func (c *Collector) collectServices(instance *marketplacev1alpha1.MeterDefinition) error {
	return nil
}
