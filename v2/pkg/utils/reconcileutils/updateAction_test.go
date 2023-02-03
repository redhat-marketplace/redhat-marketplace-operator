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

package reconcileutils

import (
	"context"

	"emperror.dev/errors"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	utilspatch "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/tests/mock/mock_client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/tests/mock/mock_patch"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("UpdateAction", func() {
	var (
		ctrl         *gomock.Controller
		client       *mock_client.MockClient
		statusWriter *mock_client.MockStatusWriter
		cc           ClientCommandRunner
		sut          *updateAction
		pod          *corev1.Pod
		ctx          context.Context
		patcher      *mock_patch.MockPatchMaker
	)

	BeforeEach(func() {
		logger := logf.Log.WithName("UpdateAction")
		ctrl = gomock.NewController(GinkgoT())
		patcher = mock_patch.NewMockPatchMaker(ctrl)
		client = mock_client.NewMockClient(ctrl)
		statusWriter = mock_client.NewMockStatusWriter(ctrl)

		scheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))
		marketplacev1alpha1.AddToScheme(scheme)
		cc = NewClientCommand(client, scheme, logger)
		ctx = context.TODO()

		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID:       types.UID("foo"),
				Name:      "foo",
				Namespace: "bar",
			},
			Status: corev1.PodStatus{
				Conditions: []corev1.PodCondition{},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("when pod is nil", func() {
		BeforeEach(func() {
			pod = nil

			sut = UpdateAction(pod)
		})

		It("should handle nil object", func() {
			client.EXPECT().Create(ctx, pod).Return(nil).Times(0)

			result, err := cc.Do(ctx, sut)

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ErrNilObject))
			Expect(result).ToNot(BeNil())
			Expect(result.Status).To(Equal(Error))
		})
	})

	Context("when pod is not nil", func() {
		var (
			expectedErr error
		)

		BeforeEach(func() {
			expectedErr = errors.NewPlain("mock fail")
			sut = UpdateAction(pod)
		})

		It("should handle create", func() {
			client.EXPECT().Update(ctx, pod).Return(nil).Times(1)

			result, err := cc.Do(ctx, sut)

			Expect(err).To(Succeed())
			Expect(result).ToNot(BeNil())
			Expect(result.Status).To(Equal(Requeue))
		})

		It("should handle create with error", func() {
			client.EXPECT().Update(ctx, pod).Return(expectedErr).Times(1)

			result, err := cc.Do(ctx, sut)

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(expectedErr))
			Expect(result).ToNot(BeNil())
			Expect(result.Status).To(Equal(Error))
		})
	})

	Context("verify patcher", func() {
		var (
			expectedErr     error
			mockPatchResult *patch.PatchResult
			patcherTool     utilspatch.Patcher
		)

		BeforeEach(func() {
			expectedErr = errors.NewPlain("mock fail")
			mockPatchResult = &patch.PatchResult{}
			patcherTool = utilspatch.NewPatcher("test")
			patcherTool.PatchMaker = patcher
		})

		It("should handle err", func() {
			patcher.EXPECT().Calculate(pod, pod).Return(mockPatchResult, expectedErr)

			patchResult, err := patcherTool.Calculate(pod, pod)
			Expect(err).To(MatchError(expectedErr))
			Expect(patchResult).To(BeNil())
		})
	})

	Context("verify update status condition", func() {
		var (
			condition status.Condition
			meterbase *marketplacev1alpha1.MeterBase
			testErr   error
		)

		BeforeEach(func() {
			condition = status.Condition{
				Type:    marketplacev1alpha1.ConditionInstalling,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonMeterBaseStartInstall,
				Message: "created",
			}
			testErr = errors.NewPlain("test error")
			meterbase = &marketplacev1alpha1.MeterBase{
				Status: marketplacev1alpha1.MeterBaseStatus{},
			}
			meterbase.Status.Conditions = status.Conditions{}
		})

		It("should handle nil", func() {
			gomock.InOrder(
				client.EXPECT().Status().Return(statusWriter).Times(0),
				statusWriter.EXPECT().Update(ctx, nil).Return(nil).Times(0),
			)
			result, err := cc.Do(ctx, UpdateStatusCondition(nil, nil, condition))
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ErrNilObject))
			Expect(result).ToNot(BeNil())
			Expect(result.Status).To(Equal(Error))
		})

		It("should handle error", func() {
			gomock.InOrder(
				client.EXPECT().Status().Return(statusWriter).Times(1),
				statusWriter.EXPECT().Update(ctx, meterbase).Return(testErr).Times(1),
			)
			result, err := cc.Do(ctx, UpdateStatusCondition(meterbase, &meterbase.Status.Conditions, condition))
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(testErr))
			Expect(result).ToNot(BeNil())
			Expect(result.Status).To(Equal(Error))
		})
	})
})
