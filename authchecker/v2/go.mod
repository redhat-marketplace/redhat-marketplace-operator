module github.com/redhat-marketplace/redhat-marketplace-operator/authchecker/v2

go 1.15

require (
	emperror.dev/errors v0.8.0
	github.com/go-logr/logr v0.3.0
	github.com/google/wire v0.4.0
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 v2.0.0-00010101000000-000000000000
	github.com/spf13/cobra v1.1.1
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.6.4
)

replace (
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v1.8.2-0.20201015110737-0a7fdd3b7696
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 => ../../v2
	github.com/redhat-marketplace/redhat-marketplace-operator/v2/test => ../../v2/test
	k8s.io/api => k8s.io/api v0.19.4
	k8s.io/client-go => k8s.io/client-go v0.19.4
)
