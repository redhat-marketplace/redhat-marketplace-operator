// Copyright 2022 IBM Corp.
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
	context "context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

var AccessDeniedErr error = errors.New("AccessDeniedError")

type AccessChecker struct {
	kubeClient clientset.Interface
	ctx        context.Context
}

func NewAccessChecker(kubeClientSet clientset.Interface, ctx context.Context) AccessChecker {
	return AccessChecker{
		kubeClient: kubeClientSet,
		ctx:        ctx,
	}
}

func (a *AccessChecker) CheckAccess(group string, version string, kind string, namespace string) (bool, error) {
	sar := &authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			User: "system:serviceaccount:" + namespace + ":" + utils.METRIC_STATE_SERVICE_ACCOUNT,
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:     "list,watch",
				Group:    group,
				Resource: kind,
				Version:  version,
			},
		},
	}

	opts := metav1.CreateOptions{}
	review, err := a.kubeClient.AuthorizationV1().SubjectAccessReviews().Create(a.ctx, sar, opts)
	if err != nil {
		return false, err
	}

	if !review.Status.Allowed {
		return false, fmt.Errorf("%w serviceaccount/%s does not have get/list/watch access to Kind: %s, group: %s, version: %s via clusterrole/view. Create a clusterrole with get/list/watch access and bind it to serviceaccount/%s, or create a clusterrole with get/list/watch access and add the annotation rbac.authorization.k8s.io/aggregate-to-view: 'true' to add access to clusterrole/view", AccessDeniedErr, utils.METRIC_STATE_SERVICE_ACCOUNT, kind, group, version, utils.METRIC_STATE_SERVICE_ACCOUNT)
	}

	return true, nil
}
