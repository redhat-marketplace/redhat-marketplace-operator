package reconcileutils

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	opsrcApi "github.com/operator-framework/operator-marketplace/pkg/apis"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/mock/mock_client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/scheme"
)

func TestClientCommands(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	logger := logf.ZapLogger(true)

	_ = opsrcApi.AddToScheme(scheme.Scheme)
	_ = olmv1alpha1.AddToScheme(scheme.Scheme)
	_ = olmv1.AddToScheme(scheme.Scheme)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	namespacedName := types.NamespacedName{Name: "foo", Namespace: "ns"}
	pod := &corev1.Pod{}
	client := mock_client.MockClient(ctrl)
	ctx := context.TODO()

	client.
		EXPECT().
		Get(ctx, namespacedName, pod).
		Return(nil)

	cc := NewClientCommand(client, scheme.Scheme, logger)

	result, allResults := cc.Execute(
		ctx,
		&Result{
			Var: getResult,
			Action: GetAction(
				namespacedName,
				pod,
			),
		},
		&If{
			If: func() bool {
				return getResult.Is(NotFound)
			},
			Then: CreateAction(
				func() (runtime.Object, error) {
					return nil, nil
				},
			),
			Else: UpdateAction(
				pod,
				func() (bool, runtime.Object, error) {
					return false, nil, nil
				},
			),
		},
	)
}
