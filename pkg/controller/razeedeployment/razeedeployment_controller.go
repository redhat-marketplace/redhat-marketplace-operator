package razeedeployment
// TODO: remove print statements
import (
	"bytes"
	"context"
	"fmt"
	ioutil "io/ioutil"
    "path/filepath"
	marketplacev1alpha1 "github.ibm.com/symposium/marketplace-operator/pkg/apis/marketplace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8yaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"github.com/spf13/viper"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"




	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_razeedeployment")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new RazeeDeployment Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileRazeeDeployment{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("razeedeployment-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource RazeeDeployment
	err = c.Watch(&source.Kind{Type: &marketplacev1alpha1.RazeeDeployment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// watch the razee Namespace
	err = c.Watch(&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.RazeeDeployment{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileRazeeDeployment implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileRazeeDeployment{}

// ReconcileRazeeDeployment reconciles a RazeeDeployment object
type ReconcileRazeeDeployment struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a RazeeDeployment object and makes changes based on the state read
// and what is in the RazeeDeployment.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileRazeeDeployment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling RazeeDeployment")

	// Fetch the RazeeDeployment instance
	instance := &marketplacev1alpha1.RazeeDeployment{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if !instance.Spec.Enabled {
		reqLogger.Info("Razee not enabled")
		return reconcile.Result{}, nil
	}
	
	// Define a new Namespace object
	assetBase := viper.GetString("assets")
	cfgBaseFileName := filepath.Join(assetBase, "prometheus/base-configmap.yaml")
	namespace, err := createRazeeNamespaceWithUtil(cfgBaseFileName)
	// Check if this namespace already exists
	if err != nil {
		return reconcile.Result{}, err
	}
	found := &corev1.Namespace{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "razee"}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Namespace")
		err = r.client.Create(context.TODO(), namespace)
		if err != nil {
			return reconcile.Result{}, err
		}
		// Namespace created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}
	
	if err := controllerutil.SetControllerReference(instance, namespace, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Update CR status
	// List the namespaces on the cluster
	namespaceList := &corev1.NamespaceList{}
	if err = r.client.List(context.TODO(), namespaceList); err != nil {
		reqLogger.Error(err, "Failed to list namespaces")
		return reconcile.Result{}, err
	}

	// Update status.Namespace
	nsList := *namespaceList
	namespaces := getNamespaceNames(nsList.Items)
	if contains(namespaces, "razee") {
		instance.Status.Namespace = "razee"
		err := r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update RazeeDeploy status")
			return reconcile.Result{}, err
		}
	}

	// Namespace already exists - don't requeue
	reqLogger.Info("Skip reconcile: Namespace already exists", "Namespace.Namespace", found.Namespace)
	return reconcile.Result{}, nil
}

func createRazeeNamespaceWithUtil(filename string) (*corev1.Namespace, error) {
	dat, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	ns := &corev1.Namespace{}
	dec := k8yaml.NewYAMLOrJSONDecoder(bytes.NewReader(dat), 1000)
	if err := dec.Decode(&ns); err != nil {
		return nil, err
	}

	ns.Name = "razee"
	fmt.Println(ns)
	return ns, nil
}

func getNamespaceNames(ns []corev1.Namespace) []string {
	var namespaceNames []string
	for _, namespace := range ns {
		namespaceNames = append(namespaceNames, namespace.Name)
	}

	return namespaceNames
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
