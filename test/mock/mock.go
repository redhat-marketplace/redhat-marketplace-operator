//go:generate mockgen -destination mock_client/mock_k8s_client.go sigs.k8s.io/controller-runtime/pkg/client Client,StatusWriter
//go:generate mockgen -destination mock_reconcileutils/mock_k8s_reconcileutils.go github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils ClientCommandRunner
//go:generate mockgen -destination mock_patch/mock_k8s_patch.go github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/patch PatchAnnotator,PatchMaker

package mock
