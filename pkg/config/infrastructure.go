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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
)

type KubernetesInfra struct {
	version,
	platform string
}

type OpenshiftInfra struct {
	version string
}

// Infrastructure stores Kubernetes/Openshift clients
type Infrastructure struct {
	Openshift  *OpenshiftInfra
	Kubernetes *KubernetesInfra
}

func openshiftInfrastructure(c client.Client) (*OpenshiftInfra, error) {
	ch := make(chan struct {
		version *openshiftconfigv1.ClusterVersionStatus
		error
	})
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

	go func(log logr.Logger, ch chan struct {
		version *openshiftconfigv1.ClusterVersionStatus
		error
	}) {
		clusterVersion := openshiftconfigv1.ClusterVersionStatus{}
		err := c.Get(ctx, versionNamespacedName, clusterVersionObj)
		if err != nil {
			log.Error(err, "Unable to get Openshift info")
			ch <- struct {
				version *openshiftconfigv1.ClusterVersionStatus
				error
			}{nil, err}
			return
		}
		marshaledStatus, err := json.Marshal(clusterVersionObj.Object["status"])
		if err != nil {
			log.Error(err, "Error marshaling openshift api response")
			ch <- struct {
				version *openshiftconfigv1.ClusterVersionStatus
				error
			}{nil, err}
			return
		}
		if err := json.Unmarshal(marshaledStatus, &clusterVersion); err != nil {
			log.Error(err, "Error unmarshaling openshift api response")
			ch <- struct {
				version *openshiftconfigv1.ClusterVersionStatus
				error
			}{nil, err}
			return
		}
		ch <- struct {
			version *openshiftconfigv1.ClusterVersionStatus
			error
		}{&clusterVersion, err}
	}(log, ch)

	cv := <-ch
	if cv.error != nil {
		return nil, nil
	}

	return &OpenshiftInfra{
		version: cv.version.Desired.Version,
	}, nil
}
func kubernetesInfrastructure(discoveryClient *discovery.DiscoveryClient) (kInf *KubernetesInfra, err error) {
	serverVersion, err := discoveryClient.ServerVersion()
	if err == nil {
		kInf = &KubernetesInfra{
			version:  serverVersion.GitVersion,
			platform: serverVersion.Platform,
		}
	}
	return
}

func LoadInfrastructure(c client.Client, dc *discovery.DiscoveryClient) (*Infrastructure, error) {
	openshift, err := openshiftInfrastructure(c)
	if err != nil {
		// check if api exists on the cluster
		if !errors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			log.Error(err, "unable to get Openshift version")
		} else {
			return nil, err
		}
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

func (k KubernetesInfra) Version() string {
	return k.version
}

func (k KubernetesInfra) Platform() string {
	return k.platform
}

func (o OpenshiftInfra) Version() string {
	return o.version
}

func (inf Infrastructure) HasOpenshift() bool {
	return inf.Openshift != nil
}
