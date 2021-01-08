module github.com/redhat-marketplace/redhat-marketplace-operator/v2/test

go 1.15

require (
	emperror.dev/errors v0.8.0
	github.com/banzaicloud/k8s-objectmatcher v1.5.0
	github.com/caarlos0/env v3.5.0+incompatible
	github.com/caarlos0/env/v6 v6.4.0
	github.com/go-logr/logr v0.3.0
	github.com/golang/mock v1.4.4
	github.com/google/wire v0.4.0
	github.com/gotidy/ptr v1.3.0
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.3
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.44.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	k8s.io/api v0.19.4
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v11.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.6.4
)
