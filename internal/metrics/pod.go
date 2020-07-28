package metrics

import (
	"context"
	"strings"

	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/client"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kbsm "k8s.io/kube-state-metrics/pkg/metric"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	descPodLabelsDefaultLabels = []string{"namespace", "pod"}
)

var podMetricsFamilies = []FamilyGenerator{
	{
		FamilyGenerator: kbsm.FamilyGenerator{
			Name: "meterdef_pod_info",
			Type: kbsm.Gauge,
			Help: "Metering info for pod",
		},
		GenerateMeterFunc: wrapPodFunc(func(pod *corev1.Pod, meterDefinitions []*marketplacev1alpha1.MeterDefinition) *kbsm.Family {
			metrics := []*kbsm.Metric{}

			podUID := string(pod.UID)
			priorityClass := pod.Spec.PriorityClassName

			metrics = append(metrics, &kbsm.Metric{
				LabelKeys:   []string{"pod_uid", "priority_class"},
				LabelValues: []string{podUID, priorityClass},
				Value:       1,
			})

			return &kbsm.Family{
				Metrics: metrics,
			}
		}),
	},
}

type FamilyGenerator struct {
	GenerateMeterFunc func(interface{}, []*marketplacev1alpha1.MeterDefinition) *kbsm.Family
	kbsm.FamilyGenerator
}

func (g *FamilyGenerator) generateHeader() string {
	header := strings.Builder{}
	header.WriteString("# HELP ")
	header.WriteString(g.Name)
	header.WriteByte(' ')
	header.WriteString(g.Help)
	header.WriteByte('\n')
	header.WriteString("# TYPE ")
	header.WriteString(g.Name)
	header.WriteByte(' ')
	header.WriteString(string(g.Type))

	return header.String()
}

// wrapPodFunc is a helper function for generating pod-based metrics
func wrapPodFunc(f func(*v1.Pod, []*marketplacev1alpha1.MeterDefinition) *kbsm.Family) func(obj interface{}, meterDefinitions []*marketplacev1alpha1.MeterDefinition) *kbsm.Family {
	return func(obj interface{}, meterDefinitions []*marketplacev1alpha1.MeterDefinition) *kbsm.Family {
		pod := obj.(*v1.Pod)

		metricFamily := f(pod, meterDefinitions)

		for _, m := range metricFamily.Metrics {
			m.LabelKeys = append(descPodLabelsDefaultLabels, m.LabelKeys...)
			m.LabelValues = append([]string{pod.Namespace, pod.Name}, m.LabelValues...)
		}

		metricFamily.Metrics = MapMeterDefinitions(metricFamily.Metrics, meterDefinitions)

		return metricFamily
	}
}

var (
	replicaSetGVK    = rhmclient.ObjRefToStr("apps/v1", "replicaset")
	deploymentSetGVK = rhmclient.ObjRefToStr("apps/v1", "deployment")
	daemonSetGVK     = rhmclient.ObjRefToStr("apps/v1", "daemonset")
	statefulSetGVK   = rhmclient.ObjRefToStr("apps/v1", "statefulset")
	jobGVK           = rhmclient.ObjRefToStr("batch/v1", "job")
)

type PodMeterDefFetcher struct {
	cc ClientCommandRunner
}

func (p *PodMeterDefFetcher) GetMeterDefinitions(obj interface{}) ([]*marketplacev1alpha1.MeterDefinition, error) {
	cc := p.cc
	findOwner := &findOwnerHelper{p.cc}
	results := []*marketplacev1alpha1.MeterDefinition{}
	pod, ok := obj.(*corev1.Pod)

	if !ok {
		return results, nil
	}

	owner := metav1.GetControllerOf(pod)

	if owner == nil {
		return results, nil
	}

	namespace := pod.GetNamespace()
	var err error
	var serviceDefOwner *metav1.OwnerReference

	for i := 0; i < 10; i++ {
		var lookupObj runtime.Object
		ownerGVK := rhmclient.ObjRefToStr(owner.APIVersion, owner.Kind)

		log.Info("matching", "ownerGVK", ownerGVK, "replicaset", replicaSetGVK)

		switch ownerGVK {
		case replicaSetGVK:
			lookupObj = &appsv1.ReplicaSet{}
		case daemonSetGVK:
			lookupObj = &appsv1.DaemonSet{}
		case statefulSetGVK:
			lookupObj = &appsv1.StatefulSet{}
		case deploymentSetGVK:
			lookupObj = &appsv1.Deployment{}
		case jobGVK:
			lookupObj = &batchv1.Job{}
		default:
			serviceDefOwner = owner
			lookupObj = nil
		}

		if lookupObj == nil {
			break
		}

		owner, err = findOwner.FindOwner(owner.Name, namespace, lookupObj)

		if err != nil {
			return results, err
		}

		if owner == nil {
			return results, nil
		}
	}

	ownerGVK := rhmclient.ObjRefToStr(serviceDefOwner.APIVersion, serviceDefOwner.Kind)

	meterDefinitions := &marketplacev1alpha1.MeterDefinitionList{}
	result, _ := cc.Do(
		context.TODO(),
		ListAction(meterDefinitions, client.MatchingField(rhmclient.MeterDefinitionGVK, ownerGVK)),
	)

	if !result.Is(Continue) {
		if result.Is(Error) {
			log.Error(result, "failed to get owner")
			return results, result
		}
		return results, nil
	}

	for i := 0; i < len(meterDefinitions.Items); i++ {
		results = append(results, &meterDefinitions.Items[i])
	}

	return results, nil
}

type findOwnerHelper struct {
	cc ClientCommandRunner
}

func (f *findOwnerHelper) FindOwner(name, namespace string, lookupObj runtime.Object) (owner *metav1.OwnerReference, err error) {
	result, _ := f.cc.Do(
		context.TODO(),
		GetAction(
			types.NamespacedName{
				Namespace: namespace,
				Name:      name,
			},
			lookupObj,
		))

	if !result.Is(Continue) {
		if result.Is(Error) {
			log.Error(result, "failed to get owner")
		}

		err = result
		return
	}

	o, err := meta.Accessor(lookupObj)
	if err != nil {
		return
	}

	owner = metav1.GetControllerOf(o)
	return
}
