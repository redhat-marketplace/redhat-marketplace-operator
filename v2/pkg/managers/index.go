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

package managers

import (
	"context"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/api/marketplace/v1beta1"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func AddIndices(
	ctx context.Context,
	cache cache.Cache) (CacheIsIndexed, error) {
	err := rhmclient.AddOperatorSourceIndex(cache)
	if err != nil {
		log.Error(err, "")
		return CacheIsIndexed{}, err
	}

	err = rhmclient.AddOwningControllerIndex(cache,
		[]client.Object{
			&corev1.Pod{},
			&corev1.Service{},
			&corev1.PersistentVolumeClaim{},
			&monitoringv1.ServiceMonitor{},
		})

	if err != nil {
		log.Error(err, "")
		return CacheIsIndexed{}, err
	}

	err = rhmclient.AddUIDIndex(cache,
		[]client.Object{
			&corev1.Pod{},
			&corev1.Service{},
			&corev1.PersistentVolumeClaim{},
			&marketplacev1beta1.MeterDefinition{},
			&monitoringv1.ServiceMonitor{},
		})

	if err != nil {
		log.Error(err, "")
		return CacheIsIndexed{}, err
	}

	return CacheIsIndexed{}, nil
}
