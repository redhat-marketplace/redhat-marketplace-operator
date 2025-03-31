// Copyright 2024 IBM Corp.
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

// From: https://github.com/kubernetes-sigs/controller-runtime/tree/v0.18.0/pkg/metrics/filters

package filters

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	emperror "emperror.dev/errors"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/authenticatorfactory"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	authenticationv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	authorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"

	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// WithAuthenticationAndAuthorization provides a metrics.Filter for authentication and authorization.
// Metrics will be authenticated (via TokenReviews) and authorized (via SubjectAccessReviews) with the
// kube-apiserver.
// For the authentication and authorization the controller needs a ClusterRole
// with the following rules:
// * apiGroups: authentication.k8s.io, resources: tokenreviews, verbs: create
// * apiGroups: authorization.k8s.io, resources: subjectaccessreviews, verbs: create
//
// To scrape metrics e.g. via Prometheus the client needs a ClusterRole
// with the following rule:
// * nonResourceURLs: "/metrics", verbs: get
//
// Note: Please note that configuring this metrics provider will introduce a dependency to "k8s.io/apiserver"
// to your go module.
func WithAuthenticationAndAuthorization(config *rest.Config, httpClient *http.Client) (metricsserver.Filter, error) {
	authenticationV1Client, err := authenticationv1.NewForConfigAndClient(config, httpClient)
	if err != nil {
		return nil, err
	}
	authorizationV1Client, err := authorizationv1.NewForConfigAndClient(config, httpClient)
	if err != nil {
		return nil, err
	}

	authenticatorConfig := authenticatorfactory.DelegatingAuthenticatorConfig{
		Anonymous:                false, // Require authentication.
		CacheTTL:                 1 * time.Minute,
		TokenAccessReviewClient:  authenticationV1Client,
		TokenAccessReviewTimeout: 10 * time.Second,
		// wait.Backoff is copied from: https://github.com/kubernetes/apiserver/blob/v0.29.0/pkg/server/options/authentication.go#L43-L50
		// options.DefaultAuthWebhookRetryBackoff is not used to avoid a dependency on "k8s.io/apiserver/pkg/server/options".
		WebhookRetryBackoff: &wait.Backoff{
			Duration: 500 * time.Millisecond,
			Factor:   1.5,
			Jitter:   0.2,
			Steps:    5,
		},
	}
	delegatingAuthenticator, _, err := authenticatorConfig.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticator: %w", err)
	}

	authorizerConfig := authorizerfactory.DelegatingAuthorizerConfig{
		SubjectAccessReviewClient: authorizationV1Client,
		AllowCacheTTL:             5 * time.Minute,
		DenyCacheTTL:              30 * time.Second,
		// wait.Backoff is copied from: https://github.com/kubernetes/apiserver/blob/v0.29.0/pkg/server/options/authentication.go#L43-L50
		// options.DefaultAuthWebhookRetryBackoff is not used to avoid a dependency on "k8s.io/apiserver/pkg/server/options".
		WebhookRetryBackoff: &wait.Backoff{
			Duration: 500 * time.Millisecond,
			Factor:   1.5,
			Jitter:   0.2,
			Steps:    5,
		},
	}
	delegatingAuthorizer, err := authorizerConfig.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create authorizer: %w", err)
	}

	return func(log logr.Logger, handler http.Handler) (http.Handler, error) {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()

			_, ok := req.Header["Authorization"]
			if ok {
			} else {

				eventKeyBytes, err := io.ReadAll(req.Body)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					log.Error(err, "error reading request body")
					return
				}

				if !json.Valid(eventKeyBytes) {

					w.WriteHeader(http.StatusBadRequest)
					err = emperror.New("event is not valid json")
					log.Error(err, "error validating event json")
					return
				}

				var result map[string]interface{}
				json.Unmarshal(eventKeyBytes, &result)

				fmt.Printf("AAAAA\n")
				fmt.Printf("%s\n", eventKeyBytes)
				fmt.Printf("BBBBB\n")
				fmt.Printf("%s\n", result["writeKey"])

				// rawMessage := json.RawMessage(eventKeyBytes)

			}

			res, ok, err := delegatingAuthenticator.AuthenticateRequest(req)
			if err != nil {
				log.Error(err, "Authentication failed")
				http.Error(w, "Authentication failed", http.StatusInternalServerError)
				return
			}
			if !ok {
				log.V(4).Info("Authentication failed")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			attributes := authorizer.AttributesRecord{
				User: res.User,
				Verb: strings.ToLower(req.Method),
				Path: req.URL.Path,
			}

			authorized, reason, err := delegatingAuthorizer.Authorize(ctx, attributes)
			if err != nil {
				msg := fmt.Sprintf("Authorization for user %s failed", attributes.User.GetName())
				log.Error(err, msg)
				http.Error(w, msg, http.StatusInternalServerError)
				return
			}
			if authorized != authorizer.DecisionAllow {
				msg := fmt.Sprintf("Authorization denied for user %s", attributes.User.GetName())
				log.V(4).Info(fmt.Sprintf("%s: %s", msg, reason))
				http.Error(w, msg, http.StatusForbidden)
				return
			}

			// Provide kube-rbac-proxy functionality of identity for next handler
			// https://github.com/brancz/kube-rbac-proxy/blob/v0.17.0/pkg/filters/auth.go#L117
			req.Header.Set("x-remote-user", attributes.User.GetName())
			req.Header.Set("x-remote-groups", strings.Join(attributes.User.GetGroups(), "|"))

			handler.ServeHTTP(w, req)
		}), nil
	}, nil
}
