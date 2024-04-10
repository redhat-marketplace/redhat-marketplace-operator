module github.com/redhat-marketplace/redhat-marketplace-operator/v2

go 1.20

require (
	emperror.dev/errors v0.8.1
	github.com/Masterminds/sprig/v3 v3.2.3
	github.com/banzaicloud/k8s-objectmatcher v1.8.0
	github.com/caarlos0/env/v6 v6.10.1
	github.com/cespare/xxhash v1.1.0
	github.com/foxcpp/go-mockdns v1.0.0
	github.com/go-logr/logr v1.3.0
	github.com/golang/mock v1.6.0
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/uuid v1.4.0
	github.com/google/wire v0.5.0
	github.com/goph/emperror v0.17.2
	github.com/gotidy/ptr v1.4.0
	github.com/hashicorp/go-retryablehttp v0.7.4 // indirect
	github.com/imdario/mergo v0.3.16
	github.com/jpillora/backoff v1.0.0
	github.com/oliveagle/jsonpath v0.0.0-20180606110733-2e52cf6e6852
	github.com/onsi/gomega v1.29.0
	github.com/openshift/api v3.9.0+incompatible
	github.com/operator-framework/api v0.18.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator v0.66.0 // indirect
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.67.1
	github.com/prometheus/client_golang v1.17.0
	github.com/prometheus/common v0.45.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.16.0
	github.com/stretchr/testify v1.8.4
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.17.0
	golang.org/x/time v0.3.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/api v0.28.3
	k8s.io/apimachinery v0.28.3
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b
	sigs.k8s.io/controller-runtime v0.16.3
	sigs.k8s.io/yaml v1.4.0
)

require (
	github.com/modern-go/reflect2 v1.0.2
	go.uber.org/zap v1.26.0
	golang.org/x/exp v0.0.0-20230713183714-613f0c0eb8a1
)

require (
	dario.cat/mergo v1.0.0
	github.com/blang/semver/v4 v4.0.0
	github.com/golang-jwt/jwt/v5 v5.0.0
	github.com/openshift/cluster-monitoring-operator v0.0.4
	github.com/operator-framework/operator-lifecycle-manager v0.25.0
)

require (
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/aws/aws-sdk-go v1.44.334 // indirect
	github.com/cncf/xds/go v0.0.0-20230310173818-32f1caf87195 // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/edsrzf/mmap-go v1.1.0 // indirect
	github.com/efficientgo/core v1.0.0-rc.2 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/analysis v0.21.4 // indirect
	github.com/go-openapi/errors v0.20.4 // indirect
	github.com/go-openapi/loads v0.21.2 // indirect
	github.com/go-openapi/runtime v0.26.0 // indirect
	github.com/go-openapi/spec v0.20.9 // indirect
	github.com/go-openapi/strfmt v0.21.7 // indirect
	github.com/go-openapi/validate v0.22.1 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/pprof v0.0.0-20231101202521-4ca4178f5c7a // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/metalmatze/signal v0.0.0-20210307161603-1c9aa721a97a // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/openshift/library-go v0.0.0-20220525173854-9b950a41acdc // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pelletier/go-toml/v2 v2.0.9 // indirect
	github.com/prometheus-community/prom-label-proxy v0.7.0 // indirect
	github.com/prometheus-operator/prometheus-operator/pkg/client v0.67.1 // indirect
	github.com/prometheus/alertmanager v0.26.0 // indirect
	github.com/prometheus/common/sigv4 v0.1.0 // indirect
	github.com/prometheus/prometheus v1.8.2-0.20220315145411-881111fec433 // indirect
	go.mongodb.org/mongo-driver v1.12.1 // indirect
	go.opentelemetry.io/otel v1.16.0 // indirect
	go.opentelemetry.io/otel/metric v1.16.0 // indirect
	go.opentelemetry.io/otel/trace v1.16.0 // indirect
	golang.org/x/sync v0.4.0 // indirect
)

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.2.1 // indirect
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/zapr v1.2.4 // indirect
	github.com/go-openapi/jsonpointer v0.20.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.4 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/grafana/regexp v0.0.0-20221122212121-6b5c0a4cb7fd // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/huandu/xstrings v1.4.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/miekg/dns v1.1.56 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/onsi/ginkgo/v2 v2.13.0
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/procfs v0.11.1 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/afero v1.9.5 // indirect
	github.com/spf13/cast v1.5.1 // indirect
	github.com/spf13/cobra v1.7.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/subosito/gotenv v1.4.2 // indirect
	go.uber.org/goleak v1.2.1 // indirect
	golang.org/x/crypto v0.17.0 // indirect
	golang.org/x/mod v0.13.0 // indirect
	golang.org/x/oauth2 v0.12.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
	golang.org/x/term v0.15.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/tools v0.14.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.3.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.27.7 // indirect
	k8s.io/apiserver v0.27.7 // indirect
	k8s.io/component-base v0.27.7 // indirect
	k8s.io/klog/v2 v2.100.1 // indirect
	k8s.io/kube-aggregator v0.27.7 // indirect
	k8s.io/kube-openapi v0.0.0-20230501164219-8b0f38b5fd1f // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.3.0 // indirect
)

replace (
	github.com/banzaicloud/k8s-objectmatcher => github.com/banzaicloud/k8s-objectmatcher v1.6.1
	github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
	github.com/golang/mock => github.com/golang/mock v1.4.4
	github.com/openshift/api => github.com/openshift/api v0.0.0-20221021112143-4226c2167e40 // release-4.12
	github.com/openshift/cluster-monitoring-operator => github.com/openshift/cluster-monitoring-operator v0.1.1-0.20220930042853-58d1e5ad06f6
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.16.0
	github.com/prometheus/common => github.com/prometheus/common v0.44.0
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v0.44.0
	k8s.io/api => k8s.io/api v0.27.7
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.27.7
	k8s.io/apimachinery => k8s.io/apimachinery v0.27.7
	k8s.io/apiserver => k8s.io/apiserver v0.27.7
	k8s.io/client-go => k8s.io/client-go v0.27.7
	k8s.io/component-base => k8s.io/component-base v0.27.7
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.27.7
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.15.2
)
