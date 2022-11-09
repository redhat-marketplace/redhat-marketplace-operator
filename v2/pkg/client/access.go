package client

import (
	context "context"
	"fmt"

	"github.com/pkg/errors"
	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

var AccessDeniedErr error = errors.New("rhm operator does not have access to object")

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

func (a *AccessChecker) CheckAccess(group string, version string, kind string) (bool, error) {
	sar := &authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			User: "system:serviceaccount:openshift-redhat-marketplace:redhat-marketplace-metric-state",
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
		return false, fmt.Errorf("cannot list or watch Kind: %s, group: %s, version: %s: %w", kind, group, version, AccessDeniedErr)
	}

	return true, nil
}
