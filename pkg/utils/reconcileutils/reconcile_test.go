package reconcileutils

import (
	"context"
	"fmt"
	"testing"

	emperrors "emperror.dev/errors"
	"github.com/golang/mock/gomock"
	"github.com/operator-framework/operator-sdk/pkg/status"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/mock/mock_client"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/scheme"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

type setupFunc func(sut *testHarness, client *mock_client.MockClient, statusWriter *mock_client.MockStatusWriter)

func TestClientCommandsAll(t *testing.T) {
	tests := []struct {
		setupMock setupFunc
	}{
		// test if get returns err
		{func(sut *testHarness, client *mock_client.MockClient, statusWriter *mock_client.MockStatusWriter) {
			gomock.InOrder(
				client.EXPECT().
					Get(sut.ctx, sut.namespacedName, sut.pod).
					Return(sut.testErr).
					Times(1),
			)

			sut.Expected.ResultStatus = Error
			sut.Expected.Err = sut.testErr
		}},
		// test get with ignore not found
		{func(sut *testHarness, client *mock_client.MockClient, statusWriter *mock_client.MockStatusWriter) {
			sut.ignoreNotFound = true

			gomock.InOrder(
				client.EXPECT().
					Get(sut.ctx, sut.namespacedName, sut.pod).
					Return(errors.NewNotFound(schema.GroupResource{Group: "", Resource: "Pod"}, sut.namespacedName.Name)).
					Times(1),
				client.EXPECT().
					Update(sut.ctx, gomock.Any()).
					Return(sut.testErr).Times(1),
			)

			sut.Expected.ResultStatus = Error
			sut.Expected.Err = sut.testErr
		}},
		// test get w/ not found and if with create
		{func(sut *testHarness, client *mock_client.MockClient, statusWriter *mock_client.MockStatusWriter) {
			gomock.InOrder(
				client.EXPECT().
					Get(sut.ctx, sut.namespacedName, sut.pod).
					Return(errors.NewNotFound(schema.GroupResource{Group: "", Resource: "Pod"}, sut.namespacedName.Name)).
					Times(1),
				client.EXPECT().Create(sut.ctx, sut.pod).Return(nil).Times(1),
				client.EXPECT().Status().Return(statusWriter).Times(1),
				statusWriter.EXPECT().Update(sut.ctx, sut.meterbase).Return(nil).Times(1),
			)
		}},
		// test get and update
		{func(sut *testHarness, client *mock_client.MockClient, statusWriter *mock_client.MockStatusWriter) {
			client.EXPECT().Create(sut.ctx, sut.pod).Return(nil).Times(0)
			gomock.InOrder(
				client.EXPECT().
					Get(sut.ctx, sut.namespacedName, sut.pod).
					Return(nil).
					Times(1),
				client.EXPECT().
					Update(sut.ctx, gomock.Any()).
					DoAndReturn(func(ctx context.Context, obj runtime.Object) error {
						if obj == nil {
							t.Error("Update did not get a new value; got nil")
						}
						return nil
					}).Times(1),
				client.EXPECT().Status().Return(statusWriter).Times(1),
				statusWriter.EXPECT().Update(sut.ctx, sut.meterbase).Return(nil).Times(1),
			)
		}},
		// test get, no update, and delete
		{func(sut *testHarness, client *mock_client.MockClient, statusWriter *mock_client.MockStatusWriter) {
			sut.pod.Annotations["foo"] = "bar"

			client.EXPECT().Create(sut.ctx, sut.pod).Return(nil).Times(0)
			client.EXPECT().
				Update(sut.ctx, gomock.Any()).
				Return(nil).Times(0)

			gomock.InOrder(
				client.EXPECT().
					Get(sut.ctx, sut.namespacedName, sut.pod).
					Return(nil).
					Times(1),
				client.EXPECT().
					Delete(sut.ctx, sut.pod).
					Return(nil).
					Times(1),
				client.EXPECT().Status().Return(statusWriter).Times(1),
				statusWriter.EXPECT().Update(sut.ctx, sut.meterbase).Return(nil).Times(1),
			)

			sut.Expected.ResultStatus = Continue
		}},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("test-%v", i+1), func(t *testing.T) {
			sut := NewTestHarness()
			result, err := sut.execClientCommands(t, tt.setupMock)

			if err != nil && result.Status != Error {
				assert.Fail(t, "there is an error with the wrong status", "status", result.Status)
			}

			assert.NotNil(t, result)
			assert.Equal(t, sut.Expected.Err, result.Err)
			assert.Equal(t, sut.Expected.ResultStatus, result.Status)
		})
	}
}

type testHarness struct {
	ctx            context.Context
	meterbase      *marketplacev1alpha1.MeterBase
	namespacedName types.NamespacedName
	pod            *corev1.Pod
	updatedPod     *corev1.Pod
	testErr        error
	ignoreNotFound bool

	Expected struct {
		ResultStatus ActionResultStatus
		Err          error
	}
}

func NewTestHarness() *testHarness {
	harness := &testHarness{}
	harness.meterbase = &marketplacev1alpha1.MeterBase{
		Status: marketplacev1alpha1.MeterBaseStatus{},
	}
	harness.testErr = emperrors.New("a test error")
	harness.namespacedName = types.NamespacedName{Name: "foo", Namespace: "ns"}
	harness.pod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID("foo"),
			Name:      "foo",
			Namespace: "bar",
		},
	}
	utils.RhmAnnotator.SetLastAppliedAnnotation(harness.pod)
	harness.ctx = context.TODO()
	harness.Expected.ResultStatus = Requeue
	harness.Expected.Err = nil
	return harness
}

func (h *testHarness) execClientCommands(
	t *testing.T,
	setupMock setupFunc,
) (*ExecResult, error) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock_client.NewMockClient(ctrl)
	statusWriter := mock_client.NewMockStatusWriter(ctrl)

	setupMock(h, client, statusWriter)

	logf.SetLogger(logf.ZapLogger(true))
	logger := logf.Log.WithName("clienttest")

	cc := NewClientCommand(client, scheme.Scheme, logger)
	return cc.Do(
		h.ctx,
		GetAction(h.namespacedName, h.pod, GetWithIgnoreNotFound(h.ignoreNotFound)),
		If(
			func(getResult *ExecResult) bool {
				return getResult.Is(NotFound)
			},
			CreateAction(
				func() (runtime.Object, error) {
					return h.pod, nil
				},
				CreateWithPatch(utils.RhmAnnotator),
				CreateWithAddOwner(h.pod),
				CreateWithStatusCondition(func(result *ExecResult, err error) (update bool, instance runtime.Object, conditions *status.Conditions, condition status.Condition) {
					return result.Is(Requeue), h.meterbase, h.meterbase.Status.Conditions, status.Condition{
						Type:    marketplacev1alpha1.ConditionInstalling,
						Status:  corev1.ConditionTrue,
						Reason:  marketplacev1alpha1.ReasonMeterBaseStartInstall,
						Message: "created",
					}
				}),
			),
		),
		UpdateWithPatch(h.pod, utils.RhmPatchMaker,
			func() (updatedObject runtime.Object, err error) {
				h.updatedPod = h.pod.DeepCopy()
				h.updatedPod.Annotations["foo"] = "bar"
				return h.updatedPod, nil
			},
			UpdateWithStatusCondition(func(result *ExecResult, err error) (update bool, instance runtime.Object, conditions *status.Conditions, condition status.Condition) {
				return result.Is(Requeue), h.meterbase, h.meterbase.Status.Conditions, status.Condition{
					Type:    marketplacev1alpha1.ConditionInstalling,
					Status:  corev1.ConditionTrue,
					Reason:  marketplacev1alpha1.ReasonMeterBaseStartInstall,
					Message: "created",
				}
			}),
		),
		DeleteAction(h.pod,
			DeleteWithStatusCondition(
				func(result *ExecResult, err error) (update bool, instance runtime.Object, conditions *status.Conditions, condition status.Condition) {
					return result.Is(Continue), h.meterbase, h.meterbase.Status.Conditions, status.Condition{
						Type:    marketplacev1alpha1.ConditionInstalling,
						Status:  corev1.ConditionTrue,
						Reason:  marketplacev1alpha1.ReasonMeterBaseStartInstall,
						Message: "deleted",
					}
				}),
		),
	)
}
