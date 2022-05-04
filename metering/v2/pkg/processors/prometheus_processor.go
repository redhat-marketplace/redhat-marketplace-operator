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

package processors

import (
	"context"
	"fmt"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/internal/metrics"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/dictionary"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/mailbox"
	pkgtypes "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	"github.com/sasha-s/go-deadlock"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PrometheusProcessor will update the meter definition
// status with the objects that matched it.
type PrometheusProcessor struct {
	*Processor
	log            logr.Logger
	kubeClient     client.Client
	mutex          deadlock.Mutex
	scheme         *runtime.Scheme
	prometheusData *metrics.PrometheusData
}

// NewPrometheusProcessor is the provider that creates
// the processor.
func ProvidePrometheusProcessor(
	log logr.Logger,
	kubeClient client.Client,
	mb *mailbox.Mailbox,
	scheme *runtime.Scheme,
	prometheusData *metrics.PrometheusData,
) *PrometheusProcessor {
	sp := &PrometheusProcessor{
		Processor: &Processor{
			log:           log,
			digestersSize: 1,
			retryCount:    3,
			mailbox:       mb,
			channelName:   mailbox.ObjectChannel,
		},
		log:            log.WithValues("process", "prometheusProcessor"),
		kubeClient:     kubeClient,
		scheme:         scheme,
		prometheusData: prometheusData,
	}

	sp.Processor.DeltaProcessor = sp
	return sp
}

// Process will receive a new ObjectResourceMessage and find and update the metere
// definition associated with the object. To prevent gaps, it bulk retrieves the
// resources and checks it against the status.
func (u *PrometheusProcessor) Process(ctx context.Context, inObj cache.Delta) error {
	if inObj.Object == nil {
		return nil
	}

	obj, ok := inObj.Object.(*pkgtypes.MeterDefinitionEnhancedObject)

	if !ok {
		err := errors.New("object is not expected type")
		u.log.Error(err, "failed to process", "type", fmt.Sprintf("%T", inObj.Object))
		return nil
	}

	metaobj, _ := meta.Accessor(obj.Object)

	u.mutex.Lock()
	defer u.mutex.Unlock()

	switch inObj.Type {
	case cache.Deleted:
		if err := u.prometheusData.Remove(obj.Object); err != nil {
			u.log.Error(err, "error deleting obj to prometheus")
			return errors.WithStack(err)
		}
	case cache.Replaced:
		fallthrough
	case cache.Sync:
		fallthrough
	case cache.Updated:
		fallthrough
	case cache.Added:
		u.log.Info("object added", "object", metaobj.GetUID(), "meterdefs", len(obj.MeterDefinitions))
		if err := u.prometheusData.Add(obj.Object, obj.MeterDefinitions); err != nil {
			u.log.Error(err, "error adding obj to prometheus")
			return errors.WithStack(err)
		}
	default:
		return nil
	}

	return nil
}

// PrometheusMdefProcessor will update the meter definition
// status with the objects that matched it.
type PrometheusMdefProcessor struct {
	*Processor
	log            logr.Logger
	kubeClient     client.Client
	mutex          deadlock.Mutex
	scheme         *runtime.Scheme
	prometheusData *metrics.PrometheusData
}

// NewPrometheusMdefProcessor is the provider that creates
// the processor.
func ProvidePrometheusMdefProcessor(
	log logr.Logger,
	kubeClient client.Client,
	mb *mailbox.Mailbox,
	scheme *runtime.Scheme,
	prometheusData *metrics.PrometheusData,
) *PrometheusMdefProcessor {
	sp := &PrometheusMdefProcessor{
		Processor: &Processor{
			log:           log,
			digestersSize: 1,
			retryCount:    3,
			mailbox:       mb,
			channelName:   mailbox.MeterDefinitionChannel,
		},
		log:            log.WithValues("process", "prometheusProcessor"),
		kubeClient:     kubeClient,
		scheme:         scheme,
		prometheusData: prometheusData,
	}

	sp.Processor.DeltaProcessor = sp
	return sp
}

// Process will receive a new ObjectResourceMessage and find and update the metere
// definition associated with the object. To prevent gaps, it bulk retrieves the
// resources and checks it against the status.
func (u *PrometheusMdefProcessor) Process(ctx context.Context, inObj cache.Delta) error {
	if inObj.Object == nil {
		return nil
	}

	meterdef, ok := inObj.Object.(*dictionary.MeterDefinitionExtended)

	if !ok {
		return errors.New("encountered unexpected type")
	}

	u.mutex.Lock()
	defer u.mutex.Unlock()

	switch inObj.Type {
	case cache.Deleted:
		if err := u.prometheusData.Remove(&meterdef.MeterDefinition); err != nil {
			u.log.Error(err, "error deleting obj from prometheus")
			return errors.WithStack(err)
		}
	case cache.Replaced:
		fallthrough
	case cache.Sync:
		fallthrough
	case cache.Updated:
		// Flush the prometheus data when a MeterDefinition is updated
		if err := u.prometheusData.Remove(&meterdef.MeterDefinition); err != nil {
			u.log.Error(err, "error deleting obj from prometheus")
			return errors.WithStack(err)
		}
		fallthrough
	case cache.Added:
		if err := u.prometheusData.Add(&meterdef.MeterDefinition, nil); err != nil {
			u.log.Error(err, "error adding obj to prometheus")
			return errors.WithStack(err)
		}
	default:
		return nil
	}

	return nil
}
