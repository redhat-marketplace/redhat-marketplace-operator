package marketplaceconfig

import (
	"context"

	opsrcv1 "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	"github.com/operator-framework/operator-sdk/pkg/status"
	pflag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	marketplacev1alpha1 "github.ibm.com/symposium/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	RELATED_IMAGE_MARKETPLACE_AGENT = "RELATED_IMAGE_MARKETPLACE_AGENT"
	DEFAULT_IMAGE_MARKETPLACE_AGENT = "marketplace-agent:latest"
	RAZEE_FLAG                      = "razee"
	METERBASE_FLAG                  = "meterbase"
)

var (
	log                      = logf.Log.WithName("controller_marketplaceconfig")
	marketplaceConfigFlagSet *pflag.FlagSet
	defaultFeatures          = []string{RAZEE_FLAG, METERBASE_FLAG}
)

// Init declares our FlagSet for the MarketplaceConfig
// Currently only has 1 set of flags for setting the Image
func init() {
	marketplaceConfigFlagSet = pflag.NewFlagSet("marketplaceconfig", pflag.ExitOnError)
	marketplaceConfigFlagSet.StringSlice(
		"features",
		defaultFeatures,
		"List of additional features to install. Ex. [razee, meterbase], etc.",
	)
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

	if marketplaceConfig.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionInstalling) == nil {
		marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionInstalling,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonStartInstall,
			Message: "Installing starting",
		})

		_ = r.client.Status().Update(context.TODO(), marketplaceConfig)
	}

	installFeatures := viper.GetStringSlice("features")
	installSet := make(map[string]bool)
	for _, installFlag := range installFeatures {
		reqLogger.Info("Feature Flag Found", "Flag Name: ", installFlag)
		installSet[installFlag] = true
	}

	// If auto-install is true MarketplaceConfig should create RazeeDeployment CR and MeterBase CR
	reqLogger.Info("auto installing crs")
	_, installExists := installSet[RAZEE_FLAG]
	if installExists {
		//Check if RazeeDeployment exists, if not create one
		foundRazee := &marketplacev1alpha1.RazeeDeployment{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: utils.RAZEE_NAME, Namespace: marketplaceConfig.Namespace}, foundRazee)
		if err != nil && errors.IsNotFound(err) {
			newRazeeCrd := utils.BuildRazeeCr(marketplaceConfig.Namespace, marketplaceConfig.Spec.ClusterUUID, marketplaceConfig.Spec.DeploySecretName)
			reqLogger.Info("creating razee cr")
			err = r.client.Create(context.TODO(), newRazeeCrd)

			patch := client.MergeFrom(marketplaceConfig.DeepCopy())

			marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonRazeeInstalled,
				Message: "RazeeDeployment installed.",
			})

			_ = r.client.Status().Patch(context.TODO(), marketplaceConfig, patch)

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
		if err = controllerutil.SetControllerReference(marketplaceConfig, foundRazee, r.scheme); err != nil {
			return reconcile.Result{}, err
		}
		reqLogger.Info("found razee")
	}

	_, installExists = installSet[METERBASE_FLAG]
	if installExists {
		// Check if MeterBase exists, if not create one
		foundMeterBase := &marketplacev1alpha1.MeterBase{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: utils.METERBASE_NAME, Namespace: marketplaceConfig.Namespace}, foundMeterBase)
		if err != nil && errors.IsNotFound(err) {
			newMeterBaseCr := utils.BuildMeterBaseCr(marketplaceConfig.Namespace)
			reqLogger.Info("creating meterbase")
			err = r.client.Create(context.TODO(), newMeterBaseCr)
			if err != nil {
				reqLogger.Error(err, "Failed to create a new MeterBase CR.")
				return reconcile.Result{}, err
			}

			patch := client.MergeFrom(marketplaceConfig.DeepCopy())

			marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonMeterBaseInstalled,
				Message: "Meter base installed.",
			})

			_ = r.client.Status().Patch(context.TODO(), marketplaceConfig, patch)

			return reconcile.Result{Requeue: true}, nil
		} else if err != nil {
			reqLogger.Error(err, "Failed to get MeterBase CR")
			return reconcile.Result{}, err
		}
		// Sets the owner for MeterBase
		if err = controllerutil.SetControllerReference(marketplaceConfig, foundMeterBase, r.scheme); err != nil {
			return reconcile.Result{}, err
		}
		reqLogger.Info("found meterbase")
	}

	// Check if operator source exists, or create a new one
	foundOpSrc := &opsrcv1.OperatorSource{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      utils.OPSRC_NAME,
		Namespace: utils.OPERATOR_MKTPLACE_NS},
		foundOpSrc)
	if err != nil && errors.IsNotFound(err) {
		// Define a new operator source
		newOpSrc := utils.BuildNewOpSrc()
		reqLogger.Info("Creating a new opsource")
		err = r.client.Create(context.TODO(), newOpSrc)
		if err != nil {
			reqLogger.Info("Failed to create an OperatorSource.", "OperatorSource.Namespace ", newOpSrc.Namespace, "OperatorSource.Name", newOpSrc.Name)
			return reconcile.Result{}, err
		}

		patch := client.MergeFrom(marketplaceConfig.DeepCopy())

		marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionInstalling,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonMeterBaseInstalled,
			Message: "RHM Operator source installed.",
		})

		_ = r.client.Status().Patch(context.TODO(), marketplaceConfig, patch)

		// Operator Source created successfully - return and requeue
		newOpSrc.ForceUpdate()
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		// Could not get Operator Source
		reqLogger.Error(err, "Failed to get OperatorSource")
	}

	patch := client.MergeFrom(marketplaceConfig.DeepCopy())

	marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
		Type:    marketplacev1alpha1.ConditionInstalling,
		Status:  corev1.ConditionFalse,
		Reason:  marketplacev1alpha1.ReasonInstallFinished,
		Message: "Finished Installing necessary components",
	})

	_ = r.client.Status().Patch(context.TODO(), marketplaceConfig, patch)

	reqLogger.Info("Found opsource")
	reqLogger.Info("reconciling finished")
	return reconcile.Result{}, nil
}

// labelsForMarketplaceConfig returs the labels for selecting the resources
// belonging to the given marketplaceConfig custom resource name
func labelsForMarketplaceConfig(name string) map[string]string {
	return map[string]string{"app": "marketplaceconfig", "marketplaceconfig_cr": name}
}
