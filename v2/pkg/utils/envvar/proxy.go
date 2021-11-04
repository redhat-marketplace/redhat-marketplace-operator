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

package envvar

import (
	"golang.org/x/net/http/httpproxy"
	corev1 "k8s.io/api/core/v1"
)

func AddHttpsProxy() Changes {
	proxyInfo := httpproxy.FromEnvironment()

	envVarChanges := Changes{}

	if proxyInfo.HTTPProxy != "" {
		envVarChanges.Add(corev1.EnvVar{
			Name:  "HTTP_PROXY",
			Value: proxyInfo.HTTPProxy,
		})
	} else {
		envVarChanges.Remove(corev1.EnvVar{
			Name: "HTTP_PROXY",
		})
	}

	if proxyInfo.HTTPSProxy != "" {
		envVarChanges.Add(corev1.EnvVar{
			Name:  "HTTPS_PROXY",
			Value: proxyInfo.HTTPSProxy,
		})
	} else {
		envVarChanges.Remove(corev1.EnvVar{
			Name: "HTTPS_PROXY",
		})
	}

	if proxyInfo.NoProxy != "" {
		envVarChanges.Add(corev1.EnvVar{
			Name:  "NO_PROXY",
			Value: proxyInfo.NoProxy,
		})
	} else {
		envVarChanges.Remove(corev1.EnvVar{
			Name: "NO_PROXY",
		})
	}

	return envVarChanges
}
