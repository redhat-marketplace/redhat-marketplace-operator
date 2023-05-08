// Copyright 2021 IBM Corp.
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

package envvar_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/envvar"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

var _ = Describe("envvar", func() {
	var (
		var1  = v1.EnvVar{Name: "foo"}
		var2  = v1.EnvVar{Name: "foo2"}
		var2a = v1.EnvVar{Name: "foo2", Value: "a"}
		var2b = v1.EnvVar{Name: "foo2", Value: "b"}

		container corev1.Container
		changes   envvar.Changes
	)

	BeforeEach(func() {
		container = corev1.Container{
			Env: []v1.EnvVar{var1},
		}
		changes = envvar.Changes{}
	})

	It("should add env vars", func() {
		changes.Add(var2)
		changes.Merge(&container)
		Expect(container.Env).To(ConsistOf(var1, var2))
	})

	It("should remove env vars", func() {
		changes.Remove(var1)
		changes.Merge(&container)
		Expect(container.Env).To(BeEmpty())
	})

	It("should override if same name", func() {
		changes.Add(var2a)
		changes.Add(var2b)
		changes.Merge(&container)
		Expect(container.Env).To(ConsistOf(var1, var2b))
	})

	It("should add/remove env vars", func() {
		changes.Remove(var1)
		changes2 := envvar.Changes{}
		changes2.Add(var2)

		changes.Append(changes2)
		changes.Merge(&container)
		Expect(container.Env).To(ConsistOf(var2))
	})
})
