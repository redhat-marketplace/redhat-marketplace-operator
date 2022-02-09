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

package processorsenders

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/mailbox"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/razeeclient"
	"github.com/sasha-s/go-deadlock"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/watch"

	merrors "emperror.dev/errors"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const maxToSend = 50

type RazeeProcessorSender struct {
	*ProcessorSender
	log                logr.Logger
	kubeClient         client.Client
	mutex              deadlock.Mutex
	scheme             *runtime.Scheme
	processedEventObjs ProcessedEventObjs
	pollStarted        string
}

// EventObj is the obj to be marshal and sent
type EventObj struct {
	Type   watch.EventType `json:"type,omitempty"`
	Object runtime.Object  `json:"object,omitempty"`
}

type EventObjs []EventObj

type ProcessedEventObjs struct {
	eventObjs []EventObj
	mu        sync.Mutex
}

func (p *ProcessedEventObjs) Add(eventObj EventObj) int {
	p.mu.Lock()
	p.eventObjs = append(p.eventObjs, eventObj)
	length := len(p.eventObjs)
	p.mu.Unlock()
	return length
}

func (p *ProcessedEventObjs) Flush() []EventObj {
	p.mu.Lock()
	flushedProcessedEventObjs := p.eventObjs
	p.eventObjs = nil
	p.mu.Unlock()
	return flushedProcessedEventObjs
}

func (p *ProcessedEventObjs) IsEmpty() bool {
	p.mu.Lock()
	length := len(p.eventObjs)
	p.mu.Unlock()

	if length != 0 {
		return false
	}
	return true
}

// NewPrometheusProcessor is the provider that creates
// the processor.
func ProvideRazeeProcessorSender(
	log logr.Logger,
	kubeClient client.Client,
	mb *mailbox.Mailbox,
	scheme *runtime.Scheme,
) *RazeeProcessorSender {
	sp := &RazeeProcessorSender{
		ProcessorSender: &ProcessorSender{
			log:           log,
			digestersSize: 1,
			retryCount:    3,
			mailbox:       mb,
			channelName:   mailbox.RazeeChannel,
		},
		log:        log.WithValues("process", "razeeProcessor"),
		kubeClient: kubeClient,
		scheme:     scheme,
	}

	sp.ProcessorSender.DeltaProcessorSender = sp
	return sp
}

// Process will receive a new ObjectResourceMessage
// match the cache type to the event type
// sanitize the object
// bundle the event type and object and prepare
func (r *RazeeProcessorSender) Process(ctx context.Context, inObj cache.Delta) error {
	if inObj.Object == nil {
		return nil
	}

	// cache.Delta.Object does not retain GVK
	// Use scheme to determine GVK and return a runtime.Object
	rtObj, err := r.getRuntimeObj(inObj.Object)
	if err != nil {
		return err
	}

	rtObjCopy := rtObj.DeepCopyObject()

	// Skip filtered out objects
	filterOut, err := r.filterOut(rtObjCopy)
	if err != nil {
		return err
	}
	if filterOut {
		return nil
	}

	// Sanitize the object
	r.prepObject2Send(rtObjCopy)

	// Map the cache type to the watch/event type
	var eventType watch.EventType
	switch inObj.Type {
	case cache.Deleted:
		eventType = watch.Deleted
	case cache.Replaced:
		eventType = watch.Modified
	case cache.Sync:
		eventType = watch.Modified
	case cache.Updated:
		eventType = watch.Modified
	case cache.Added:
		eventType = watch.Added
	default:
		return nil
	}

	// Track start of event batch for poll-cycle header
	if r.processedEventObjs.IsEmpty() {
		r.pollStarted = time.Now().Format(time.RFC3339)
	}

	// Build the eventObj, as per Razee
	numEventObjs := r.processedEventObjs.Add(EventObj{Type: eventType, Object: rtObjCopy})

	if numEventObjs >= maxToSend {
		r.ProcessorSender.sendReadyChan <- true
	}

	return nil
}

func (r *RazeeProcessorSender) Send(ctx context.Context) error {
	if !r.processedEventObjs.IsEmpty() {

		clusterID, err := razeeclient.GetClusterID(r.kubeClient)
		if err != nil {
			return err
		}

		//baseurl, razeeOrgKey, err := razeeclient.GetRazeeDashKeys(r.kubeClient, razeeclient.GetNamespace())
		baseurl, razeeOrgKey, err := razeeclient.GetRazeeDashKeysFromFile()
		if err != nil {
			return err
		}

		// build full razeedash url
		fullurl, err := r.getRazeeDashURL(string(baseurl), string(clusterID))
		if err != nil {
			return err
		}

		// Marshal to send
		b, err := json.Marshal(r.processedEventObjs.Flush())
		if err != nil {
			return err
		}

		header := make(http.Header)
		header.Set("razee-org-key", string(razeeOrgKey))
		header.Set("Content-Type", "application/json")
		header.Set("poll-cycle", r.pollStarted)

		// Post
		err = razeeclient.PostToRazeeDash(fullurl, bytes.NewBuffer(b), header)
		if err != nil {
			return err
		}

		r.log.Info("Sent Objects to Destination", "URL", fullurl)

	}
	return nil
}

// Set the GVK on the object from the scheme/watched types
func (r *RazeeProcessorSender) getRuntimeObj(inObj interface{}) (runtime.Object, error) {

	rtObj, ok := inObj.(runtime.Object)
	if !ok {
		return nil, merrors.New("Could not convert cache delta object to runtime object")
	}

	gvks, _, err := r.scheme.ObjectKinds(rtObj)
	if err != nil {
		return nil, err
	}

	rtObj.GetObjectKind().SetGroupVersionKind(gvks[0])

	return rtObj, nil
}

// Filter out criteria
func (r *RazeeProcessorSender) filterOut(obj interface{}) (bool, error) {
	// Only Process CSVs with a Subscription containing
	if clusterserviceversion, ok := obj.(*olmv1alpha1.ClusterServiceVersion); ok {
		subscriptionList := &olmv1alpha1.SubscriptionList{}

		if err := r.kubeClient.List(context.TODO(), subscriptionList, client.InNamespace(clusterserviceversion.ObjectMeta.Namespace)); err != nil {
			return false, err
		}

		isTagged := false
		if len(subscriptionList.Items) > 0 {
			for _, subscription := range subscriptionList.Items {
				if value, ok := subscription.GetLabels()[utils.LicenseServerTag]; ok {
					if value == "true" {
						if subscription.Status.InstalledCSV == clusterserviceversion.ObjectMeta.Name {
							return false, nil // CSV is tagged, do not filterOut
						}
					}
				}
			}
		}
		return !isTagged, nil //CSV is not tagged, filterOut
	}

	return false, nil
}

// Sanitize the objects to send
func (r *RazeeProcessorSender) prepObject2Send(obj interface{}) {

	metaobj, err := meta.Accessor(obj)
	if err == nil {
		annotations := metaobj.GetAnnotations()
		delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
		delete(annotations, "kapitan.razee.io/last-applied-configuration")
		delete(annotations, "deploy.razee.io/last-applied-configuration")
		metaobj.SetAnnotations(annotations)
	}

	// Nodes
	if node, ok := obj.(*corev1.Node); ok {
		node.Status.Images = []corev1.ContainerImage{}
	}

	// Deployments
	if deployment, ok := obj.(*appsv1.Deployment); ok {
		for i, _ := range deployment.Spec.Template.Spec.Containers {
			deployment.Spec.Template.Spec.Containers[i].Env = []corev1.EnvVar{}
		}
	}
}

func (r *RazeeProcessorSender) getRazeeDashURL(baseurl string, clusterID string) (string, error) {
	var urlStr string

	urlStr = strings.Join([]string{baseurl, "clusters", clusterID, "resources"}, "/")
	url, err := url.Parse(urlStr)
	if err != nil {
		return urlStr, err
	}

	return url.String(), nil
}
