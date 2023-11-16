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

//go:build tools
// +build tools

// Place any runtime dependencies as imports in this file.
// Go modules will be forced to download and install them.
package tools

import (
	_ "github.com/Masterminds/semver/v3"
	_ "github.com/Shyp/bump_version"
	_ "github.com/golang/mock/mockgen"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/google/addlicense"
	_ "github.com/google/wire"
	_ "github.com/launchdarkly/go-options"
	_ "github.com/mikefarah/yq/v4"
	_ "github.com/onsi/ginkgo/v2/ginkgo"
	_ "github.com/spf13/cobra"
	_ "github.com/tcnksm/ghr"
	_ "github.com/wadey/gocovmerge"
	_ "k8s.io/code-generator/cmd/client-gen"
)
