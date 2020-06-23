//go:generate mockgen -destination mock_client/mock_client.go sigs.k8s.io/controller-runtime/pkg/client Client,StatusWriter
//go:generate mockgen -destination mock_reconcileutils/mock_reconcileutils.go github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils PatchAnnotator,PatchMaker

package mock
