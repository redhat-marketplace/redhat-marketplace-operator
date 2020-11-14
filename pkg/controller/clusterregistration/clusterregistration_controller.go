package clusterregistration

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/dgrijalva/jwt-go"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/marketplace"
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
	RHM_REGISTRATION_ENDPOINT = "https://marketplace.redhat.com/provisioning/v1/registered-clusters?accountId=account-id&uuid=cluster-uuid"
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
			secretName := secret.ObjectMeta.Name
			if _, ok := secret.Data[RHM_PULL_SECRET_KEY]; ok && secretName == RHM_PULL_SECRET_NAME {
				return true
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			secret, ok := e.ObjectNew.(*v1.Secret)
			if !ok {
				return false
			}
			if _, ok := secret.Data[RHM_PULL_SECRET_KEY]; !ok {
				return false
			}
			return e.ObjectOld != e.ObjectNew
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
	rhmPullSecret := v1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: RHM_PULL_SECRET_NAME, Namespace: request.Namespace}, &rhmPullSecret)
	if err != nil {
		return reconcile.Result{}, err
	}
	annotations := rhmPullSecret.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	reqLogger.Info("redhat-marketplace-pull-secret Secret found")

	// Check condition if 'PULL_SECRET' key is missing in secret
	if _, ok := rhmPullSecret.Data[RHM_PULL_SECRET_KEY]; !ok {
		reqLogger.Info("Missing token filed in secret")
		annotations[RHM_PULL_SECRET_STATUS] = "error"
		annotations[RHM_PULL_SECRET_MESSAGE] = "key with name 'PULL_SECRET' is missing in secret"
		rhmPullSecret.SetAnnotations(annotations)
		if err := r.client.Update(context.TODO(), &rhmPullSecret); err != nil {
			reqLogger.Error(err, "Failed to patch secret with Endpoint status")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Secret updated with status on failiure")
	}

	//Get Account Id from Pull Secret Token
	rhmAccountId, err := getAccountIdFromJWTToken(string(rhmPullSecret.Data[RHM_PULL_SECRET_KEY]))
	reqLogger.Info("Account id found in token", "accountid", rhmAccountId)
	if rhmAccountId == "" || err != nil {
		reqLogger.Error(err, "Token is missing account id")
		annotations[RHM_PULL_SECRET_STATUS] = "error"
		annotations[RHM_PULL_SECRET_MESSAGE] = "Account id is not available in provided token, Pleasr generate token from RH Marketplace again"
		rhmPullSecret.SetAnnotations(annotations)
		if err := r.client.Update(context.TODO(), &rhmPullSecret); err != nil {
			reqLogger.Error(err, "Failed to patch secret with Endpoint status")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Secret updated with account id missing error")
	}

	// Check condition if secret is having different namespace
	if rhmPullSecret.ObjectMeta.Namespace != RHM_NAMESPACE {
		annotations[RHM_PULL_SECRET_STATUS] = "error"
		annotations[RHM_PULL_SECRET_MESSAGE] = "Secret should create in 'openshift-redhat-marketplace' namespace"
		rhmPullSecret.SetAnnotations(annotations)
		if err := r.client.Update(context.TODO(), &rhmPullSecret); err != nil {
			reqLogger.Error(err, "Failed to patch secret with Endpoint status")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Secret updated with status on failiure")
	}

	// Fetch the Marketplace Config object
	reqLogger.Info("Finding MarketPlace config object >>>>>>>>>>>>>>")
	marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
	err = r.client.Get(context.TODO(), request.NamespacedName, marketplaceConfig)
	if err != nil {
		reqLogger.Info("MarketPlace config object not found, move to create")

	}

	if err == nil && marketplaceConfig.Spec.RhmAccountID != "" {
		reqLogger.Info("MarketPlace config object found, check status if its installed or not")
		//Setting MarketplaceClientAccount
		mClientRegistrationConfig := &MarketplaceClientConfig{
			Url:   RHM_REGISTRATION_ENDPOINT,
			Token: string(rhmPullSecret.Data[RHM_PULL_SECRET_KEY]),
		}
		marketplaceClientAccount := &MarketplaceClientAccount{
			AccountId:   marketplaceConfig.Spec.RhmAccountID,
			ClusterUuid: marketplaceConfig.Spec.ClusterUUID,
		}
		newMarketPlaceRegistrationClient, err := NewMarketplaceClient(mClientRegistrationConfig)
		if err != nil {
			return reconcile.Result{}, nil
		}
		// Marketplace config object found
		reqLogger.Info("Pulling MarketPlace config object status")
		registrationStatusOutput := newMarketPlaceRegistrationClient.RegistrationStatus(marketplaceClientAccount)

		if registrationStatusOutput.RegistrationStatus == "INSTALLED" {
			reqLogger.Info("MarketPlace config object is already registered for account", "accountid", marketplaceConfig.Spec.RhmAccountID)
			return reconcile.Result{}, nil
		}
	}
	reqLogger.Info("MarketPlace config object not found ")

	reqLogger.Info("RHMarketPlace Pull Secret token found>>>>>>>", "token", string(rhmPullSecret.Data[RHM_PULL_SECRET_KEY]))
	//Calling POST endpoint to pull the secret definition
	mClientPullSecretConfig := &MarketplaceClientConfig{
		Url:   RHM_PULL_SECRET_ENDPOINT,
		Token: string(rhmPullSecret.Data[RHM_PULL_SECRET_KEY]),
	}
	newMarketPlaceSecretClient, err := NewMarketplaceClient(mClientPullSecretConfig)
	data, err := newMarketPlaceSecretClient.GetMarketPlaceSecret()

	reqLogger.Info("RHMarketPlaceSecret data found")
	if err != nil {
		reqLogger.Info("RHMarketPlaceSecret failiure")
		reqLogger.Error(err, "RHMarketPlaceSecret failiure")
		annotations[RHM_PULL_SECRET_STATUS] = "error"
		annotations[RHM_PULL_SECRET_MESSAGE] = string(data)
		rhmPullSecret.SetAnnotations(annotations)
		if err := r.client.Update(context.TODO(), &rhmPullSecret); err != nil {
			reqLogger.Error(err, "Failed to patch secret with Endpoint status")
		}
		return reconcile.Result{}, err

	}
	//Convert yaml string to Secret object
	reqLogger.Info("Secret object", "data", data)
	newOptSecretObj := v1.Secret{}
	err = json.Unmarshal(data, &newOptSecretObj)
	if err != nil {
		return reconcile.Result{}, err
	}

	//Fetch the Secret with name redhat-Operator-secret
	optSecret := v1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: RHM_OPERATOR_SECRET_NAME, Namespace: request.Namespace}, &optSecret)
	if err != nil {
		reqLogger.Error(err, "rhm-operator-secret Secret not found")
		err = r.client.Create(context.TODO(), &newOptSecretObj)
		if err != nil {
			reqLogger.Error(err, "Failed to Create Secret Object")
			return reconcile.Result{}, err
		}
	}

	if err == nil {
		reqLogger.Info("optSecret child value>>>>>", "optSecretchild", optSecret.Data["CHILD_RRS3_YAML_FILENAME"])
		reqLogger.Info("newOptSecretObj child value>>>>>", "newOptSecretObjchild", newOptSecretObj.Data["CHILD_RRS3_YAML_FILENAME"])
		reqLogger.Info("Comparing old and new rhm-operator-secret")
		//compareRhmOperatorSecretObject(newOptSecretObj,optSecret)
		optSecretChanged := reflect.DeepEqual(newOptSecretObj.Data, optSecret.Data)
		if !optSecretChanged {
			reqLogger.Info("rhm-operator-secret are different copy, Replacing with new", "optSecretChanged", optSecretChanged)
			err := r.client.Update(context.TODO(), &newOptSecretObj)
			if err != nil {
				reqLogger.Error(err, "could not update rhm-operator-secret with new object", "Resource", RHM_OPERATOR_SECRET_NAME)
				return reconcile.Result{}, err
			}
		}

	}

	//Update secret with status
	if err == nil {
		reqLogger.Info("Updating secret with success status")
		annotations[RHM_PULL_SECRET_STATUS] = "success"
		annotations[RHM_PULL_SECRET_MESSAGE] = "rhm-operator-secret generated successfully"
		rhmPullSecret.SetAnnotations(annotations)
		if err := r.client.Update(context.TODO(), &rhmPullSecret); err != nil {
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
	newMarketplaceConfig.Spec.RhmAccountID = rhmAccountId
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

//Parsing JWT token and fetching rhmAccountId
func getAccountIdFromJWTToken(jwtToken string) (string, error) {
	token, _ := jwt.Parse(jwtToken, nil)
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		rhmAccountClaim := claims["rhmAccountId"]
		if rhmAccountID, ok := rhmAccountClaim.(string); ok {
			return rhmAccountID, nil
		}
	}
	return "", errors.New("rhmAccountId not found")
}
