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
	"context"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	"github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"
	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// blank assignment to verify that ReconcileNode implements reconcile.Reconciler
var _ reconcile.Reconciler = &RemoteResourceS3Reconciler{}

// ReconcileNode reconciles a Node object
type RemoteResourceS3Reconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger

	cfg *config.OperatorConfig
}

func (r *RemoteResourceS3Reconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (r *RemoteResourceS3Reconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	r.cfg = cfg
	return nil
}

func (r *RemoteResourceS3Reconciler) SetupWithManager(mgr manager.Manager) error {
	labelPreds := []predicate.Predicate{
		predicate.Funcs{
			UpdateFunc: func(evt event.UpdateEvent) bool {
				return evt.ObjectNew.GetNamespace() == r.cfg.DeployedNamespace && evt.ObjectNew.GetNamespace() == r.cfg.DeployedNamespace
			},
			CreateFunc: func(evt event.CreateEvent) bool {
				return evt.Object.GetNamespace() == r.cfg.DeployedNamespace
			},
			GenericFunc: func(evt event.GenericEvent) bool {
				return false
			},
			DeleteFunc: func(evt event.DeleteEvent) bool {
				return false
			},
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&marketplacev1alpha1.RemoteResourceS3{}).
		Watches(&source.Kind{Type: &marketplacev1alpha1.RemoteResourceS3{}}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(labelPreds...)).
		Complete(r)
}

// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=remoteresources3s,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=remoteresources3s;remoteresources3s/status,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=operators.coreos.com,resources=subscriptions;operatorgroups,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=*,verbs=get;list;watch;create;update;patch

// Reconcile reads that state of the cluster for a Node object and makes changes based on the state read
// and what is in the Node.Spec
func (r *RemoteResourceS3Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Name", request.Name, "Request.Namespace", request.Namespace)
	reqLogger.Info("Reconciling RemoteResourceS3")

	// Fetch the Node instance
	instance := &marketplacev1alpha1.RemoteResourceS3{}

	err := r.Client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Error(err, "remoteresources3 does not exist")
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get node")
		return reconcile.Result{}, err
	}

	if instance.Status.Touched == nil {
		instance.Status = marketplacev1alpha1.RemoteResourceS3Status{
			Touched: ptr.Bool(true),
		}
		err := r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		reqLogger.Info("updated remoteresources3")
	}

	// we don't need to look everywhere, just the spec.auth.iam
	// because we only use IAM right now
	authEndpoint := instance.Spec.Auth.Iam.URL
	secretKeyRef := instance.Spec.Auth.Iam.APIKeyRef.ValueFrom.SecretKeyRef
	apiKeySecret := &corev1.Secret{}

	err = r.Client.Get(ctx, types.NamespacedName{Name: secretKeyRef.Name, Namespace: instance.Namespace}, apiKeySecret)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Error(err, utils.RHMOperatorSecretName+" does not exist")
			return reconcile.Result{}, nil
		}

		reqLogger.Error(err, "Failed to get"+utils.RHMOperatorSecretName)
		return reconcile.Result{}, err
	}

	apiKey := apiKeySecret.Data[secretKeyRef.Key]
	conf := aws.NewConfig().
		WithCredentials(
			ibmiam.NewStaticCredentials(aws.NewConfig(), authEndpoint, string(apiKey), "")).
		WithS3ForcePathStyle(true)

	requests := append([]marketplacev1alpha1.Request{}, instance.Spec.Requests...)
	conditions := append(status.Conditions{}, instance.Status.Conditions...)

	var updateSpec, updateStatus bool

	updateStatus = updateStatus || conditions.SetCondition(status.Condition{
		Type:    marketplacev1alpha1.ResourceInstallError,
		Status:  corev1.ConditionFalse,
		Message: "No error found",
		Reason:  marketplacev1alpha1.NoBadRequest,
	})

	var failedRequestErr error

	for i := 0; i < len(requests); i++ {
		request := &requests[i]
		updateSpec = updateSpec || r.handleRequest(ctx, instance, request, *conf)

		if request.StatusCode >= 300 {
			failedRequestErr = fmt.Errorf("%s", request.Message)
			updateStatus = updateStatus || conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ResourceInstallError,
				Status:  corev1.ConditionTrue,
				Message: request.Message,
				Reason:  marketplacev1alpha1.FailedRequest,
			})
		}
	}

	if updateSpec || updateStatus {
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			err := r.Client.Get(ctx, client.ObjectKeyFromObject(instance), instance)

			if err != nil {
				return err
			}

			instance.Spec.Requests = requests
			return r.Client.Update(ctx, instance)
		})

		if err != nil {
			reqLogger.Error(err, "Failed to update CR.")
			return reconcile.Result{}, err
		}
	}

	if failedRequestErr != nil {
		return reconcile.Result{}, failedRequestErr
	}

	reqLogger.Info("finished reconcile, requeuing in a minute")
	return reconcile.Result{RequeueAfter: 2 * time.Minute}, nil
}

func (r *RemoteResourceS3Reconciler) handleRequest(
	ctx context.Context,
	instance *marketplacev1alpha1.RemoteResourceS3,
	request *marketplacev1alpha1.Request,
	conf aws.Config,
) (update bool) {
	reqLogger := r.Log.WithValues("instance.Name", instance.Name, "instance.Namespace", instance.Namespace)
	update = true

	fileURLString := request.Options.URL
	fileURL, err := url.Parse(fileURLString)

	if err != nil {
		reqLogger.Error(err, "failed to get file", "file", fileURLString)
		request.Message = err.Error()
		request.StatusCode = 500
		return
	}

	pathSplit := strings.Split(fileURL.Path, "/")
	if len(pathSplit) < 3 {
		request.Message = "file path is not long enough"
		request.StatusCode = 500
		return
	}

	bucket := pathSplit[1]
	key := strings.Join(pathSplit[2:], "/")

	reqLogger.Info("file parsed", "bucket", bucket, "key", key)

	sess, err := session.NewSession()
	if err != nil {
		reqLogger.Error(err, "failed to get file", "file", fileURLString)
		request.Message = err.Error()
		request.StatusCode = 500
		return
	}

	s3Client := s3.New(sess, (&conf).WithEndpoint(fileURL.Host))
	result, err := s3Client.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		reqLogger.Error(err, "failed to get file", "file", fileURLString)
		request.Message = err.Error()
		request.StatusCode = 500
		return
	}

	fileBody, err := ioutil.ReadAll(result.Body)
	if err != nil {
		reqLogger.Error(err, "failed to get file", "file", fileURLString)
		request.Message = err.Error()
		request.StatusCode = 500
		return
	}

	bodyHash := fmt.Sprintf("%x", sha256.Sum256(fileBody))

	remoteObj := &unstructured.Unstructured{}
	localObj := &unstructured.Unstructured{}

	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, gvk, err := dec.Decode(fileBody, nil, remoteObj)

	if err != nil {
		reqLogger.Error(err, "failed to decode file")
		request.Message = err.Error()
		request.StatusCode = 500
		return
	}

	localObj.SetGroupVersionKind(*gvk)

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		err = r.Client.Get(ctx, client.ObjectKeyFromObject(remoteObj), localObj)

		reqLogger = reqLogger.WithValues("file", fileURLString, "key", client.ObjectKeyFromObject(remoteObj))

		if err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "failed to get file")
			return err
		}

		if errors.IsNotFound(err) {
			controllerutil.SetOwnerReference(instance, remoteObj, r.Scheme)
			err = r.Client.Create(ctx, remoteObj)
			if err != nil {
				reqLogger.Error(err, "failed to create file")
				return err
			}

			request.Hash = bodyHash
			request.Message = "Created"
			request.StatusCode = 201
			return nil
		}

		if request.Hash != "" && bodyHash == request.Hash {
			update = false
			return nil
		}

		request.Hash = bodyHash

		var remoteMetadata, localMetadata, remoteAnnotations, remoteLabels map[string]interface{}

		if meta, ok := remoteObj.Object["metadata"]; ok {
			remoteMetadata = meta.(map[string]interface{})
			if annotations, ok := remoteMetadata["annotations"]; ok {
				remoteAnnotations = annotations.(map[string]interface{})
			}

			if labels, ok := remoteMetadata["labels"]; ok {
				remoteLabels = labels.(map[string]interface{})
			}
		}

		if meta, ok := localObj.Object["metadata"]; ok {
			localMetadata = meta.(map[string]interface{})
		}

		localMetadata["annotations"] = remoteAnnotations
		localMetadata["labels"] = remoteLabels
		localObj.Object["spec"] = remoteObj.Object["spec"]
		return r.Client.Update(ctx, localObj)
	})

	if err != nil {
		reqLogger.Error(err, "failed to update")
		request.Message = err.Error()
		request.StatusCode = 500
		return
	}

	request.Message = "Updated"
	request.StatusCode = 204
	return
}
