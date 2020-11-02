package clusterregistration

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"

	yaml "github.com/ghodss/yaml"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_registration_watcher")

const (
	RHM_PULL_SECRET_NAME      = "redhat-marketplace-pull-secret"
	RHM_OPERATOR_SECRET_NAME  = "rhm-operator-secret"
	RHM_PULL_SECRET_KEY       = "PULL_SECRET"
	RHM_PULL_SECRET_ENDPOINT  = "https://marketplace.redhat.com/provisioning/v1/rhm-operator/rhm-operator-secret"
	RHM_REGISTRATION_ENDPOINT = "https://marketplace.redhat.com/provisioning/v1/registered-clusters?accountId=account-id&uuid=cluster-uuid&status=INSTALLED"
	RHM_PULL_SECRET_STATUS    = "marketplace.redhat.com/rhm-operator-secret-Status"
	RHM_PULL_SECRET_MESSAGE   = "marketplace.redhat.com/rhm-operator-secret-message"
	RHM_NAMESPACE             = "openshift-redhat-marketplace"
)

// Add creates a new ClusterRegistration Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileClusterRegistration{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// blank assignment to verify that ReconcileClusterRegistration implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileClusterRegistration{}

// ReconcileClusterRegistration reconciles a Registration object
type ReconcileClusterRegistration struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("clusterregistration-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for secret created with name 'redhat-marketplace-pull-secret'
	namePredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			secret, ok := e.Object.(*v1.Secret)
			if !ok {
				return false
			}
			value := secret.ObjectMeta.Name
			return value == RHM_PULL_SECRET_NAME
		},
	}
	err = c.Watch(&source.Kind{Type: &v1.Secret{}}, &handler.EnqueueRequestForObject{}, namePredicate)
	if err != nil {
		return err
	}
	return nil
}

// Reconcile reads that state of the cluster for a ClusterRegistration object and makes changes based on the state read
// and what is in the ClusterRegistration.Spec
func (r *ReconcileClusterRegistration) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ClusterRegistration")

	//Fetch the Secret with name redhat-marketplace-pull-secret
	secret := v1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: RHM_PULL_SECRET_NAME, Namespace: request.Namespace}, &secret)
	if err != nil {
		return reconcile.Result{}, err
	}

	//Fetch the Secret with name redhat-Operator-secret
	optSecret := v1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: RHM_OPERATOR_SECRET_NAME, Namespace: request.Namespace}, &optSecret)
	if err != nil {
		reqLogger.Error(err, "rhm-operator-secret Secret not found")
	}
	if err == nil {
		reqLogger.Info("Deleteing rhm-operator-secret")
		err := r.client.Delete(context.TODO(), &optSecret)
		if err != nil {
			reqLogger.Error(err, "could not delete optSecret", "Resource", RHM_OPERATOR_SECRET_NAME)
			return reconcile.Result{}, err
		}

	}
	reqLogger.Info("redhat-marketplace-pull-secret Secret found")
	annotations := secret.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	// Check condition if 'PULL_SECRET' key is missing in secret
	if _, ok := secret.Data[RHM_PULL_SECRET_KEY]; !ok {
		reqLogger.Info("Missing token filed in secret")
		annotations[RHM_PULL_SECRET_STATUS] = "error"
		annotations[RHM_PULL_SECRET_MESSAGE] = "key with name 'PULL_SECRET' is missing in secret"
		secret.SetAnnotations(annotations)
		if err := r.client.Update(context.TODO(), &secret); err != nil {
			reqLogger.Error(err, "Failed to patch secret with Endpoint status")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Secret updated with status on failiure")
	}
	//Calling POST endpoint to pull the secret definition
	bearerToken := "Bearer " + string(secret.Data[RHM_PULL_SECRET_KEY])
	httpClient := &http.Client{}
	// Fetch the Marketplace Config object
	marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
	err = r.client.Get(context.TODO(), request.NamespacedName, marketplaceConfig)
	if err != nil {
		reqLogger.Info("MarketPlace config object err >>>>>>>>>>>>>>")
		//return reconcile.Result{}, err
	}
	reqLogger.Info("FInding MarketPlace config object >>>>>>>>>>>>>>")
	if err == nil && marketplaceConfig.Spec.RhmAccountID != "" {
		// Marketplace config object found
		reqLogger.Info("MarketPlace config object found")
		accountId := marketplaceConfig.Spec.RhmAccountID
		clusterUuid := marketplaceConfig.Spec.ClusterUUID
		u, err := url.Parse(RHM_REGISTRATION_ENDPOINT)
		if err != nil {
			return reconcile.Result{}, err
		}
		q := u.Query()
		q.Set("accountId", accountId)
		q.Set("uuid", clusterUuid)
		u.RawQuery = q.Encode()
		req, _ := http.NewRequest("GET", u.String(), nil)
		req.Header.Add("Authorization", bearerToken)
		resp, err := httpClient.Do(req)

		if err != nil {
			return reconcile.Result{}, err
		}
		defer resp.Body.Close()
		clusterDef, err := ioutil.ReadAll(resp.Body)
		if resp.StatusCode == 200 && string(clusterDef) != "" {
			return reconcile.Result{}, nil
		}
	}
	reqLogger.Info("MarketPlace config object not found ")
	//Calling POST endpoint to pull the secret definition

	req, _ := http.NewRequest("GET", RHM_PULL_SECRET_ENDPOINT, nil)
	req.Header.Add("Authorization", bearerToken)
	resp, err := httpClient.Do(req)

	if err != nil {
		return reconcile.Result{}, err
	}
	defer resp.Body.Close()
	rhOperatorSecretDef, err := ioutil.ReadAll(resp.Body)
	reqLogger.Info("Responste SatusCode to pull secret from Marketplace endpoint", "status", resp.StatusCode)
	// Check condition if status code is not 200
	if resp.StatusCode != 200 {
		reqLogger.Info("Updating secret with error status", "response", string(rhOperatorSecretDef))
		annotations[RHM_PULL_SECRET_STATUS] = "error"
		annotations[RHM_PULL_SECRET_MESSAGE] = string(rhOperatorSecretDef)
		secret.SetAnnotations(annotations)
		if err := r.client.Update(context.TODO(), &secret); err != nil {
			reqLogger.Error(err, "Failed to patch secret with Endpoint status")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Secret updated with status on failiure")
	}
	// Check condition if secret is having different namespace
	if secret.ObjectMeta.Namespace != RHM_NAMESPACE {
		reqLogger.Info("Updating secret with error status", "response", string(rhOperatorSecretDef))
		annotations[RHM_PULL_SECRET_STATUS] = "error"
		annotations[RHM_PULL_SECRET_MESSAGE] = "Secret should create in 'openshift-redhat-marketplace' namespace"
		secret.SetAnnotations(annotations)
		if err := r.client.Update(context.TODO(), &secret); err != nil {
			reqLogger.Error(err, "Failed to patch secret with Endpoint status")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Secret updated with status on failiure")
	}
	if err != nil {
		return reconcile.Result{}, err
	}
	//Convert yaml string to Secret object
	secretObj := v1.Secret{}
	data, err := yaml.YAMLToJSON(rhOperatorSecretDef) //Converting Yaml to JSON so it can parse to secret object
	if err != nil {
		reqLogger.Error(err, "Failed to parse Yaml to json")
		return reconcile.Result{}, err
	}
	err = json.Unmarshal(data, &secretObj)

	if err != nil {
		return reconcile.Result{}, err
	}
	//Apply http request response in cluster to create secret
	err = r.client.Create(context.TODO(), &secretObj)
	if err != nil {
		reqLogger.Error(err, "Failed to Create Secret Object")
		return reconcile.Result{}, err
	}
	//Update secret with status
	if err == nil {
		reqLogger.Info("Updating secret with success status")
		annotations[RHM_PULL_SECRET_STATUS] = "success"
		annotations[RHM_PULL_SECRET_MESSAGE] = "rhm-operator-secret generated successfully"
		secret.SetAnnotations(annotations)
		if err := r.client.Update(context.TODO(), &secret); err != nil {
			reqLogger.Error(err, "Failed to patch secret with Endpoint status")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Secret updated with status on success")
	}
	//Create Markeplace Config object
	reqLogger.Info("finding clusterversion resource>>>>>>>>>>>>>>>>>>>>")
	clusterVersion := &unstructured.Unstructured{}
	clusterVersion.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "config.openshift.io",
		Kind:    "ClusterVersion",
		Version: "v1",
	})
	err = r.client.Get(context.Background(), client.ObjectKey{
		Name: "version",
	}, clusterVersion)
	if err != nil {
		reqLogger.Error(err, "Failed to retrieve clusterversion resource")
		return reconcile.Result{}, err
	}
	jsonString := clusterVersion.Object["spec"]
	var clusterID string
	if rec, ok := jsonString.(map[string]interface{}); ok {
		for key, val := range rec {
			if key == "clusterID" {
				if str, ok := val.(string); ok {
					clusterID = str
				} else {
					reqLogger.Info("clusterID is a Type object", key, val)
				}
				break
			}

		}
	} else {
		reqLogger.Info("record not a map[string]interface{}: %v\n", jsonString)
	}
	reqLogger.Info("Clusterversion object found with clusterID", "clusterID", clusterID)
	newMarketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
	newMarketplaceConfig.ObjectMeta.Name = "marketplaceconfig"
	newMarketplaceConfig.ObjectMeta.Namespace = "openshift-redhat-marketplace"
	newMarketplaceConfig.Spec.ClusterUUID = clusterID
	newMarketplaceConfig.Spec.RhmAccountID = "5f08d00472746d0014e10903" //Hard coded
	// Create Marketplace Config object with ClusterID
	reqLogger.Info("Marketplace Config creating >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>")
	err = r.client.Create(context.TODO(), newMarketplaceConfig)
	if err != nil {
		reqLogger.Error(err, "Failed to Create Marketplace Config Object>>>>>>>>>>>>>>>>>>>>>>")
		return reconcile.Result{}, err
	}
	reqLogger.Info("Marketplace Config Created >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>")
	return reconcile.Result{}, nil
}
