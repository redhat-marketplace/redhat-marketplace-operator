package meter_definition

import (
	"context"
	"reflect"
	"sort"

	"github.com/go-logr/logr"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
)

// StatusProcessor will update the meter definition
// status with the objects that matched it.
type StatusProcessor struct {
	log           logr.Logger
	cc            ClientCommandRunner
	meterDefStore *MeterDefinitionStore
}

// NewStatusProcessor is the provider that creates
// the processor.
func NewStatusProcessor(
	log logr.Logger,
	cc ClientCommandRunner,
	meterDefStore *MeterDefinitionStore,
) *StatusProcessor {
	return &StatusProcessor{
		log:           log,
		cc:            cc,
		meterDefStore: meterDefStore,
	}
}

// Start will register it's listener and execute the function.
func (u *StatusProcessor) Start(ctx context.Context) error {
	p := NewProcessor(u.log, u.cc, u.meterDefStore, u)
	return p.Start(ctx)
}

// Process will receive a new ObjectResourceMessage and find and update the metere
// definition associated with the object. To prevent gaps, it bulk retrieves the
// resoruces and checks it against the status.
func (u *StatusProcessor) Process(ctx context.Context, inObj *ObjectResourceMessage) error {
	log := u.log.WithValues("process", "statusProcessor")
	mdef := &marketplacev1alpha1.MeterDefinition{}

	if inObj == nil {
		return nil
	}

	if inObj.Action == DeleteMessageAction {
		return nil
	}

	result, _ := u.cc.Do(ctx,
		HandleResult(
			GetAction(inObj.MeterDef, mdef),
			OnContinue(Call(func() (ClientAction, error) {
				objs := u.meterDefStore.GetMeterDefObjects(mdef.UID)
				log.Info("found objs", "objs", len(objs))

				updatedMeterDef := mdef.DeepCopy()
				resources := make([]marketplacev1alpha1.WorkloadResource, 0, len(objs))

				for _, obj := range objs {
					resource := obj.WorkloadResource
					resources = append(resources, *resource)
				}

				sort.Sort(marketplacev1alpha1.ByAlphabetical(resources))
				updatedMeterDef.Status.WorkloadResources = resources

				if reflect.DeepEqual(updatedMeterDef.Status, mdef.Status) {
					return nil, nil
				}

				return UpdateAction(updatedMeterDef, UpdateStatusOnly(true)), nil
			})),
		),
	)

	if result.Is(NotFound) {
		return nil
	}

	if result.Is(Error) {
		return result
	}

	return nil
}
