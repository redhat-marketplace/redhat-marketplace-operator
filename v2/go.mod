module github.com/redhat-marketplace/redhat-marketplace-operator/v2

go 1.15

require (
	emperror.dev/errors v0.8.0
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/banzaicloud/k8s-objectmatcher v1.5.0
	github.com/blang/semver v3.5.1+incompatible
	github.com/caarlos0/env/v6 v6.4.0
	github.com/cloudflare/cfssl v1.5.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/go-logr/logr v0.3.0
	github.com/go-logr/zapr v0.3.0 // indirect
	github.com/golang/mock v1.4.4
	github.com/google/go-cmp v0.5.4 // indirect
	github.com/google/uuid v1.1.2
	github.com/google/wire v0.4.0
	github.com/goph/emperror v0.17.2
	github.com/gotidy/ptr v1.3.0
	github.com/imdario/mergo v0.3.11
	github.com/jpillora/backoff v1.0.0
	github.com/kr/text v0.2.0 // indirect
	github.com/mitchellh/mapstructure v1.3.2 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/oliveagle/jsonpath v0.0.0-20180606110733-2e52cf6e6852
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.4
	github.com/openshift/api v0.0.0-20200930075302-db52bc4ef99f
	github.com/operator-framework/api v0.3.25
	github.com/pelletier/go-toml v1.8.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator v0.44.0
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.44.0
	github.com/prometheus/client_golang v1.8.0
	github.com/prometheus/common v0.14.0
	github.com/sirupsen/logrus v1.7.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	golang.org/x/mod v0.4.0 // indirect
	golang.org/x/text v0.3.4 // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	golang.org/x/tools v0.1.0 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/yaml.v2 v2.4.0
	honnef.co/go/tools v0.0.1-2020.1.6 // indirect
	k8s.io/api v0.19.4
	k8s.io/apiextensions-apiserver v0.19.3
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kube-openapi v0.0.0-20201113171705-d219536bb9fd // indirect
	k8s.io/kubectl v0.19.4
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
	sigs.k8s.io/controller-runtime v0.6.4
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v1.8.2-0.20201015110737-0a7fdd3b7696
	github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2 => ../metering/v2
	k8s.io/api => k8s.io/api v0.19.4
	k8s.io/client-go => k8s.io/client-go v0.19.4
)
