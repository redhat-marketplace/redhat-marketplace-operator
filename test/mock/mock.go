//go:generate mockgen -destination mock_client/mock_client.go sigs.k8s.io/controller-runtime/pkg/client Client,StatusWriter
//go:generate mockgen -destination mock_reconcileutils/mock_reconcileutils.go github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils ClientCommandRunner
//go:generate mockgen -destination mock_patch/mock_patch.go github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/patch PatchAnnotator,PatchMaker

package mock
