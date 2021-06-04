package metrics

import (
	"fmt"
	"io"
	"reflect"
	"sync"

	"emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type PrometheusData []*PrometheusDataMap

func (p PrometheusData) Add(obj interface{}, meterdefs []*v1beta1.MeterDefinition) error {
	foundOne := false

	for _, dm := range p {
		if dm.IsExpectedType(obj) {
			foundOne = true
			err := dm.Add(obj, meterdefs)
			if err != nil {
				return err
			}
		}
	}

	if !foundOne {
		return errors.New(fmt.Sprintf("expected type %T not found", obj))
	}

	return nil
}

func (p PrometheusData) Remove(obj interface{}) error {
	for _, dm := range p {
		if dm.IsExpectedType(obj) {
			err := dm.Remove(obj)

			if err != nil {
				return err
			}
		}
	}

	return nil
}

type PrometheusDataMap struct {
	sync.RWMutex
	metrics map[types.UID][][]byte

	expectedType        reflect.Type
	headers             []string
	generateMetricsFunc func(interface{}, []*marketplacev1beta1.MeterDefinition) []FamilyByteSlicer
}

func (s *PrometheusDataMap) Remove(obj interface{}) error {
	s.Lock()
	defer s.Unlock()

	metaObj, ok := obj.(metav1.Object)

	if !ok {
		return errors.New("object does not conform to metav1.Object")
	}

	delete(s.metrics, metaObj.GetUID())
	return nil
}

func (s *PrometheusDataMap) IsExpectedType(obj interface{}) bool {
	thisType := reflect.TypeOf(obj)
	return thisType == s.expectedType
}

func (s *PrometheusDataMap) Add(obj interface{}, meterdefs []*v1beta1.MeterDefinition) error {
	s.Lock()
	defer s.Unlock()

	if !s.IsExpectedType(obj) {
		thisType := reflect.TypeOf(obj)
		return errors.NewWithDetails("unexpected type",
			"type", thisType,
			"expectedType", s.expectedType)
	}

	families := s.generateMetricsFunc(obj, meterdefs)
	familyStrings := make([][]byte, len(families))

	for i, f := range families {
		familyStrings[i] = f.ByteSlice()
	}

	metaObj, ok := obj.(metav1.Object)

	if !ok {
		return errors.New("object does not conform to metav1.Object")
	}

	s.metrics[metaObj.GetUID()] = familyStrings
	return nil
}

// WriteAll writes all metrics of the store into the given writer, zipped with the
// help text of each metric family.
func (s *PrometheusDataMap) WriteAll(w io.Writer) {
	s.RLock()
	defer s.RUnlock()

	for i, help := range s.headers {
		w.Write([]byte(help))
		w.Write([]byte{'\n'})
		for _, metricFamilies := range s.metrics {
			w.Write(metricFamilies[i])
		}
	}
}

func ProvidePrometheusData() PrometheusData {
	return PrometheusData{
		ProvidePodPrometheusData(),
		ProvideServicePrometheusData(),
		ProvidePersistentVolumeClaimPrometheusData(),
		ProvideMeterDefPrometheusData(),
	}
}
