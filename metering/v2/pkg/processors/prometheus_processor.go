package processors

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/internal/metrics"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/mailbox"
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
	mutex          sync.Mutex
	scheme         *runtime.Scheme
	prometheusData metrics.PrometheusData
}

// NewPrometheusProcessor is the provider that creates
// the processor.
func ProvidePrometheusProcessor(
	log logr.Logger,
	kubeClient client.Client,
	mb *mailbox.Mailbox,
	scheme *runtime.Scheme,
	prometheusData metrics.PrometheusData,
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

	u.mutex.Lock()
	defer u.mutex.Unlock()

	switch inObj.Type {
	case cache.Deleted:
		if err := u.prometheusData.Remove(inObj.Object); err != nil {
			u.log.Error(err, "error deleteing obj to prometheus")
			return err
		}
	case cache.Replaced:
		fallthrough
	case cache.Sync:
		fallthrough
	case cache.Updated:
		fallthrough
	case cache.Added:
		if err := u.prometheusData.Add(inObj.Object, nil); err != nil {
			u.log.Error(err, "error adding obj to prometheus")
			return err
		}
	default:
		return nil
	}

	return nil
}
