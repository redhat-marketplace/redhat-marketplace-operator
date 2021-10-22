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
