module github.com/redhat-marketplace/redhat-marketplace-operator/v2

go 1.16

require (
	emperror.dev/errors v0.8.0
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/banzaicloud/k8s-objectmatcher v1.6.1
	github.com/blang/semver v3.5.1+incompatible
	github.com/caarlos0/env/v6 v6.4.0
	github.com/cespare/xxhash v1.1.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/foxcpp/go-mockdns v0.0.0-20210729171921-fb145fc6f897
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-logr/logr v1.2.0
	github.com/golang-jwt/jwt v3.2.1+incompatible // indirect
	github.com/golang/mock v1.5.0
	github.com/google/addlicense v1.0.0 // indirect
	github.com/google/uuid v1.1.2
	github.com/google/wire v0.4.0
	github.com/goph/emperror v0.17.2
	github.com/gotidy/ptr v1.3.0
	github.com/imdario/mergo v0.3.12
	github.com/jpillora/backoff v1.0.0
	github.com/oliveagle/jsonpath v0.0.0-20180606110733-2e52cf6e6852
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/openshift/api v0.0.0-20200930075302-db52bc4ef99f
	github.com/openshift/cluster-monitoring-operator v0.1.1-0.20210130044457-b344b13b469f
	github.com/operator-framework/api v0.3.25
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator v0.44.0
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.46.0
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/common v0.28.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.1
	github.com/stretchr/testify v1.7.0
	golang.org/x/net v0.0.0-20211209124913-491a49abca63
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.23.5
	k8s.io/apiextensions-apiserver v0.23.5
	k8s.io/apimachinery v0.23.5
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/utils v0.0.0-20211116205334-6203023598ed
	sigs.k8s.io/controller-runtime v0.11.2
	sigs.k8s.io/yaml v1.3.0
)

replace (
	github.com/dgrijalva/jwt-go => github.com/redhat-marketplace/jwt v3.2.1+incompatible
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v1.8.2-0.20201015110737-0a7fdd3b7696
	k8s.io/api => k8s.io/api v0.23.0
	k8s.io/client-go => k8s.io/client-go v0.23.0
)
