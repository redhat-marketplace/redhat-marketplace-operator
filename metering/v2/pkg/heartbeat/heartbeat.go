// Copyright 2021 IBM Corp.
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

package heartbeat

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/razeeclient"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Heartbeat struct {
	kubeClient      client.Client
	discoveryClient *discovery.DiscoveryClient
	log             logr.Logger

	mainContext  *context.Context
	localContext *context.Context
	cancelFunc   context.CancelFunc
}

func ProvideHeartbeat(
	log logr.Logger,
	kubeClient client.Client,
	discoveryClient *discovery.DiscoveryClient,
) *Heartbeat {
	return &Heartbeat{
		log:             log,
		kubeClient:      kubeClient,
		discoveryClient: discoveryClient,
	}
}

func (h *Heartbeat) Start(ctx context.Context) error {

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				err := h.heartbeat()
				if err != nil {
					h.log.Error(err, "Error sending heartbeat")
				}
			}
		}
	}()

	return nil
}

func (h *Heartbeat) heartbeat() error {

	clusterID, err := razeeclient.GetClusterID(h.kubeClient)
	if err != nil {
		return err
	}

	baseurl, razeeOrgKey, err := razeeclient.GetRazeeDashKeys(h.kubeClient, razeeclient.GetNamespace())
	if err != nil {
		return err
	}

	fullurl, err := h.getRazeeDashURL(string(baseurl), clusterID)
	if err != nil {
		return err
	}

	body, err := h.getBody()
	if err != nil {
		return err
	}

	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	err = razeeclient.PostToRazeeDash(fullurl, bytes.NewReader(b), string(razeeOrgKey))
	if err != nil {
		return err
	}

	h.log.Info("Sent heartbeat to", "URL", fullurl)

	return nil

}

type Body struct {
	Name        openshiftconfigv1.ClusterID `json:"name"`
	Hostname    string                      `json:"hostname"`
	PID         int                         `json:"pid"`
	Level       int                         `json:"level"`
	KubeVersion *version.Info               `json:"kube_version"`
	Message     string                      `json:"msg"`
	Time        string                      `json:"time"`
	Verbosity   int                         `json:"v"`
}

func (h *Heartbeat) getBody() (*Body, error) {

	instance := &openshiftconfigv1.ClusterVersion{}
	err := h.kubeClient.Get(context.TODO(), types.NamespacedName{Name: "version"}, instance)
	if err != nil {
		return nil, err
	}

	kubeVersion, err := h.discoveryClient.ServerVersion()
	if err != nil {
		return nil, err
	}

	return &Body{
		Name:        instance.Spec.ClusterID,
		Hostname:    os.Getenv("POD_NAME"),
		PID:         os.Getpid(),
		Level:       30,
		KubeVersion: kubeVersion,
		Message:     "",
		Time:        time.Now().UTC().Format(time.RFC3339),
		Verbosity:   0,
	}, nil
}

func (h *Heartbeat) getRazeeDashURL(baseurl string, clusterID string) (string, error) {
	var urlStr string

	urlStr = strings.Join([]string{baseurl, "clusters", clusterID}, "/")
	url, err := url.Parse(urlStr)
	if err != nil {
		return urlStr, err
	}

	return url.String(), nil
}
