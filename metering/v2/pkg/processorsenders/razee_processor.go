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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/mailbox"
	"github.com/sasha-s/go-deadlock"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/watch"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	merrors "emperror.dev/errors"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const maxToSend = 50

type RazeeProcessor struct {
	*ProcessorSender
	log                logr.Logger
	kubeClient         client.Client
	mutex              deadlock.Mutex
	scheme             *runtime.Scheme
	processedEventObjs ProcessedEventObjs
}

// EventObj is the obj to be marshal and sent
type EventObj struct {
	Type   watch.EventType           `json:"type,omitempty"`
	Object unstructured.Unstructured `json:"object,omitempty"`
}

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
func ProvideRazeeProcessor(
	log logr.Logger,
	kubeClient client.Client,
	mb *mailbox.Mailbox,
	scheme *runtime.Scheme,
) *RazeeProcessor {
	sp := &RazeeProcessor{
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
func (r *RazeeProcessor) Process(ctx context.Context, inObj cache.Delta) error {

	r.log.Info("dac debug startProcess")

	if inObj.Object == nil {
		return nil
	}

	svcobj, ok := inObj.Object.(*corev1.Service)
	if !ok {
		r.log.Info("dac debug NotOkToService")
	}

	unstructuredObj := unstructured.Unstructured{}
	unstructuredContent, err := runtime.DefaultUnstructuredConverter.ToUnstructured(inObj.Object)
	if err != nil {
		return err
	}
	unstructuredObj.SetUnstructuredContent(unstructuredContent)

	/*
		_, ok := inObj.Object.(*corev1.Service)
		if ok {
			unstructuredObj.SetGroupVersionKind
		}
	*/

	r.log.Info("dac debug Process", "EventType", inObj.Type)

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

	r.log.Info("dac debug Process Sanitize")

	// Sanitize Object & append to processed
	r.prepObject2Send(&unstructuredObj)
	numEventObjs := r.processedEventObjs.Add(EventObj{Type: eventType, Object: unstructuredObj})

	// dac debug
	eventTest := EventObj{Type: eventType, Object: unstructuredObj}
	_ = eventTest
	b, err := json.Marshal(svcobj)
	if err != nil {
		return err
	}
	r.log.Info("dac debug", "marshal", string(b))

	b, err = svcobj.Marshal()
	if err != nil {
		return err
	}
	r.log.Info("dac debug", "svcobj marshal", string(b))

	// Accumulator is full, ready to send
	if numEventObjs >= maxToSend {
		r.ProcessorSender.sendReadyChan <- true
	}

	return nil
}

func (r *RazeeProcessor) Send(ctx context.Context) error {

	if !r.processedEventObjs.IsEmpty() {

		// Fetch the Openshift ClusterVersion
		instance := &openshiftconfigv1.ClusterVersion{}
		err := r.kubeClient.Get(context.TODO(), types.NamespacedName{Name: "version"}, instance)
		if err != nil {
			if errors.IsNotFound(err) {
				r.log.Error(err, "ClusterVersion does not exist")
				return err
			}
			r.log.Error(err, "Failed to get ClusterVersion")
			return err
		}

		clusterID := instance.Spec.ClusterID
		r.log.Info(string(clusterID))

		// read razeedash url secret & org secret for header
		baseurl, razeeOrgKey, err := r.getRazeeDashKeys()
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

		_ = fullurl
		_ = b
		_ = razeeOrgKey
		// Post
		/*
			err = r.postToRazeeDash(fullurl, bytes.NewReader(b), string(razeeOrgKey))
			if err != nil {
				return err
			}
		*/
	}
	return nil
}

func (r *RazeeProcessor) prepObject2Send(obj *unstructured.Unstructured) {
	annotations := obj.GetAnnotations()
	delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
	delete(annotations, "kapitan.razee.io/last-applied-configuration")
	delete(annotations, "deploy.razee.io/last-applied-configuration")
	obj.SetAnnotations(annotations)
}

// Get the RazeeDash URL & Org Key from the rhm-operator-secret
func (r *RazeeProcessor) getRazeeDashKeys() ([]byte, []byte, error) {
	var url []byte
	var key []byte

	ns, ok := os.LookupEnv("POD_NAMESPACE")
	if !ok {
		return url, key, merrors.New("Environmental Variable POD_NAMESPACE is not set")
	}

	rhmOperatorSecret := corev1.Secret{}
	err := r.kubeClient.Get(context.TODO(), types.NamespacedName{
		Name:      utils.RHM_OPERATOR_SECRET_NAME,
		Namespace: ns,
	}, &rhmOperatorSecret)
	if err != nil {
		return url, key, err
	}

	url, err = utils.ExtractCredKey(&rhmOperatorSecret, corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: utils.RHM_OPERATOR_SECRET_NAME,
		},
		Key: utils.RAZEE_DASH_URL_FIELD,
	})
	if err != nil {
		return url, key, nil
	}

	key, err = utils.ExtractCredKey(&rhmOperatorSecret, corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: utils.RHM_OPERATOR_SECRET_NAME,
		},
		Key: utils.RAZEE_DASH_ORG_KEY_FIELD,
	})
	if err != nil {
		return url, key, nil
	}

	return url, key, nil
}

func (r *RazeeProcessor) getRazeeDashURL(baseurl string, clusterID string) (string, error) {
	var urlStr string

	urlStr = fmt.Sprintf(baseurl, "/clusters/", clusterID, "/resources")
	url, err := url.Parse(urlStr)
	if err != nil {
		return urlStr, err
	}

	return url.String(), nil
}

func (r *RazeeProcessor) postToRazeeDash(url string, body io.Reader, razeeOrgKey string) error {

	client := retryablehttp.NewClient()
	client.RetryWaitMin = 3000 * time.Millisecond
	client.RetryWaitMax = 5000 * time.Millisecond
	client.RetryMax = 5

	req, err := retryablehttp.NewRequest("POST", url, body)
	if err != nil {
		return merrors.Wrap(err, "Error constructing RazeeDash POST request")
	}

	req.Header.Set("razee-org-key", razeeOrgKey)
	req.Header.Set("Content-Type", "application/json")

	_, err = client.Do(req)
	if err == nil {
		return merrors.Wrap(err, "Error POSTing to RazeeDash")
	}

	return nil
}
