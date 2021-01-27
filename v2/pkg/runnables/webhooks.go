package runnables

import (
	"bytes"
	"context"
	"sync"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	typedapiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

type BootstrapWebhook struct {
	logger logr.Logger
	cc     ClientCommandRunner
	cfg    config.OperatorConfig
	client kubernetes.Interface
}

func NewBootstrapWebhook(
	logger logr.Logger,
	cc ClientCommandRunner,
	cfg config.OperatorConfig,
	client kubernetes.Interface,
) *BootstrapWebhook {
	log := logger.WithValues("func", "bootstrapWebhook")

	return &BootstrapWebhook{
		logger: log,
		cc:     cc,
		cfg:    cfg,
		client: client,
	}
}

type WebhooksAvailable struct {
	Webhooks map[string]bool
	sync.Mutex
}

func (a *BootstrapWebhook) NeedLeaderElection() bool {
	return true
}

func (a *BootstrapWebhook) Start(stop <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	webhooksAvailable := map[string]bool{
		"mmeterdefinition.marketplace.redhat.com": false,
		"vmeterdefinition.marketplace.redhat.com": false,
	}

	webhooks := &WebhooksAvailable{
		Webhooks: webhooksAvailable,
	}

	errChan := make(chan error)

	go func() {
		err := a.Run(ctx, webhooks)
		defer close(errChan)

		if err != nil {
			a.logger.Error(err, "error running bootstrapWebhook")
			errChan <- err
		}
		cancel()
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errChan:
		return err
	case <-stop:
		cancel()
		return nil
	}
}

const (
	OLMOwnerName      = "olm.owner"
	OLMOwnerKind      = "olm.owner.kind"
	OLMOwnerNamespace = "olm.owner.namespace"

	InjectCAAnnotation = "service.beta.openshift.io/inject-cabundle"
)

// +kubebuilder:rbac:groups="",resources=secret,verbs=get;list;watch
// +kubebuilder:rbac:groups="apiextensions.k8s.io",resources=customresourcedefinitions,verbs=get;list;watch
// +kubebuilder:rbac:groups="apiextensions.k8s.io",resources=customresourcedefinitions,resourceNames=meterdefinitions.marketplace.redhat.com,verbs=update

func (a *BootstrapWebhook) Run(ctx context.Context, webhooks *WebhooksAvailable) error {
	a.logger.Info("starting bootstrapWebhook")

	mutatingClient := a.client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations()
	validatingClient := a.client.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations()

	// labelSelector := &metav1.LabelSelector{
	// 	MatchLabels: map[string]string{
	// 		OLMOwnerName:      a.cfg.OLMInformation.OwnerName,
	// 		OLMOwnerKind:      a.cfg.OLMInformation.OwnerKind,
	// 		OLMOwnerNamespace: a.cfg.OLMInformation.OwnerNamespace,
	// 	},
	// }

	mWebhookWatcher, err := mutatingClient.Watch(
		ctx, metav1.ListOptions{
			//LabelSelector: metav1.FormatLabelSelector(labelSelector),
		},
	)

	if err != nil {
		a.logger.Error(err, "error creating mutating watcher")
		return err
	}

	vWebhookWatcher, err := validatingClient.Watch(
		ctx, metav1.ListOptions{
			//LabelSelector: metav1.FormatLabelSelector(labelSelector),
		},
	)

	if err != nil {
		a.logger.Error(err, "error creating validation watcher")
		return err
	}

	for {
		select {
		case evt := <-mWebhookWatcher.ResultChan():
			obj, err := meta.Accessor(evt.Object)
			if err != nil {
				a.logger.Error(err, "error to convert")
				return err
			}

			a.logger.Info("received mwebhook", "name", obj.GetName())

			err = a.addAnnotationToWebhooks(obj, webhooks, func() error {
				a.logger.Info("updating mwebhook", "name", obj.GetName())
				webhook, err := mutatingClient.Get(ctx, obj.GetName(), metav1.GetOptions{})
				if err != nil {
					return err
				}

				for _, hook := range webhook.Webhooks {
					localHook := &hook
					localHook.ClientConfig.Service.Name = "redhat-marketplace-controller-webhook-service"
				}

				if webhook.GetAnnotations() == nil {
					webhook.SetAnnotations(make(map[string]string))
				}

				webhook.GetAnnotations()[InjectCAAnnotation] = "true"

				obj, err = mutatingClient.Update(ctx, webhook, metav1.UpdateOptions{})

				return err
			})

			if err != nil {
				a.logger.Error(err, "error updating")
				return err
			}
		case evt := <-vWebhookWatcher.ResultChan():
			obj, err := meta.Accessor(evt.Object)
			if err != nil {
				a.logger.Error(err, "error converting")
				return err
			}

			a.logger.Info("received vwebhook", "name", obj.GetName())

			err = a.addAnnotationToWebhooks(obj, webhooks, func() error {
				a.logger.Info("updating vwebhook", "name", obj.GetName())
				webhook, err := validatingClient.Get(ctx, obj.GetName(), metav1.GetOptions{})
				if err != nil {
					return err
				}

				if webhook.GetAnnotations() == nil {
					webhook.SetAnnotations(make(map[string]string))
				}

				for _, hook := range webhook.Webhooks {
					localHook := &hook
					localHook.ClientConfig.Service.Name = "redhat-marketplace-controller-webhook-service"
				}

				webhook.GetAnnotations()[InjectCAAnnotation] = "true"

				obj, err = validatingClient.Update(ctx, webhook, metav1.UpdateOptions{})

				return err
			})
			if err != nil {
				a.logger.Error(err, "error updating")
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (a *BootstrapWebhook) addAnnotationToWebhooks(obj metav1.Object, webhooks *WebhooksAvailable, updateFunc func() error) error {
	webhooks.Lock()
	defer webhooks.Unlock()

	if obj.GetLabels() == nil {
		return nil
	}

	name, ok := obj.GetLabels()["olm.webhook-description-generate-name"]

	if !ok {
		return nil
	}

	a.logger.Info("looking up webhook", "name", name)

	if found, ok := webhooks.Webhooks[name]; ok && !found {
		err := retry.RetryOnConflict(retry.DefaultBackoff, updateFunc)
		if err != nil {
			return err
		}

		webhooks.Webhooks[obj.GetName()] = true
	}

	return nil
}

type CRDUpdater struct {
	Logger logr.Logger
	CC     ClientCommandRunner
	Config config.OperatorConfig
	Rest   *rest.Config
	Client kubernetes.Interface
}

func (a *CRDUpdater) NeedLeaderElection() bool {
	return true
}

func (a *CRDUpdater) Start(stop <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a.Logger = a.Logger.WithValues("function", "crdUpdater")

	crdsAvailable := map[string]bool{
		"meterdefinitions.marketplace.redhat.com": false,
	}

	crds := &CRDToUpdate{
		CRDs: crdsAvailable,
	}

	errChan := make(chan error)

	go func() {
		err := a.Run(ctx, crds)
		defer close(errChan)

		if err != nil {
			a.Logger.Error(err, "error running bootstrapWebhook")
			errChan <- err
		}
		cancel()
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errChan:
		return err
	case <-stop:
		cancel()
		return nil
	}
}

type CRDToUpdate struct {
	CRDs map[string]bool
	sync.Mutex
}

const secretName = "redhat-marketplace-controller-manager-service-cert"

func (a *CRDUpdater) secretWatch(ctx context.Context, secretChan chan<- *corev1.Secret) error {
	fieldSelector := fields.AndSelectors(
		fields.OneTermEqualSelector("metadata.name", secretName),
	)
	secretWatcher, err := a.Client.CoreV1().Secrets(a.Config.DeployedNamespace).Watch(
		ctx,
		metav1.ListOptions{
			FieldSelector: fieldSelector.String(),
		})
	defer secretWatcher.Stop()

	if err != nil {
		a.Logger.Error(err, "can't find secret")
		return err
	}

	for {
		select {
		case evt := <-secretWatcher.ResultChan():
			switch evt.Type {
			case watch.Added:
				fallthrough
			case watch.Modified:
				secret := evt.Object.(*corev1.Secret)
				secretChan <- secret
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (a *CRDUpdater) Run(ctx context.Context, crds *CRDToUpdate) error {
	a.Logger.Info("starting")

	secretChannel := make(chan *corev1.Secret)
	defer close(secretChannel)

	go a.secretWatch(ctx, secretChannel)

	cfg := a.Rest
	cfg.WarningHandler = rest.NoWarnings{}
	extendedClient, err := typedapiextensionsv1beta1.NewForConfig(cfg)
	if err != nil {
		a.Logger.Error(err, "error getting typed client")
		return err
	}

	crdWatcher, err := extendedClient.CustomResourceDefinitions().Watch(ctx, metav1.ListOptions{})
	defer crdWatcher.Stop()

	if err != nil {
		a.Logger.Error(err, "error getting crdWatcher")
		return err
	}

	var secret *corev1.Secret

	for {
		select {
		case secret = <-secretChannel:
		case evt := <-crdWatcher.ResultChan():
			switch evt.Type {
			case watch.Added:
				fallthrough
			case watch.Modified:
				if secret == nil {
					continue
				}

				crd := evt.Object.(*apiextensionsv1beta1.CustomResourceDefinition)

				_, found := crds.CRDs[crd.Name]

				if !found {
					continue
				}

				if crd.Spec.Conversion == nil ||
					crd.Spec.Conversion.WebhookClientConfig == nil ||
					crd.Spec.Conversion.WebhookClientConfig.Service == nil {
					continue
				}

				olmCAKey, ok := secret.Data["olmCAKey"]

				if !ok {
					a.Logger.Error(errors.New("olmCAKey missing"), "key not on secret")
					continue
				}

				if bytes.Compare(crd.Spec.Conversion.WebhookClientConfig.CABundle, olmCAKey) == 0 {
					continue
				}

				err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
					fetched, err := extendedClient.CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
					if err != nil {
						return err
					}

					fetched.Spec.Conversion.WebhookClientConfig.CABundle = olmCAKey

					_, err = extendedClient.CustomResourceDefinitions().Update(ctx, fetched, metav1.UpdateOptions{})
					return err
				})

				if err != nil {
					a.Logger.Error(err, "failed to update")
					continue
				}

				a.Logger.Info("updated crd", "name", crd.Name)
				crds.CRDs[crd.Name] = true
			}
		case <-ctx.Done():
			return nil
		}
	}
}
