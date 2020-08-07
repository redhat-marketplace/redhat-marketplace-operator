package metrics

import (
	"context"
	"io"
	"reflect"
	"sync"

	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/meter_definition"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
)

type FamilyByteSlicer interface {
	ByteSlice() []byte
}

type MeterDefinitionFetcher interface {
	GetMeterDefinitions(interface{}) ([]*marketplacev1alpha1.MeterDefinition, error)
}

// MetricsStore implements the k8s.io/client-go/tools/cache.Store
// interface. Instead of storing entire Kubernetes objects, it stores metrics
// generated based on those objects.
type MetricsStore struct {
	// Protects metrics
	mutex sync.RWMutex
	// metrics is a map indexed by Kubernetes object id, containing a slice of
	// metric families, containing a slice of metrics. We need to keep metrics
	// grouped by metric families in order to zip families with their help text in
	// MetricsStore.WriteAll().
	metrics map[types.UID][][]byte
	// headers contains the header (TYPE and HELP) of each metric family. It is
	// later on zipped with with their corresponding metric families in
	// MetricStore.WriteAll().
	headers []string

	// generateMetricsFunc generates metrics based on a given Kubernetes object
	// and returns them grouped by metric family.
	generateMetricsFunc func(interface{}, []*marketplacev1alpha1.MeterDefinition) []FamilyByteSlicer

	meterDefStore *meter_definition.MeterDefinitionStore

	meterDefFetcher MeterDefinitionFetcher

	expectedType interface{}
}

// NewMetricsStore returns a new MetricsStore
func NewMetricsStore(
	headers []string,
	generateFunc func(interface{}, []*marketplacev1alpha1.MeterDefinition) []FamilyByteSlicer,
	meterDefStore *meter_definition.MeterDefinitionStore,
	meterDefFetcher MeterDefinitionFetcher,
	expectedType interface{},
) *MetricsStore {
	return &MetricsStore{
		generateMetricsFunc: generateFunc,
		headers:             headers,
		meterDefFetcher:     meterDefFetcher,
		meterDefStore:       meterDefStore,
		expectedType:        expectedType,
		metrics:             map[types.UID][][]byte{},
	}
}

func (s *MetricsStore) Start(
	ctx context.Context,
) {
	ch := make(chan *meter_definition.ObjectResourceMessage)
	s.meterDefStore.RegisterListener(ch)
	expType := s.expectedType

	go func() {
		defer close(ch)

		for {
			select {
			case msg := <-ch:
				if msg == nil || reflect.TypeOf(msg.Object) != reflect.TypeOf(expType) {
					break
				}

				switch msg.Action {
				case meter_definition.AddMessageAction:
					_ = s.Add(msg.Object)
				case meter_definition.DeleteMessageAction:
					_ = s.Delete(msg.Object)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Implementing k8s.io/client-go/tools/cache.Store interface

// Add inserts adds to the MetricsStore by calling the metrics generator functions and
// adding the generated metrics to the metrics map that underlies the MetricStore.
func (s *MetricsStore) Add(obj interface{}) error {
	o, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	meterDefs, err := s.meterDefFetcher.GetMeterDefinitions(o)

	if err != nil {
		return err
	}

	families := s.generateMetricsFunc(obj, meterDefs)
	familyStrings := make([][]byte, len(families))

	for i, f := range families {
		familyStrings[i] = f.ByteSlice()
	}

	s.metrics[o.GetUID()] = familyStrings

	return nil
}

// Update updates the existing entry in the MetricsStore.
func (s *MetricsStore) Update(obj interface{}) error {
	// TODO: For now, just call Add, in the future one could check if the resource version changed?
	return s.Add(obj)
}

// Delete deletes an existing entry in the MetricsStore.
func (s *MetricsStore) Delete(obj interface{}) error {

	o, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.metrics, o.GetUID())

	return nil
}

// List implements the List method of the store interface.
func (s *MetricsStore) List() []interface{} {
	return nil
}

// ListKeys implements the ListKeys method of the store interface.
func (s *MetricsStore) ListKeys() []string {
	return nil
}

// Get implements the Get method of the store interface.
func (s *MetricsStore) Get(obj interface{}) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

// GetByKey implements the GetByKey method of the store interface.
func (s *MetricsStore) GetByKey(key string) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

// Replace will delete the contents of the store, using instead the
// given list.
func (s *MetricsStore) Replace(list []interface{}, _ string) error {
	s.mutex.Lock()
	s.metrics = map[types.UID][][]byte{}
	s.mutex.Unlock()

	for _, o := range list {
		err := s.Add(o)
		if err != nil {
			return err
		}
	}

	return nil
}

// Resync implements the Resync method of the store interface.
func (s *MetricsStore) Resync() error {
	return nil
}

// WriteAll writes all metrics of the store into the given writer, zipped with the
// help text of each metric family.
func (s *MetricsStore) WriteAll(w io.Writer) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for i, help := range s.headers {
		w.Write([]byte(help))
		w.Write([]byte{'\n'})
		for _, metricFamilies := range s.metrics {
			w.Write(metricFamilies[i])
		}
	}
}
