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

package marketplace

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Remote resource S3 controller", func() {

	const timeout = time.Second * 30
	const interval = time.Second * 1

	var (
		name      = "remoteresources3"
		namespace = "default"
		created   *marketplacev1alpha1.RemoteResourceS3
	)

	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	badRequest := &marketplacev1alpha1.Request{
		StatusCode: 500,
		Message:    "something went wrong",
	}

	correctRequest := &marketplacev1alpha1.Request{
		StatusCode: 200,
		Message:    "No error found",
	}

	defaultRequest := &marketplacev1alpha1.Request{
		StatusCode: 0,
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		created = &marketplacev1alpha1.RemoteResourceS3{
			ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
			Spec:       marketplacev1alpha1.RemoteResourceS3Spec{},
		}
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		Expect(k8sClient.Delete(context.TODO(), created)).Should(Succeed())
	})

	It("should have a good status for a correct request", func() {
		Expect(k8sClient.Create(context.TODO(), created)).Should(Succeed())
		By("Expecting status touched")
		Eventually(func() bool {
			f := &marketplacev1alpha1.RemoteResourceS3{}
			k8sClient.Get(context.TODO(), key, f)
			if f.Status.Touched == nil {
				return false
			}
			return *f.Status.Touched
		}, timeout, interval).Should(BeTrue())
		By("Expecting status conditionFalse for no requests")
		Eventually(func() *status.Condition {
			f := &marketplacev1alpha1.RemoteResourceS3{}
			k8sClient.Get(context.TODO(), key, f)
			return f.Status.Conditions.GetCondition(marketplacev1alpha1.ResourceInstallError)
		}, timeout, interval).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Status":  Equal(corev1.ConditionFalse),
			"Message": Equal(correctRequest.Message),
			"Reason":  Equal(marketplacev1alpha1.NoBadRequest),
		})))

		k8sClient.Get(context.TODO(), key, created)
		created.Spec.Requests = []marketplacev1alpha1.Request{*defaultRequest}
		Expect(k8sClient.Update(context.TODO(), created)).Should(Succeed())
		By("Expecting status conditionFalse for default request")
		Eventually(func() *status.Condition {
			f := &marketplacev1alpha1.RemoteResourceS3{}
			k8sClient.Get(context.TODO(), key, f)
			return f.Status.Conditions.GetCondition(marketplacev1alpha1.ResourceInstallError)
		}, timeout, interval).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Status":  Equal(corev1.ConditionFalse),
			"Message": Equal(correctRequest.Message),
			"Reason":  Equal(marketplacev1alpha1.NoBadRequest),
		})))

		k8sClient.Get(context.TODO(), key, created)
		created.Spec.Requests = []marketplacev1alpha1.Request{*correctRequest}
		Expect(k8sClient.Update(context.TODO(), created)).Should(Succeed())
		By("Expecting status conditionFalse for correct request")
		Eventually(func() *status.Condition {
			f := &marketplacev1alpha1.RemoteResourceS3{}
			k8sClient.Get(context.TODO(), key, f)
			return f.Status.Conditions.GetCondition(marketplacev1alpha1.ResourceInstallError)
		}, timeout, interval).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Status":  Equal(corev1.ConditionFalse),
			"Message": Equal(correctRequest.Message),
			"Reason":  Equal(marketplacev1alpha1.NoBadRequest),
		})))

		k8sClient.Get(context.TODO(), key, created)
		created.Spec.Requests = []marketplacev1alpha1.Request{*badRequest}
		Expect(k8sClient.Update(context.TODO(), created)).Should(Succeed())
		By("Expecting status conditionTrue for bad request")
		Eventually(func() *status.Condition {
			f := &marketplacev1alpha1.RemoteResourceS3{}
			k8sClient.Get(context.TODO(), key, f)
			return f.Status.Conditions.GetCondition(marketplacev1alpha1.ResourceInstallError)
		}, timeout, interval).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Status":  Equal(corev1.ConditionTrue),
			"Message": Equal(badRequest.Message),
			"Reason":  Equal(marketplacev1alpha1.FailedRequest),
		})))

		k8sClient.Get(context.TODO(), key, created)
		created.Spec.Requests = []marketplacev1alpha1.Request{*correctRequest, *badRequest}
		Expect(k8sClient.Update(context.TODO(), created)).Should(Succeed())
		By("Expecting status ConditionTrue for correct and bad request")
		Eventually(func() *status.Condition {
			f := &marketplacev1alpha1.RemoteResourceS3{}
			k8sClient.Get(context.TODO(), key, f)
			return f.Status.Conditions.GetCondition(marketplacev1alpha1.ResourceInstallError)
		}, timeout, interval).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Status":  Equal(corev1.ConditionTrue),
			"Message": Equal(badRequest.Message),
			"Reason":  Equal(marketplacev1alpha1.FailedRequest),
		})))
	})
})
