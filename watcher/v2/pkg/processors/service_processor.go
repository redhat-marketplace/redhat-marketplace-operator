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

package processors

import (
	"context"
	"reflect"
	"time"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/mailbox"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceProcessor will update the meter definition
// status with the objects that matched it.
type ServiceAnnotatorProcessor struct {
	*Processor
	log        logr.Logger
	kubeClient client.Client
}

// NewServiceProcessor is the provider that creates
// the processor.
func ProvideServiceAnnotatorProcessor(
	log logr.Logger,
	kubeClient client.Client,
	mb *mailbox.Mailbox,
) *ServiceAnnotatorProcessor {
	sp := &ServiceAnnotatorProcessor{
		Processor: &Processor{
			log:           log,
			digestersSize: 1,
			retryCount:    3,
			mailbox:       mb,
			channelName:   mailbox.ObjectChannel,
		},
		log:        log.WithValues("process", "serviceAnnotatorProcessor"),
		kubeClient: kubeClient,
	}

	sp.Processor.DeltaProcessor = sp
	return sp
}

var serviceType = reflect.TypeOf(&corev1.Service{})

// Process will receive a new ObjectResourceMessage and find and update the metere
// definition associated with the object. To prevent gaps, it bulk retrieves the
// resources and checks it against the status.
func (w *ServiceAnnotatorProcessor) Process(ctx context.Context, inObj cache.Delta) error {

	if inObj.Type != cache.Added {
		return nil
	}

	obj, ok := inObj.Object.(*corev1.Service)

	if !ok {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	list := monitoringv1.ServiceMonitorList{}
	err := w.kubeClient.List(ctx, &list, client.MatchingFields{
		rhmclient.IndexOwnerRefContains: string(obj.UID),
	})

	if err != nil && k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return errors.WithStack(err)
	}

	for i := range list.Items {
		retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			sm := list.Items[i]
			key := client.ObjectKeyFromObject(sm)

			serviceMonitor := monitoringv1.ServiceMonitor{}
			err := w.kubeClient.Get(ctx, key, &serviceMonitor)

			if err != nil {
				return errors.WithStack(err)
			}

			if utils.HasMapKey(serviceMonitor.ObjectMeta.Labels, utils.MeteredAnnotation) {
				return nil
			}

			if serviceMonitor.ObjectMeta.Labels == nil {
				serviceMonitor.ObjectMeta.Labels = make(map[string]string)
			}

			utils.SetMapKeyValue(serviceMonitor.ObjectMeta.Labels, utils.MeteredAnnotation)

			w.log.Info("found servicemonitor to label", "sm", serviceMonitor, "labels", sm.Labels)
			return w.kubeClient.Update(ctx, &serviceMonitor)
		})
	}

	return nil
}
