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
	"crypto/tls"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/mailbox"
	"github.com/sasha-s/go-deadlock"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/watch"

	merrors "emperror.dev/errors"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const maxToSend = 50

type RazeeProcessorSender struct {
	*ProcessorSender
	log                logr.Logger
	kubeClient         client.Client
	mutex              deadlock.Mutex
	scheme             *runtime.Scheme
	processedEventObjs ProcessedEventObjs
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

	r.log.Info("dac debug startProcess")

	if inObj.Object == nil {
		return nil
	}

	r.log.Info("dac debug", "inObj", inObj)

	metaObj, ok := inObj.Object.(metav1.Object)
	if !ok {
		merrors.New("Could not convert cache delta object to runtime object")
	} else {
		r.log.Info("dac debug", "type", inObj.Type, "name", metaObj.GetName(), "namespace", metaObj.GetNamespace())
	}

	// cache.Delta.Object does not retain GVK
	// Use scheme to determine GVK and return a runtime.Object
	rtObj, err := r.getRuntimeObj(inObj.Object)
	if err != nil {
		return err
	}

	/*
		rtObj, ok := inObj.Object.(runtime.Object)
		if !ok {
			return merrors.New("Could not convert cache delta object to runtime object")
		}
	*/

	rtObjCopy := rtObj.DeepCopyObject()

	// Skip filtered out objects
	r.log.Info("dac debug filter out")
	filterOut, err := r.filterOut(rtObjCopy)
	if err != nil {
		return err
	}
	if filterOut {
		r.log.Info("dac debug filterOut")
		return nil
	}

	// Sanitize the object
	r.log.Info("dac debug Process Sanitize")
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

	// Build the eventObj, as per Razee
	numEventObjs := r.processedEventObjs.Add(EventObj{Type: eventType, Object: rtObjCopy})

	r.log.Info("dac debug", "numEventObjs", numEventObjs)

	if numEventObjs >= maxToSend {
		r.log.Info("dac debug sendReadyChan")
		r.ProcessorSender.sendReadyChan <- true
		r.log.Info("dac debug sendReadyChan sent")
	}

	r.log.Info("dac debug razeeprocessorsender done")

	return nil
}

func (r *RazeeProcessorSender) Send(ctx context.Context) error {
	if !r.processedEventObjs.IsEmpty() {

		r.log.Info("dac debug get CSV")
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

		r.log.Info("dac debug getRazeeDashKeys")
		// read razeedash url secret & org secret for header
		baseurl, razeeOrgKey, err := r.getRazeeDashKeys()
		if err != nil {
			return err
		}

		r.log.Info("dac debug getRazeeDashURL")
		// build full razeedash url
		fullurl, err := r.getRazeeDashURL(string(baseurl), string(clusterID))
		if err != nil {
			return err
		}

		r.log.Info("dac debug marshal")
		// Marshal to send
		b, err := json.Marshal(r.processedEventObjs.Flush())
		if err != nil {
			return err
		}

		r.log.Info("dac debug", "marshal", string(b))

		// Post
		r.log.Info("Attempt to send Objects to Destination", "URL", fullurl)

		err = r.postToRazeeDash(fullurl, bytes.NewReader(b), string(razeeOrgKey))
		if err != nil {
			return err
		}

	}
	return nil
}

// Set the GVK on the object from the scheme/watched types
func (r *RazeeProcessorSender) getRuntimeObj(inObj interface{}) (runtime.Object, error) {

	rtObj, ok := inObj.(runtime.Object)
	if !ok {
		r.log.Info("dac debug", "badconvert", inObj)
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

	/*
		// Handled in Watcher
		// Only Process RRS3 Deployment
		if deployment, ok := obj.(*appsv1.Deployment); ok {
			if deployment.ObjectMeta.Name != utils.RHM_REMOTE_RESOURCE_S3_DEPLOYMENT_NAME {
				return true, nil
			}
		}
	*/

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
	} else {
		r.log.Info("dac debug could not get a meta accessor for", "obj", obj)
	}

	// Nodes
	if node, ok := obj.(*corev1.Node); ok {
		r.log.Info("dac debug sanitize node")
		node.Status.Images = []corev1.ContainerImage{}
	}

	// Deployments
	if deployment, ok := obj.(*appsv1.Deployment); ok {
		r.log.Info("dac debug sanitize deployment")
		for i, _ := range deployment.Spec.Template.Spec.Containers {
			deployment.Spec.Template.Spec.Containers[i].Env = []corev1.EnvVar{}
		}
	}

}

// Get the RazeeDash URL & Org Key from the rhm-operator-secret
func (r *RazeeProcessorSender) getRazeeDashKeys() ([]byte, []byte, error) {
	var url []byte
	var key []byte

	r.log.Info("dac debug", "namespace", r.getNamespace())

	rhmOperatorSecret := corev1.Secret{}
	err := r.kubeClient.Get(context.TODO(), types.NamespacedName{
		Name:      utils.RHM_OPERATOR_SECRET_NAME,
		Namespace: r.getNamespace(),
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
		return url, key, err
	}

	key, err = utils.ExtractCredKey(&rhmOperatorSecret, corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: utils.RHM_OPERATOR_SECRET_NAME,
		},
		Key: utils.RAZEE_DASH_ORG_KEY_FIELD,
	})
	if err != nil {
		return url, key, err
	}

	return url, key, nil
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

func (r *RazeeProcessorSender) postToRazeeDash(url string, body io.Reader, razeeOrgKey string) error {

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

	if os.Getenv("INSECURE_CLIENT") == "true" {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client.HTTPClient.Transport = tr
	}

	_, err = client.Do(req)
	if err != nil {
		return merrors.Wrap(err, "Error POSTing to RazeeDash")
	}

	r.log.Info("Sent Objects to Destination", "URL", url)

	return nil
}

func (r *RazeeProcessorSender) getNamespace() string {
	if ns, ok := os.LookupEnv("POD_NAMESPACE"); ok {
		return ns
	}

	// Fall back to the namespace associated with the service account token, if available
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}

	return "default"
}
