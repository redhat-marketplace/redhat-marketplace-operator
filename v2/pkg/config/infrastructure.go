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
package config

import (
	"context"
	"encoding/json"
	"time"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KubernetesInfra stores Kubernetes information
type KubernetesInfra struct {
	Version,
	Platform string
}

// OpenshiftInfra stores Openshift information (if Openshift is present)
type OpenshiftInfra struct {
	Version string
}

// Infrastructure stores Kubernetes/Openshift clients
type Infrastructure struct {
	Openshift  *OpenshiftInfra
	Kubernetes *KubernetesInfra
}

func openshiftInfrastructure(c client.Client) (*OpenshiftInfra, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3200*time.Millisecond)
	defer cancel()
	clusterVersionObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "config.openshift.io/v1",
			"kind":       "ClusterVersion",
			"metadata": map[string]interface{}{
				"name": "version",
			},
			"spec": "console",
		},
	}
	versionNamespacedName := client.ObjectKey{
		Name: "version",
	}
	clusterVersion := openshiftconfigv1.ClusterVersionStatus{}
	err := c.Get(ctx, versionNamespacedName, clusterVersionObj)
	if err != nil {
		log.Error(err, "Unable to get Openshift info")
		return nil, err
	}
	marshaledStatus, err := json.Marshal(clusterVersionObj.Object["status"])
	if err != nil {
		log.Error(err, "Error marshaling openshift api response")
		return nil, err
	}
	if err := json.Unmarshal(marshaledStatus, &clusterVersion); err != nil {
		log.Error(err, "Error unmarshaling openshift api response")
		return nil, err
	}

	return &OpenshiftInfra{
		Version: clusterVersion.Desired.Version,
	}, nil
}
func kubernetesInfrastructure(discoveryClient *discovery.DiscoveryClient) (kInf *KubernetesInfra, err error) {
	serverVersion, err := discoveryClient.ServerVersion()
	if err == nil {
		kInf = &KubernetesInfra{
			Version:  serverVersion.GitVersion,
			Platform: serverVersion.Platform,
		}
	}
	return
}

// LoadInfrastructure loads Kubernetes and Openshift information
func LoadInfrastructure(c client.Client, dc *discovery.DiscoveryClient) (*Infrastructure, error) {
	openshift, err := openshiftInfrastructure(c)
	if err != nil {
		// Openshift is not mandatory
		log.Error(err, "unable to get Openshift version")
	}
	kuberentes, err := kubernetesInfrastructure(dc)
	if err != nil {
		return nil, err
	}

	return &Infrastructure{
		Openshift:  openshift,
		Kubernetes: kuberentes,
	}, nil
}

// Version gets Kubernetes Git version
func (i *Infrastructure) KubernetesVersion() string {
	if i.Kubernetes == nil {
		return ""
	}
	return i.Kubernetes.Version
}

// Platform returns platform information
func (i *Infrastructure) KubernetesPlatform() string {
	if i.Kubernetes == nil {
		return ""
	}

	return i.Kubernetes.Platform
}

// Version gets Openshift version√
func (i *Infrastructure) OpenshiftVersion() string {
	if i.Openshift == nil {
		return ""
	}

	return i.Openshift.Version
}

// HasOpenshift checks if Openshift is available
func (i *Infrastructure) HasOpenshift() bool {
	return i.Openshift != nil
}

// IsDefined tells you if the infrastructure has been created
func (i *Infrastructure) IsDefined() bool {
	return i.Openshift != nil || i.Kubernetes != nil
}
