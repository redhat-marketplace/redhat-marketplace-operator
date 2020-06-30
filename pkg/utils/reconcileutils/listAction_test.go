package reconcileutils

import (
	"context"

	"emperror.dev/errors"
	"github.com/coreos/prometheus-operator/pkg/client/versioned/scheme"
	"github.com/golang/mock/gomock"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/mock/mock_client"
	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ListAction", func() {
	var (
		ctrl        *gomock.Controller
		client      *mock_client.MockClient
		podList     *corev1.PodList
		cc          ClientCommandRunner
		ctx         context.Context
		expectedErr error
	)

	BeforeEach(func() {
		logger := logf.Log.WithName("CreateAction")
		ctrl = gomock.NewController(GinkgoT())
		client = mock_client.NewMockClient(ctrl)
		podList = &corev1.PodList{}
		cc = NewClientCommand(client, scheme.Scheme, logger)
		ctx = context.TODO()
		expectedErr = errors.NewPlain("mock fail")
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should return a list", func() {
		client.EXPECT().List(ctx, podList).Return(nil).Times(1)

		result, err := cc.Do(ctx, ListAction(podList))

		Expect(err).To(Succeed())
		Expect(result).ToNot(BeNil())
		Expect(result.Status).To(Equal(Continue))
	})

	It("should handle error", func() {
		client.EXPECT().List(ctx, podList).Return(expectedErr).Times(1)

		result, err := cc.Do(ctx, ListAction(podList))

		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(expectedErr))
		Expect(result).ToNot(BeNil())
		Expect(result.Status).To(Equal(Error))
	})
})
