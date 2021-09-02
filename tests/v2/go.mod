module github.com/redhat-marketplace/redhat-marketplace-operator/tests/v2

go 1.16

require (
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/blang/semver v3.5.1+incompatible
	github.com/go-logr/logr v0.4.0
	github.com/google/uuid v1.1.2
	github.com/google/wire v0.5.0
	github.com/gotidy/ptr v1.3.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.16.0
	github.com/openshift/api v0.0.0-20200930075302-db52bc4ef99f
	github.com/operator-framework/api v0.3.25
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.46.0
	github.com/prometheus/client_golang v1.9.0
	github.com/prometheus/common v0.15.0
	github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2 v2.0.0-20210409165614-0a6355b7a700
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 v2.0.0-20210409165614-0a6355b7a700
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.20.1
	k8s.io/apimachinery v0.20.1
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.6.4
)

replace (
	github.com/dgrijalva/jwt-go => github.com/redhat-marketplace/jwt v3.2.1+incompatible
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v1.8.2-0.20201015110737-0a7fdd3b7696
	github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2 => ../../reporter/v2
	github.com/redhat-marketplace/redhat-marketplace-operator/tests/v2 => ./
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 => ../../v2
	k8s.io/api => k8s.io/api v0.19.4
	k8s.io/client-go => k8s.io/client-go v0.19.4
)
