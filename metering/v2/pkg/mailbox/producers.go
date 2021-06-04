package mailbox

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/dictionary"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/meterdefinition"
	"k8s.io/client-go/tools/cache"
)

type MailboxChannelProducer struct {
	queue   cache.Queue
	mailbox *Mailbox
	log     logr.Logger
	name    ChannelName
}

type ObjectChannelProducer struct {
	MailboxChannelProducer
}

func ProvideObjectChannelProducer(
	meterDefinitionStore *meterdefinition.MeterDefinitionStore,
	mb *Mailbox,
	log logr.Logger,
) *ObjectChannelProducer {
	return &ObjectChannelProducer{
		MailboxChannelProducer: MailboxChannelProducer{
			queue:   meterDefinitionStore,
			mailbox: mb,
			log:     log,
			name:    ObjectChannel,
		},
	}
}

type MeterDefinitionChannelProducer struct {
	MailboxChannelProducer
}

func ProvideMeterDefinitionChannelProducer(
	dictionary *dictionary.MeterDefinitionDictionary,
	mb *Mailbox,
	log logr.Logger,
) *MeterDefinitionChannelProducer {
	return &MeterDefinitionChannelProducer{
		MailboxChannelProducer: MailboxChannelProducer{
			queue:   dictionary,
			mailbox: mb,
			log:     log,
			name:    MeterDefinitionChannel,
		},
	}
}

// Start will start processing pops from the dictionary. Will block
func (w *MailboxChannelProducer) Start(ctx context.Context) error {
	go func() {
		select {
		case <-ctx.Done():
			return
		default:
			w.queue.Pop(w.handlePop)
		}
	}()

	return nil
}

func (w *MailboxChannelProducer) handlePop(i interface{}) error {
	delt, ok := i.(cache.Delta)
	if !ok {
		return errors.New("obj is not a delta")
	}
	w.mailbox.Broadcast(w.name, delt)
	return nil
}
