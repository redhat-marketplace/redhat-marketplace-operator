package client

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	timer := time.NewTimer(a.retryTime)

	for {
		select {
		case <-timer.C:
			list, err := a.resourceClient.Namespace(a.namespace).List(context.Background(), metav1.ListOptions{})
			if errors.IsUnauthorized(err) {
				log.Error(err, "list call is unauthorized")
				return err
			}
			log.Info("retrieved list", "len", len(list.Items))
		case <-ctx.Done():
			timer.Stop()
			return nil
		}
	}
}
