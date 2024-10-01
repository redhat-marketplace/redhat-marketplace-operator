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
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func (a *AccessChecker) CheckAccess(gvr schema.GroupVersionResource) (bool, error) {

	reqAttrs := []authv1.ResourceAttributes{
		{Resource: gvr.Resource, Group: gvr.Group, Version: gvr.Version, Verb: "list"},
		{Resource: gvr.Resource, Group: gvr.Group, Version: gvr.Version, Verb: "get"},
		{Resource: gvr.Resource, Group: gvr.Group, Version: gvr.Version, Verb: "watch"},
	}

	for _, reqAttr := range reqAttrs {
		sar := &authv1.SelfSubjectAccessReview{
			Spec: authv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &reqAttr,
			},
		}

		review, err := a.kubeClient.AuthorizationV1().SelfSubjectAccessReviews().Create(a.ctx, sar, metav1.CreateOptions{})
		if err != nil {
			return false, err
		}

		if !review.Status.Allowed {
			return false, fmt.Errorf("%w serviceaccount/%s does not have get/list/watch access to Resource: %s, Group: %s, Version: %s via clusterrole/view. Create a clusterrole with get/list/watch access and bind it to serviceaccount/%s, or create a clusterrole with get/list/watch access and add the annotation rbac.authorization.k8s.io/aggregate-to-view: 'true' to add access to clusterrole/view", AccessDeniedErr, utils.METRIC_STATE_SERVICE_ACCOUNT, gvr.Resource, gvr.Group, gvr.Version, utils.METRIC_STATE_SERVICE_ACCOUNT)
		}
	}

	return true, nil
}
