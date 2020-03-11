package marketplaceconfig

import (
	"context"

	opsrcApi "github.com/operator-framework/operator-marketplace/pkg/apis"
	opsrcv1 "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	pflag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	marketplacev1alpha1 "github.ibm.com/symposium/marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.ibm.com/symposium/marketplace-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	istr "k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	CSCFinalizer                    = "finalizer.MarketplaceConfigs.operators.coreos.com"
	OPSRC_NAME                      = "redhat-marketplace-operators"
	RAZEE_NAME                      = "marketplaceconfig-razeedeployment"
	RELATED_IMAGE_MARKETPLACE_AGENT = "RELATED_IMAGE_MARKETPLACE_AGENT"
	DEFAULT_IMAGE_MARKETPLACE_AGENT = "marketplace-agent:latest"
)

var (
	log                      = logf.Log.WithName("controller_marketplaceconfig")
	marketplaceConfigFlagSet *pflag.FlagSet
)

// Init declares our FlagSet for the MarketplaceConfig
// Currently only has 1 set of flags for setting the Image
func init() {
	marketplaceConfigFlagSet = pflag.NewFlagSet("marketplaceconfig", pflag.ExitOnError)
	marketplaceConfigFlagSet.String(
		"related-image-operator-agent",
		utils.Getenv(RELATED_IMAGE_MARKETPLACE_AGENT, DEFAULT_IMAGE_MARKETPLACE_AGENT),
		"Image for marketplaceConfig")
}

// FlagSet returns our FlagSet
func FlagSet() *pflag.FlagSet {
	return marketplaceConfigFlagSet
}

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new MarketplaceConfig Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMarketplaceConfig{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("marketplaceconfig-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MarketplaceConfig
	err = c.Watch(&source.Kind{Type: &marketplacev1alpha1.MarketplaceConfig{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner MarketplaceConfig
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MarketplaceConfig{},
	})
	if err != nil {
		return err
	}

	// watch operator source and requeue the owner MarketplaceConfig
	err = c.Watch(&source.Kind{Type: &opsrcv1.OperatorSource{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MarketplaceConfig{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileMarketplaceConfig implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMarketplaceConfig{}

// ReconcileMarketplaceConfig reconciles a MarketplaceConfig object
type ReconcileMarketplaceConfig struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a MarketplaceConfig object and makes changes based on the state read
// and what is in the MarketplaceConfig.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileMarketplaceConfig) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MarketplaceConfig")

	// Fetch the MarketplaceConfig instance
	marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
	err := r.client.Get(context.TODO(), request.NamespacedName, marketplaceConfig)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("Resource not found. Ignoring since object must be deleted")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get MarketplaceConfig")
		return reconcile.Result{}, err
	}

	// Check if deployment exists, otherwise create a new one
	found := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: marketplaceConfig.Name, Namespace: marketplaceConfig.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {

		// Define a new deployment
		dep := r.deploymentForMarketplaceConfig(marketplaceConfig)
		reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.client.Create(context.TODO(), dep)
		// Error creating deployment - requeue the request
		if err != nil {
			reqLogger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return reconcile.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		// Could not get delpoyment
		reqLogger.Error(err, "Failed to get Deployment")
		return reconcile.Result{}, err
	}

	// Ensure deployment size is the same as spec
	size := marketplaceConfig.Spec.Size
	if *found.Spec.Replicas != size {
		found.Spec.Replicas = &size
		err = r.client.Update(context.TODO(), found)
		// Failed to update deployment
		if err != nil {
			reqLogger.Error(err, "Failed to update Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
			return reconcile.Result{}, err
		}
		//Spec updated - return and requeue
		return reconcile.Result{Requeue: true}, nil
	}

	// Check if operator source exists, or create a new one
	foundOpSrc := &opsrcv1.OperatorSource{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: OPSRC_NAME, Namespace: marketplaceConfig.Namespace}, foundOpSrc)
	if err != nil && errors.IsNotFound(err) {
		// Define a new operator source
		newOpSrc := createNewOpSrc(marketplaceConfig)
		err = r.client.Create(context.TODO(), newOpSrc)
		if err != nil {
			reqLogger.Error(err, "Failed to create an OperatorSource.", "OperatorSource.Namespace ", newOpSrc.Namespace, "OperatorSource.Name", newOpSrc.Name)
			return reconcile.Result{}, err
		}
		// Operator Source created successfully - return and requeue
		newOpSrc.ForceUpdate()
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		// Could not get Operator Source
		reqLogger.Error(err, "Failed to get OperatorSource")
		return reconcile.Result{}, err
	}

	// Sets the owner for foundOpSrc
	opsrcApi.AddToScheme(r.scheme)
	if err = controllerutil.SetControllerReference(found, foundOpSrc, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	//Check if RazeeDeployment exists, if not create one
	foundRazee := &marketplacev1alpha1.RazeeDeployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: RAZEE_NAME, Namespace: marketplaceConfig.Namespace}, foundRazee)
	if err != nil && errors.IsNotFound(err) {
		newRazeeCrd := createRazeeCr(marketplaceConfig)
		err = r.client.Create(context.TODO(), newRazeeCrd)
		if err != nil {
			reqLogger.Error(err, "Failed to create a new RazeeDeployment CR.")
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get RazeeDeployment CR")
		return reconcile.Result{}, err
	}

	// Sets the owner for foundRazee
	if err = controllerutil.SetControllerReference(found, foundRazee, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// deploymentForMarketplaceConfig will return a marketplaceConfig Deployment object
func (r *ReconcileMarketplaceConfig) deploymentForMarketplaceConfig(m *marketplacev1alpha1.MarketplaceConfig) *appsv1.Deployment {
	ls := labelsForMarketplaceConfig(m.Name)
	replicas := m.Spec.Size

	image := viper.GetString("related-image-operator-agent")

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
			Labels:    ls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:           image,
						Name:            "marketconfig",
						ImagePullPolicy: "IfNotPresent",
						//TODO: After merge, can use utils, for probs instead
						LivenessProbe: &corev1.Probe{
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/v1/check/healthz",
									Port:   istr.FromInt(8080),
									Scheme: "HTTP",
								},
							},
							InitialDelaySeconds: 3,
							TimeoutSeconds:      1,
							PeriodSeconds:       3,
							SuccessThreshold:    1,
							FailureThreshold:    3,
						},
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8080,
						}},
					}},
				},
			},
		},
	}
	controllerutil.SetControllerReference(m, dep, r.scheme)
	return dep
}

// labelsForMarketplaceConfig returs the labels for selecting the resources
// belonging to the given marketplaceConfig custom resource name
func labelsForMarketplaceConfig(name string) map[string]string {
	return map[string]string{"app": "marketplaceconfig", "marketplaceconfig_cr": name}
}

// createNewOpSrc returns a new Operator Source
func createNewOpSrc(cr *marketplacev1alpha1.MarketplaceConfig) *opsrcv1.OperatorSource {

	opsrc := &opsrcv1.OperatorSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OPSRC_NAME,
			Namespace: cr.Namespace,
		},
		Spec: opsrcv1.OperatorSourceSpec{
			DisplayName:       "Red Hat Marketplace",
			Endpoint:          "https://quay.io/cnr",
			Publisher:         "Red Hat Marketplace",
			RegistryNamespace: "redhat-marketplace",
			Type:              "appregistry",
		},
	}

	return opsrc
}

// createRazeeCrd returns a RazeeDeployment cr with default values
func createRazeeCr(marketplace *marketplacev1alpha1.MarketplaceConfig) *marketplacev1alpha1.RazeeDeployment {

	cr := &marketplacev1alpha1.RazeeDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RAZEE_NAME, // CHANGE THIS -> DONT LEAVE HARDCODED
			Namespace: marketplace.Namespace,
		},
		Spec: marketplacev1alpha1.RazeeDeploymentSpec{
			Enabled: true,
		},
	}

	return cr
}
