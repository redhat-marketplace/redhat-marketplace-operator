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

package runnables

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	typedapiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

const (
	OLMOwnerName      = "olm.owner"
	OLMOwnerKind      = "olm.owner.kind"
	OLMOwnerNamespace = "olm.owner.namespace"

	InjectCAAnnotation = "service.beta.openshift.io/inject-cabundle"
)

type CRDUpdater struct {
	Logger  logr.Logger
	CC      ClientCommandRunner
	Config  *config.OperatorConfig
	Rest    *rest.Config
	Client  kubernetes.Interface
	Factory *manifests.Factory

	caInfo *CAInformation `wire:"-"`
}

type CAInformation struct {
	sync.Mutex
	logr.Logger
	configMap *corev1.ConfigMap
	secret    *corev1.Secret
	port      *int32
}

func (c *CAInformation) updateSecret(s *corev1.Secret) {
	c.Lock()
	defer c.Unlock()

	c.secret = s
}

func (c *CAInformation) updateConfigMap(cm *corev1.ConfigMap) {
	c.Lock()
	defer c.Unlock()

	c.configMap = cm
}

func (c *CAInformation) updatePort(port int32) {
	c.Lock()
	defer c.Unlock()

	c.port = &port
}

func (c *CAInformation) GetPort() *int32 {
	c.Lock()
	defer c.Unlock()
	return c.port
}

func (c *CAInformation) GetCA() ([]byte, bool) {
	c.Lock()
	defer c.Unlock()

	if c.configMap == nil && c.secret == nil {
		return []byte{}, false
	}

	openshiftPath := filepath.Join(WebhookCertDir, WebhookKeyName)
	if _, err := os.Stat(openshiftPath); !os.IsNotExist(err) {
		if c.secret != nil {
			olmCAKey, ok := c.secret.Data["olmCAKey"]

			if !ok {
				olmCAKey, ok = c.secret.Data["tls.crt"]
				return olmCAKey, ok
			}

			return olmCAKey, ok
		}

		return []byte{}, false
	}

	if c.configMap != nil {
		olmCAKey, ok := c.configMap.Data["service-ca.crt"]
		return []byte(olmCAKey), ok
	}

	return []byte{}, false
}

func (c *CAInformation) Load(ctx context.Context, a *CRDUpdater) error {
	c.Logger.Info("loading ca info")
	cm := &corev1.ConfigMap{}
	secret := &corev1.Secret{}
	managerService := &corev1.Service{}

	result, _ := a.CC.Exec(ctx,
		reconcileutils.Do(
			reconcileutils.GetAction(types.NamespacedName{
				Name:      serviceName,
				Namespace: a.Config.DeployedNamespace,
			}, managerService),
		))

	if result.Is(reconcileutils.Continue) {
		if len(managerService.Spec.Ports) != 0 {
			a.Logger.Info("setting port")
			port := managerService.Spec.Ports[0].TargetPort.IntVal
			c.updatePort(port)
		}
	}

	result, _ = a.CC.Exec(ctx, reconcileutils.Do(
		reconcileutils.GetAction(types.NamespacedName{
			Name:      secretName,
			Namespace: a.Config.DeployedNamespace,
		}, secret),
	))

	if result.Is(reconcileutils.Continue) {
		c.Logger.Info("setting secret")
		c.updateSecret(secret)
	}

	// get cm for ca
	result, _ = a.CC.Exec(ctx, reconcileutils.Do(
		reconcileutils.GetAction(types.NamespacedName{
			Name:      configMapName,
			Namespace: a.Config.DeployedNamespace,
		}, cm),
	))

	if result.Is(reconcileutils.Continue) {
		c.Logger.Info("setting configmap")
		c.updateConfigMap(cm)
	}

	return nil
}

func (a *CRDUpdater) NeedLeaderElection() bool {
	return true
}

const (
	WebhookCertDir = "/apiserver.local.config/certificates"
	WebhookKeyName = "apiserver.key"
)

func (a *CRDUpdater) Start(stop <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if a.caInfo == nil {
		a.caInfo = &CAInformation{
			Logger: a.Logger.WithValues("struct", "CAInformation"),
		}
	}

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

const operatorDeployment = "redhat-marketplace-controller-manager"
const configMapName = "serving-certs-ca-bundle"
const secretName = "redhat-marketplace-controller-manager-service-cert"
const serviceName = "redhat-marketplace-controller-manager-service"
const meteringServiceName = "redhat-marketplace-controller-manager-metrics-service"

func (a *CRDUpdater) reviewAndUpdateOwnerReferences(
	ctx context.Context,
) error {
	deployment := &appsv1.Deployment{}
	meteringService := &corev1.Service{}
	managerService := &corev1.Service{}

	a.Logger.Info("reviewing owner references")

	result, _ := a.CC.Do(ctx,
		reconcileutils.GetAction(types.NamespacedName{
			Name:      operatorDeployment,
			Namespace: a.Config.DeployedNamespace,
		}, deployment),
		reconcileutils.HandleResult(
			reconcileutils.Do(
				reconcileutils.GetAction(types.NamespacedName{
					Name:      meteringServiceName,
					Namespace: a.Config.DeployedNamespace,
				}, meteringService),
			),
			reconcileutils.OnContinue(reconcileutils.Call(func() (reconcileutils.ClientAction, error) {
				if len(deployment.OwnerReferences) == 0 {
					return nil, nil
				}

				actions := []reconcileutils.ClientAction{}
				owner := deployment.OwnerReferences[0]
				found := func() bool {
					for _, ref := range meteringService.OwnerReferences {
						if reflect.DeepEqual(ref, owner) {
							return true
						}
					}

					return false
				}()

				if !found {
					meteringService.OwnerReferences = append(meteringService.OwnerReferences, owner)
					actions = append(actions,
						reconcileutils.HandleResult(
							reconcileutils.UpdateAction(meteringService),
							reconcileutils.OnRequeue(reconcileutils.ContinueResponse())))
				}

				return reconcileutils.Do(actions...), nil
			})),
		),
		reconcileutils.HandleResult(
			reconcileutils.Do(
				reconcileutils.GetAction(types.NamespacedName{
					Name:      serviceName,
					Namespace: a.Config.DeployedNamespace,
				}, managerService),
			),
			reconcileutils.OnContinue(reconcileutils.Call(func() (reconcileutils.ClientAction, error) {
				if len(deployment.OwnerReferences) == 0 {
					return nil, nil
				}

				actions := []reconcileutils.ClientAction{}
				owner := deployment.OwnerReferences[0]
				found := func() bool {
					for _, ref := range managerService.OwnerReferences {
						if reflect.DeepEqual(ref, owner) {
							return true
						}
					}

					return false
				}()

				if !found {
					managerService.OwnerReferences = append(managerService.OwnerReferences, owner)
					actions = append(actions,
						reconcileutils.HandleResult(
							reconcileutils.UpdateAction(managerService),
							reconcileutils.OnRequeue(reconcileutils.ContinueResponse())))
				}

				return reconcileutils.Do(actions...), nil
			}),
			),
		),
	)

	if result.Is(reconcileutils.Error) {
		a.Logger.Error(result.Err, "failed to update service")
	}

	a.Logger.Info("updated owner references")
	return nil
}

func (a *CRDUpdater) createCMIfMissing(ctx context.Context) error {
	configmap := &corev1.ConfigMap{}
	result, err := a.CC.Exec(ctx, manifests.CreateIfNotExistsFactoryItem(
		configmap,
		func() (runtime.Object, error) {
			return a.Factory.PrometheusServingCertsCABundle()
		},
	))

	if result.Is(Error) {
		a.Logger.Error(err, "failed to create configmap")
	}

	return nil
}

func (a *CRDUpdater) ensureConfigmapExists(
	ctx context.Context,
	crds *CRDToUpdate,
) error {
	cfg := a.Rest
	cfg.WarningHandler = rest.NoWarnings{}
	extendedClient, err := typedapiextensionsv1beta1.NewForConfig(cfg)

	if err != nil {
		a.Logger.Error(err, "failed to get CRD client")
		return err
	}

	workComplete := false

	work := func() error {
		// update service ownerrefs for < 4.5
		err = a.reviewAndUpdateOwnerReferences(ctx)
		if err != nil {
			return err
		}

		// create cm for ca cert
		err = a.createCMIfMissing(ctx)
		if err != nil {
			return err
		}

		// update if necessary
		err = a.caInfo.Load(ctx, a)
		if err != nil {
			return err
		}

		caInfo, ok := a.caInfo.GetCA()
		port := a.caInfo.GetPort()

		if !ok {
			a.Logger.Info("caInfo isn't set")
			return nil
		}

		for crdName := range crds.CRDs {
			a.Logger.Info("updating crd", "name", crdName)

			err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				crd, err := extendedClient.CustomResourceDefinitions().Get(ctx, crdName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				if crd.Spec.Conversion == nil ||
					crd.Spec.Conversion.WebhookClientConfig == nil ||
					crd.Spec.Conversion.WebhookClientConfig.Service == nil {
					return nil
				}

				if crd.Spec.Conversion.WebhookClientConfig.Service.Port == nil ||
					port == nil {
					return nil
				}

				next := true

				if bytes.Compare(crd.Spec.Conversion.WebhookClientConfig.CABundle, caInfo) != 0 {
					next = next && false
				}

				if *crd.Spec.Conversion.WebhookClientConfig.Service.Port != *port {
					next = next && false
				}

				if next {
					return nil
				}

				workComplete = true

				crd.Spec.Conversion.WebhookClientConfig.CABundle = caInfo
				crd.Spec.Conversion.WebhookClientConfig.Service.Port = port
				_, err = extendedClient.CustomResourceDefinitions().Update(ctx, crd, metav1.UpdateOptions{})
				return err
			})

			if err != nil {
				a.Logger.Error(err, "updated crd", "name", crdName)
			} else {
				a.Logger.Info("updated crd", "name", crdName)
			}
		}

		return nil
	}

	// start with a 30 second ticker then drop down to 1 hour once it was successful
	ticker := time.NewTicker(30 * time.Second)
	tickerSetToHour := false
	defer ticker.Stop()

	for {
		a.Logger.Info("starting work")
		err := work()

		if err != nil {
			a.Logger.Error(err, "error doing work")
			return err
		}

		if workComplete && !tickerSetToHour {
			a.Logger.Info("work complete setting to an hour requeue")
			ticker.Stop()
			ticker = time.NewTicker(1 * time.Hour)
			tickerSetToHour = true
		}

		select {
		case <-ctx.Done():
			a.Logger.Info("stopping")
			return nil
		case <-ticker.C:
			continue
		}
	}
}

func (a *CRDUpdater) Run(ctx context.Context, crds *CRDToUpdate) error {
	a.Logger.Info("starting")
	err := a.ensureConfigmapExists(ctx, crds)

	if err != nil {
		a.Logger.Error(err, "error running")
	}

	a.Logger.Info("stopping")
	return err
}
