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

package metrics

import (
	"fmt"
	"io"
	"reflect"
	"sort"

	"emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/sasha-s/go-deadlock"
	"k8s.io/client-go/tools/cache"
)

type PrometheusData struct {
	dataMap map[string]*PrometheusDataMap

	orderedLabels []string
}

func (p *PrometheusData) Get(name string) *PrometheusDataMap {
	return p.dataMap[name]
}

func (p *PrometheusData) WriteAll(w io.Writer) {
	if len(p.orderedLabels) != len(p.dataMap) {
		keys := make([]string, len(p.dataMap))

		i := 0
		for k := range p.dataMap {
			keys[i] = k
			i++
		}

		sorted := sort.StringSlice(keys)
		sorted.Sort()

		p.orderedLabels = sorted
	}

	for k := range p.orderedLabels {
		p.dataMap[p.orderedLabels[k]].WriteAll(w)
	}
}

func (p *PrometheusData) Add(obj interface{}, meterdefs []v1beta1.MeterDefinition) error {
	foundOne := false

	for k := range p.dataMap {
		dm := p.dataMap[k]
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

func (p *PrometheusData) Remove(obj interface{}) error {
	for k := range p.dataMap {
		dm := p.dataMap[k]
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
	deadlock.RWMutex
	metrics map[string][][]byte

	expectedType        reflect.Type
	headers             []string
	generateMetricsFunc func(interface{}, []marketplacev1beta1.MeterDefinition) []FamilyByteSlicer
}

func (s *PrometheusDataMap) Remove(obj interface{}) error {
	s.Lock()
	defer s.Unlock()

	key, err := cache.MetaNamespaceKeyFunc(obj)

	if err != nil {
		return err
	}

	delete(s.metrics, key)
	return nil
}

func (s *PrometheusDataMap) IsExpectedType(obj interface{}) bool {
	thisType := reflect.TypeOf(obj)
	return thisType == s.expectedType
}

func (s *PrometheusDataMap) Add(obj interface{}, meterdefs []v1beta1.MeterDefinition) error {
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

	key, err := cache.MetaNamespaceKeyFunc(obj)

	if err != nil {
		return err
	}

	s.metrics[key] = familyStrings
	return nil
}

func (s *PrometheusDataMap) Get(obj interface{}) ([][]byte, bool, error) {
	s.RLock()
	defer s.RUnlock()

	key, err := cache.MetaNamespaceKeyFunc(obj)

	if err != nil {
		return nil, false, err
	}

	val, ok := s.metrics[key]
	return val, ok, nil
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

func ProvidePrometheusData() *PrometheusData {
	return &PrometheusData{
		dataMap: map[string]*PrometheusDataMap{
			"pod":      ProvidePodPrometheusData(),
			"service":  ProvideServicePrometheusData(),
			"pvc":      ProvidePersistentVolumeClaimPrometheusData(),
			"meterdef": ProvideMeterDefPrometheusData(),
		},
	}
}
