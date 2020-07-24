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
	"errors"
	"strconv"
	"time"

	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	kbsm "k8s.io/kube-state-metrics/pkg/metric"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const ()

var (
	existingPods     = make(map[string]bool)
	existingServices = make(map[string]bool)
)

// NOTE: FamilyGenerator provides everything needed to generate a metric family with a Kubernetes object.
// A family object (in kube-state-metrics) represents a set of metrics with the same: name, type, and help text.

// buildPodMetric creates metrics for pods using both the rhm-operator(meterdef) and the kube-state-metrics labels
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

//buildServiceMetric creates metrics for services using both the rhm-operator(meterdef) and the kube-state-metrics labels
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

// findAndGenerateDefPods() find and generates pods associated with MeterDefinition
// Gets a list of MeterDefinitions -> Gets pod labels -> Gets pods -> Generates pod metrics
func findAndGenerateDefPods(rclient client.Client) error {
	log := logf.Log.WithName("metric_generator")
	var err error
	meterDefList := &marketplacev1alpha1.MeterDefinitionList{}

	// Get a list of meterdefinitions
	log.Info("retrieving a list of meterdefinitions - for pods")
	err = rclient.List(context.TODO(), meterDefList)
	if err != nil {
		return err
	}

	if len(meterDefList.Items) == 0 {
		log.Info("No instances of MeterDefinition found")
	}

	// for each meterdefinition, get its podlabels
	for _, meterdef := range meterDefList.Items {
		if meterdef.Spec.PodSelector == nil {
			log.Info("No labels for this meterdefinition", "MeterDefinition Name: ", meterdef.GetName(), "MeterDefinition Namespace: ", meterdef.GetNamespace())
		} else {
			log.Info("Labels found for", "MeterDefinition Name: ", meterdef.GetName(), "MeterDefinition Namespace: ", meterdef.GetNamespace())

			podLabels := &metav1.LabelSelector{}
			podLabels = meterdef.Spec.PodSelector

			listOpts := []client.ListOption{
				client.MatchingLabels(podLabels.MatchLabels),
			}

			//retrieve a list of pods, matching the pod labels
			meterdefPods := &corev1.PodList{}
			err = rclient.List(context.TODO(), meterdefPods, listOpts...)
			if err != nil {
				return err
			}
			if len(meterdefPods.Items) == 0 {
				log.Info("No pods associated with meterdefinition found")
			}

			//for each new pod -> create metric
			//add to map of known pods
			//otherwise nothing
			for _, pod := range meterdefPods.Items {
				log.Info("Pod associated with MeterDef found", "Pod Name:", pod.GetName(), "Pod Namespace: ", pod.GetNamespace())
				mapName, nil := runtimeObjectToString(&pod)
				if err != nil {
					log.Error(err, "Could not retrieve mapName of pod")
				} else {
					if _, ok := existingPods[mapName]; ok {
						log.Info("Pod already being tracked")
					} else {
						log.Info("Pod currently not being tracked")
						buildPodMetric(&pod, &meterdef)
						existingPods[mapName] = true
						log.Info("Metrics for pod, created")
					}
				}
			}
		}
	}
	return nil
}

// findAndGenerateDefServices() find and generates pods associated with MeterDefinition
// Gets a list of MeterDefinitions -> Gets service labels -> Gets services -> Generates service metrics
func findAndGenerateDefServices(rclient client.Client) error {
	log := logf.Log.WithName("metric_generator")
	var err error
	meterDefList := &marketplacev1alpha1.MeterDefinitionList{}

	log.Info("retrieving a list of meterdefinitions - for services")
	// Get a list of meterdefinitions
	err = rclient.List(context.TODO(), meterDefList)
	if err != nil {
		return err
	}

	if len(meterDefList.Items) == 0 {
		log.Info("No instances of MeterDefinition found")
	} // for each meterdefinition, get its podlabels

	for _, meterdef := range meterDefList.Items {

		if meterdef.Spec.ServiceMonitorSelector == nil {
			log.Info("No labels for this meterdefinition", "MeterDefinition Name: ", meterdef.GetName(), "MeterDefinition Namespace: ", meterdef.GetNamespace())
		} else {
			log.Info("Labels found for", "MeterDefinition Name: ", meterdef.GetName(), "MeterDefinition Namespace: ", meterdef.GetNamespace())

			serviceLabels := &metav1.LabelSelector{}
			serviceLabels = meterdef.Spec.ServiceMonitorSelector

			listOpts := []client.ListOption{
				client.MatchingLabels(serviceLabels.MatchLabels),
			}

			//retrieve a list of pods, matching the pod labels
			meterdefServices := &corev1.ServiceList{}
			err = rclient.List(context.TODO(), meterdefServices, listOpts...)
			if err != nil {
				return err
			}

			if len(meterdefServices.Items) == 0 {
				log.Info("No services associated with meterdefinition found")
			}
			//for each new pod -> create metric
			//add to map of known pods
			//otherwise nothing
			for _, service := range meterdefServices.Items {
				log.Info("Service associated with MeterDef found", "Service Name:", service.GetName(), "Service Namespace: ", service.GetNamespace())
				mapName, nil := runtimeObjectToString(&service)
				if err != nil {
					log.Error(err, "Could not retrieve map name of service")
				} else {
					if _, ok := existingServices[mapName]; ok {
						log.Info("Service already being tracked")
					} else {
						log.Info("Service currently not being tracked")
						buildServiceMetric(&service, &meterdef)
						existingServices[mapName] = true
						log.Info("Metrics for service, created")
					}
				}
			}
		}
	}
	return nil
}

func runtimeObjectToString(r runtime.Object) (string, error) {
	switch v := r.(type) {
	case *corev1.Pod:
		return "pod:" + v.GetName() + ":" + v.GetNamespace(), nil
	case *corev1.Service:
		return "service:" + v.GetName() + ":" + v.GetNamespace(), nil
	default:
		return "", errors.New("Passed an expected type")
	}
}

//CycleMeterDefMeters() cylces through the process of tracking and gnerating metrics for pods&services associated with MeterDefinition
func CycleMeterDefMeters(rclient client.Client) {

	go func(rclient client.Client) {
		log := logf.Log.WithName("metric_generator")
		var err error
		for {
			log.Info("--Cycling function: CycleMeterDefMeters()--")
			log.Info("Pods - Retrieving MeterDefintion Info")
			err = findAndGenerateDefPods(rclient)
			if err != nil {
				log.Error(err, "Failed to generate metrics for pods associated with MeterDefinition")
			}
			log.Info("Services - Retrieving MeterDefinition Info")
			err = findAndGenerateDefServices(rclient)
			if err != nil {
				log.Error(err, "Failed to generate metrics for services associated with MeterDefinition")
			}

			time.Sleep(time.Time * 5)
		}
	}(rclient)
}
