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

package client

import (
	"context"
	"time"

	emperrors "emperror.dev/errors"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

type AuthChecker struct {
	resourceClient dynamic.NamespaceableResourceInterface
	retryTime      time.Duration
	namespace      string
	logger         logr.Logger
}

type AuthCheckerConfig struct {
	Group, Kind, Version string
	RetryTime            time.Duration
	Namespace            string
}

func NewAuthChecker(
	logger logr.Logger,
	dynamicClient *DynamicClient,
	config AuthCheckerConfig,
) (*AuthChecker, error) {
	resourceClient, err := dynamicClient.ClientForKind(schema.GroupKind{
		Group: config.Group,
		Kind:  config.Kind,
	}, config.Version)

	if err != nil {
		return nil, err
	}

	checker := &AuthChecker{
		resourceClient: resourceClient,
		retryTime:      config.RetryTime,
		namespace:      config.Namespace,
		logger:         logger,
	}

	return checker, nil
}

func (a *AuthChecker) Run(ctx context.Context) error {
	ticker := time.NewTicker(a.retryTime)
	defer ticker.Stop()
	log := a.logger

	log.Info("starting to monitor", "namespace", a.namespace)

	resourceWatch, err := a.resourceClient.Namespace(a.namespace).Watch(ctx, metav1.ListOptions{})

	if err != nil {
		if errors.IsUnauthorized(err) {
			log.Error(err, "list call is unauthorized")
			return err
		}
		log.Error(err, "list call errored")
		return err
	}

	defer resourceWatch.Stop()

	for {
		select {
		case evt := <- resourceWatch.ResultChan():
			if evt.Type == watch.Error {
				obj, ok := evt.Object.(*metav1.Status)
				if !ok {
					err := emperrors.NewWithDetails("watch returned an error", "evt.Object", evt.Object)
					log.Error(err, "unknown error")
				}

				if obj.Status == metav1.StatusFailure && obj.Reason == metav1.StatusReasonUnauthorized {
					err := emperrors.NewWithDetails("watch returned an unauthorized error", "status", obj)
					log.Error(err, "list call is unauthorized")
					return err
				}
			} else {
				log.V(5).Info("see an event", "type", evt.Type)
			}
		case <-ctx.Done():
			return nil
		}
	}
}