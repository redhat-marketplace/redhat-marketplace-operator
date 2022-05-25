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

//go:generate mockgen -destination mock_client/mock_client.go sigs.k8s.io/controller-runtime/pkg/client Client,StatusWriter
//go:generate mockgen -destination mock_reconcileutils/mock_reconcileutils.go github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils ClientCommandRunner
//go:generate mockgen -destination mock_patch/mock_patch.go github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch PatchAnnotator,PatchMaker

package mock
