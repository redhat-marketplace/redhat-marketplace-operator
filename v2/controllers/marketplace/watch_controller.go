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

package marketplace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	merrors "emperror.dev/errors"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
)

type WatchReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger

	cfg *config.OperatorConfig
}

func (r *WatchReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	r.cfg = cfg
	return nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *WatchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a new controller

	// This mapFn will queue the default ClusterVersion
	mapFn := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			return []reconcile.Request{
				{NamespacedName: types.NamespacedName{
					Name: "version",
				}},
			}
		})

	return ctrl.NewControllerManagedBy(mgr).
		For(&openshiftconfigv1.ClusterVersion{}).
		Watches(&source.Kind{Type: &corev1.Node{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: mapFn,
			}).
		Complete(r)
}

// +kubebuilder:rbac:groups="config.openshift.io",resources=clusterversions,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
func (r *WatchReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Name", request.Name)
	reqLogger.Info("Reconciling Watch")

	// Fetch the Openshift ClusterVersion
	instance := &openshiftconfigv1.ClusterVersion{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name}, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Error(err, "ClusterVersion does not exist")
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get ClusterVersion")
		return reconcile.Result{}, err
	}

	clusterID := instance.Spec.ClusterID
	reqLogger.Info(string(clusterID))

	// read razeedash url secret & org secret for header
	baseurl, _, result, err := r.getRazeeDashKeys()
	if result.Requeue || result.RequeueAfter != 0 || err != nil {
		return result, nil
	}

	// build full razeedash url
	_, err = r.getRazeeDashURL(string(baseurl), string(clusterID))
	if err != nil {
		return reconcile.Result{}, err
	}

	// get the watched node objects
	nodeList := &corev1.NodeList{}
	err = r.Client.List(context.TODO(), nodeList)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(err, "NodeList does not exist")
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get NodeList")
		return reconcile.Result{}, err
	}

	// format the watched node objects
	r.prepNodeObject2Send(nodeList)

	// send the watched object json in max 50 item chunks
	chunkSize := 50
	nodeListItems := nodeList.Items
	for i := 0; i < len(nodeListItems); i += chunkSize {

		chunkEnd := i + chunkSize
		if chunkEnd > len(nodeListItems) {
			chunkEnd = len(nodeListItems)
		}

		nodeList.Items = nodeListItems[i:chunkEnd]

		b, err := json.Marshal(nodeList)
		if err != nil {
			return reconcile.Result{}, err
		}
		_ = bytes.NewReader(b)

		/*
			err = postToRazeeDash(url, r, razeeOrgKey)
			if err != nil {
				return reconcile.Result{}, err
			}
		*/

	}

	reqLogger.Info("reconcilation complete")
	return reconcile.Result{RequeueAfter: time.Second * 300}, nil
}

// Get the RazeeDash URL & Org Key from the rhm-operator-secret
func (r *WatchReconciler) getRazeeDashKeys() ([]byte, []byte, reconcile.Result, error) {
	var url []byte
	var key []byte
	rhmOperatorSecret := corev1.Secret{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      utils.RHM_OPERATOR_SECRET_NAME,
		Namespace: r.cfg.DeployedNamespace,
	}, &rhmOperatorSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			return url, key, reconcile.Result{RequeueAfter: time.Second * 60}, nil
		} else {
			return url, key, reconcile.Result{}, err
		}
	}

	url, err = utils.ExtractCredKey(&rhmOperatorSecret, corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: utils.RHM_OPERATOR_SECRET_NAME,
		},
		Key: utils.RAZEE_DASH_URL_FIELD,
	})
	if err != nil {
		return url, key, reconcile.Result{RequeueAfter: time.Second * 60}, nil
	}

	key, err = utils.ExtractCredKey(&rhmOperatorSecret, corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: utils.RHM_OPERATOR_SECRET_NAME,
		},
		Key: utils.RAZEE_DASH_ORG_KEY_FIELD,
	})
	if err != nil {
		return url, key, reconcile.Result{RequeueAfter: time.Second * 60}, nil
	}

	return url, key, reconcile.Result{}, nil
}

func (r *WatchReconciler) getRazeeDashURL(baseurl string, clusterID string) (string, error) {
	var urlStr string

	urlStr = fmt.Sprintf(baseurl, "/clusters/", clusterID, "/resources")
	url, err := url.Parse(urlStr)
	if err != nil {
		return urlStr, err
	}

	return url.String(), nil
}

func (r *WatchReconciler) prepNodeObject2Send(nodeList *corev1.NodeList) {
	for i, _ := range nodeList.Items {
		annotations := nodeList.Items[i].GetAnnotations()
		delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
		delete(annotations, "kapitan.razee.io/last-applied-configuration")
		delete(annotations, "deploy.razee.io/last-applied-configuration")
		nodeList.Items[i].SetAnnotations(annotations)

		nodeList.Items[i].Status.Images = []corev1.ContainerImage{}
	}
}

func (r *WatchReconciler) postToRazeeDash(url string, body io.Reader, razeeOrgKey string) error {

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
