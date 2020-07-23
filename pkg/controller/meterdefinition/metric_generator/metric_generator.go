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

package metric_generator

import (
	"context"
	"strconv"
	"time"

	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kbsm "k8s.io/kube-state-metrics/pkg/metric"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const ()

var ()

// NOTE: FamilyGenerator provides everything needed to generate a metric family with a Kubernetes object.
// A family object (in kube-state-metrics) represents a set of metrics with the same: name, type, and help text.

// buildPodMetric creates the metrics for pod using both rhm-operator(meterdef) and kube-state-metrics labels
func buildPodMetric(p *corev1.Pod, def *marketplacev1alpha1.MeterDefinition) []kbsm.FamilyGenerator {

	// rhm-operator labels
	metered := "true"
	meterDefKind := def.Spec.MeterKind
	meterDefVersion := def.Spec.MeterVersion
	meterDefDomain := def.Spec.MeterDomain

	// metric specific labels
	createdBy := metav1.GetControllerOf(p)
	createdByKind := "<none>"
	createdByName := "<none>"
	if createdBy != nil {
		if createdBy.Kind != "" {
			createdByKind = createdBy.Kind
		}
		if createdBy.Name != "" {
			createdByName = createdBy.Name
		}
	}
	hostIP := p.Status.HostIP
	podIP := p.Status.PodIP
	podUID := string(p.UID)
	node := p.Spec.NodeName
	priorityClass := p.Spec.PriorityClassName
	hostNetwork := strconv.FormatBool(p.Spec.HostNetwork)

	podMetricsFamilies := []kbsm.FamilyGenerator{
		{
			Name: "meterdef_pod_info",
			Type: kbsm.Gauge,
			Help: "Information about the pod",
			GenerateFunc: wrapPodFunc(func(p *corev1.Pod) *kbsm.Family {

				m := kbsm.Metric{
					LabelKeys:   []string{"host_ip", "pod_ip", "pod_uid", "node", "metered", "meter_def_kind", "meter_def_version", "meter_def_domain", "created_by_name", "created_by_kind", "priority_class", "host_network"},
					LabelValues: []string{hostIP, podIP, podUID, node, metered, meterDefKind, meterDefVersion, meterDefDomain, createdByKind, createdByName, priorityClass, hostNetwork},
					Value:       1,
				}

				return &kbsm.Family{
					Metrics: []*kbsm.Metric{&m},
				}
			}),
		},
	}
	return podMetricsFamilies
}

//
func buildServiceMetric(s *corev1.Service, def *marketplacev1alpha1.MeterDefinition) []kbsm.FamilyGenerator {

	// rhm-operator labels
	metered := "true"
	deployed := "true"
	meterDefKind := def.Spec.MeterKind
	meterDefVersion := def.Spec.MeterVersion
	meterDefDomain := def.Spec.MeterDomain

	// kube-state-metric labels
	clusterIP := s.Spec.ClusterIP
	externalName := s.Spec.ExternalName
	loadBalancerIP := s.Spec.LoadBalancerIP

	serviceMetricsFamilies := []kbsm.FamilyGenerator{
		{
			Name: "meterdef_service_info",
			Type: kbsm.Gauge,
			Help: "Info about the service for servicemonitor",
			GenerateFunc: wrapServiceFunc(func(s *corev1.Service) *kbsm.Family {
				m := kbsm.Metric{
					LabelKeys:   []string{"metered", "deployed", "meter_def_kind", "meter_def_version", "meter_def_domain", "cluster_ip", "external_name", "load_balancer_ip"},
					LabelValues: []string{metered, deployed, meterDefKind, meterDefVersion, meterDefDomain, clusterIP, externalName, loadBalancerIP},
				}
				return &kbsm.Family{
					Metrics: []*kbsm.Metric{&m},
				}
			}),
		},
	}

	return serviceMetricsFamilies
}

// findMeterDefPods() returns a PodList of pods, associated with MeterDefinition
func findMeterDefPods(rclient client.Client) (*corev1.PodList, error) {
	var err error
	meterDefPods := &corev1.PodList{}

	// What we want to do is: get list of MeterDefinitions -> Get pod labels -> get pods -> generate those metrics

	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			"marketplace.redhat.com/metered.kind": "Pod",
		}),
	}

	err = rclient.List(context.TODO(), meterDefPods, listOpts...)
	if err != nil {
		return nil, err
	}

	return meterDefPods, nil

}

// findMeterDefServices() returns a ServiceList of Services, associated with MeterDefinition
func findMeterDefServices(rclient client.Client) (*corev1.ServiceList, error) {
	var err error
	meterDefServices := &corev1.ServiceList{}

	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			"marketplace.redhat.com/metered.kind": "Service",
		}),
	}

	err = rclient.List(context.TODO(), meterDefServices, listOpts...)
	if err != nil {
		return nil, err
	}

	return meterDefServices, nil

}

//generateMetrics() generates metrics for the passed list of pods/services
func generateMetrics(podList *corev1.PodList, serviceList *corev1.ServiceList, def *marketplacev1alpha1.MeterDefinition) {

	for _, pod := range podList.Items {
		buildPodMetric(&pod, def)
	}

	for _, service := range serviceList.Items {
		buildServiceMetric(&service, def)
	}

}

//cycleMeterDefMeters cylces through the process of tracking and gnerating metrics for pods&services associated with MeterDefinition
/*
1. Get a list of pods associated with MeterDef
2. Get a list of services associated with MeterDef
3. Generate metrics for those pods/services
4. Repeat every 5 minutes

TODO:
- filter between existing and new pods/services
- add unit tests
- mgr.Add(runnable) ..
*/
func cycleMeterDefMeters(def *marketplacev1alpha1.MeterDefinition, rclient client.Client) {

	go func(def *marketplacev1alpha1.MeterDefinition, rclient client.Client) {
		log := logf.Log.WithName("controller_meterdefinition")
		reqLogger := log.WithValues("Request.Namespace", def.GetNamespace(), "Request.Name", def.GetName)
		meterDefPods := &corev1.PodList{}
		meterDefServices := &corev1.ServiceList{}
		var err error
		for {
			meterDefPods, err = findMeterDefPods(rclient)
			if err != nil {
				reqLogger.Error(err, "Failed to retrieve pods associated with MeterDefinition")
			}
			meterDefServices, err = findMeterDefServices(rclient)
			if err != nil {
				reqLogger.Error(err, "Failed to retrieve services associated with MeterDefinition")
			}

			generateMetrics(meterDefPods, meterDefServices, def)

			time.Sleep(time.Minute * 5)
		}
	}(def, rclient)

}
