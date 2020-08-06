package reconcileutils

import (
	"context"

	"emperror.dev/errors"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/mock/mock_client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/mock/mock_patch"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/scheme"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var _ = Describe("CreateAction", func() {
	var (
		ctrl   *gomock.Controller
		client *mock_client.MockClient
		//statusWriter *mock_client.MockStatusWriter
		cc        ClientCommandRunner
		sut       *createAction
		pod       *corev1.Pod
		ctx       context.Context
		meterbase *marketplacev1alpha1.MeterBase
		patcher   *mock_patch.MockPatchAnnotator
	)

	BeforeEach(func() {
		logger := logf.Log.WithName("CreateAction")
		ctrl = gomock.NewController(GinkgoT())
		patcher = mock_patch.NewMockPatchAnnotator(ctrl)
		client = mock_client.NewMockClient(ctrl)
		apis.AddToScheme(scheme.Scheme)
		//statusWriter = mock_client.NewMockStatusWriter(ctrl)
		cc = NewClientCommand(client, scheme.Scheme, logger)
		ctx = context.TODO()

		meterbase = &marketplacev1alpha1.MeterBase{
			Status: marketplacev1alpha1.MeterBaseStatus{},
		}
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

			sut = CreateAction(pod,
				CreateWithPatch(utils.RhmAnnotator),
				CreateWithAddOwner(meterbase),
			)
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
			sut = CreateAction(pod,
				CreateWithPatch(utils.RhmAnnotator),
				CreateWithAddOwner(meterbase),
			)
		})

		It("should handle create", func() {
			client.EXPECT().Create(ctx, pod).Return(nil).Times(1)

			result, err := cc.Do(ctx, sut)

			Expect(err).To(Succeed())
			Expect(result).ToNot(BeNil())
			Expect(result.Status).To(Equal(Requeue))
		})

		It("should handle create with error", func() {
			client.EXPECT().Create(ctx, pod).Return(expectedErr).Times(1)

			result, err := cc.Do(ctx, sut)

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(expectedErr))
			Expect(result).ToNot(BeNil())
			Expect(result.Status).To(Equal(Error))
		})
	})

	Context("when patcher is bad", func() {
		var (
			expectedErr error
		)

		BeforeEach(func() {
			expectedErr = errors.NewPlain("mock fail")
			sut = CreateAction(pod,
				CreateWithPatch(patcher),
				CreateWithAddOwner(meterbase),
			)
		})

		It("should handle patch error", func() {
			patcher.EXPECT().SetLastAppliedAnnotation(pod).Return(expectedErr)

			result, err := cc.Do(ctx, sut)

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(expectedErr))
			Expect(result).ToNot(BeNil())
			Expect(result.Status).To(Equal(Error))
		})
	})
})
