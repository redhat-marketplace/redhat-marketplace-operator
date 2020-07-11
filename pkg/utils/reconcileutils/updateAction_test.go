package reconcileutils

import (
	"context"

	"emperror.dev/errors"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/operator-framework/operator-sdk/pkg/status"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	utilspatch "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/patch"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/mock/mock_client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/mock/mock_patch"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/scheme"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
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
		cc = NewClientCommand(client, scheme.Scheme, logger)
		ctx = context.TODO()

		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID:       types.UID("foo"),
				Name:      "foo",
				Namespace: "bar",
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
			result, err := cc.Do(ctx, UpdateStatusCondition(meterbase, meterbase.Status.Conditions, condition))
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(testErr))
			Expect(result).ToNot(BeNil())
			Expect(result.Status).To(Equal(Error))
		})
	})
})
