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
	"sync"

	"github.com/blang/semver"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
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
	Version       string
	ParsedVersion semver.Version
}

// Infrastructure stores Kubernetes/Openshift clients
type Infrastructure struct {
	sync.Mutex
	openshift  *OpenshiftInfra
	kubernetes *KubernetesInfra
}

func NewInfrastructure(
	c client.Client,
	dc *discovery.DiscoveryClient,
) (*Infrastructure, error) {
	openshift, err := openshiftInfrastructure(c)
	if err != nil {
		log.Error(err, "unable to get Openshift version")
		return nil, err
	}

	kubernetes, err := kubernetesInfrastructure(dc)
	if err != nil {
		log.Error(err, "unable to get kubernetes version")
		return nil, err
	}

	return &Infrastructure{
		openshift:  openshift,
		kubernetes: kubernetes,
	}, nil
}

func openshiftInfrastructure(c client.Client) (*OpenshiftInfra, error) {
	clusterVersionObj := &openshiftconfigv1.ClusterVersion{}
	versionNamespacedName := client.ObjectKey{
		Name: "version",
	}

	err := c.Get(context.TODO(), versionNamespacedName, clusterVersionObj)
	if err != nil {
		if k8serrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			log.Error(err, "cluster is not an openshift cluster")
			return nil, nil
		}
		log.Error(err, "Unable to get Openshift info")
		return nil, err
	}

	parsedVersion, err := semver.ParseTolerant(clusterVersionObj.Status.Desired.Version)
	if err != nil {
		log.Error(err, "Unable to parse Openshift version")
		return nil, err
	}

	return &OpenshiftInfra{
		Version:       clusterVersionObj.Status.Desired.Version,
		ParsedVersion: parsedVersion,
	}, nil
}

func kubernetesInfrastructure(discoveryClient *discovery.DiscoveryClient) (*KubernetesInfra, error) {
	serverVersion, err := discoveryClient.ServerVersion()
	if err != nil {
		return nil, err
	}

	return &KubernetesInfra{
		Version:  serverVersion.String(),
		Platform: serverVersion.Platform,
	}, nil
}

// Version gets Kubernetes Git version
func (i *Infrastructure) KubernetesVersion() string {
	i.Lock()
	defer i.Unlock()

	if i.kubernetes == nil {
		return ""
	}
	return i.kubernetes.Version
}

// Platform returns platform information
func (i *Infrastructure) KubernetesPlatform() string {
	i.Lock()
	defer i.Unlock()

	if i.kubernetes == nil {
		return ""
	}

	return i.kubernetes.Platform
}

// Version gets Openshift version√
func (i *Infrastructure) OpenshiftVersion() string {
	i.Lock()
	defer i.Unlock()

	if i.openshift == nil {
		return ""
	}

	return i.openshift.Version
}

// Version gets Openshift parsed version
func (i *Infrastructure) OpenshiftParsedVersion() *semver.Version {
	i.Lock()
	defer i.Unlock()

	if i.openshift == nil {
		return nil
	}

	return &i.openshift.ParsedVersion
}

// HasOpenshift checks if Openshift is available
func (i *Infrastructure) HasOpenshift() bool {
	i.Lock()
	defer i.Unlock()
	return i.openshift != nil
}

// IsDefined tells you if the infrastructure has been created
func (i *Infrastructure) IsDefined() bool {
	i.Lock()
	defer i.Unlock()
	return i.openshift != nil || i.kubernetes != nil
}
