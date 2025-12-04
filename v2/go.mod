module github.com/redhat-marketplace/redhat-marketplace-operator/v2

go 1.24.0

toolchain go1.24.6

require (
	emperror.dev/errors v0.8.1
	github.com/Masterminds/sprig/v3 v3.3.0
	github.com/banzaicloud/k8s-objectmatcher v1.8.0
	github.com/caarlos0/env/v6 v6.10.1
	github.com/cespare/xxhash v1.1.0
	github.com/foxcpp/go-mockdns v1.0.0
	github.com/go-logr/logr v1.4.3
	github.com/golang/mock v1.6.0
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0
	github.com/google/wire v0.5.0
	github.com/goph/emperror v0.17.2
	github.com/gotidy/ptr v1.4.0
	github.com/imdario/mergo v1.0.2
	github.com/jpillora/backoff v1.0.0
	github.com/oliveagle/jsonpath v0.0.0-20180606110733-2e52cf6e6852
	github.com/onsi/gomega v1.38.2
	github.com/openshift/api v0.0.0-20250618185501-1c8afbdd3f90
	github.com/operator-framework/api v0.18.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.23.2
	github.com/prometheus/common v0.67.3
	github.com/spf13/pflag v1.0.10
	github.com/spf13/viper v1.16.0
	github.com/stretchr/testify v1.11.1
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.47.0
	golang.org/x/time v0.14.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/api v0.33.6
	k8s.io/apimachinery v0.33.6
	k8s.io/client-go v0.33.6
	k8s.io/utils v0.0.0-20251002143259-bc988d571ff4
	sigs.k8s.io/controller-runtime v0.21.0
	sigs.k8s.io/yaml v1.6.0
)

require (
	github.com/modern-go/reflect2 v1.0.2
	go.uber.org/zap v1.27.0
	golang.org/x/exp v0.0.0-20251113190631-e25ba8c21ef6
)

require (
	dario.cat/mergo v1.0.2
	github.com/blang/semver/v4 v4.0.0
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/openshift/cluster-monitoring-operator v0.1.1-0.20251105083458-870b44fc269a
	github.com/operator-framework/operator-lifecycle-manager v0.22.0
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.81.0
)

require (
	cel.dev/expr v0.25.1 // indirect
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/swag/cmdutils v0.25.3 // indirect
	github.com/go-openapi/swag/conv v0.25.3 // indirect
	github.com/go-openapi/swag/fileutils v0.25.3 // indirect
	github.com/go-openapi/swag/jsonname v0.25.3 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.3 // indirect
	github.com/go-openapi/swag/loading v0.25.3 // indirect
	github.com/go-openapi/swag/mangling v0.25.3 // indirect
	github.com/go-openapi/swag/netutils v0.25.3 // indirect
	github.com/go-openapi/swag/stringutils v0.25.3 // indirect
	github.com/go-openapi/swag/typeutils v0.25.3 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.3 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/cel-go v0.26.1 // indirect
	github.com/google/gnostic-models v0.7.1 // indirect
	github.com/google/pprof v0.0.0-20251114195745-4902fdda35c8 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.3 // indirect
	github.com/openshift/library-go v0.0.0-20250402180609-ce2ba53fb2a4 // indirect
	github.com/pelletier/go-toml/v2 v2.0.9 // indirect
	github.com/prometheus/prometheus v0.301.0 // indirect
	github.com/stoewer/go-strcase v1.3.1 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.63.0 // indirect
	go.opentelemetry.io/otel v1.38.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.38.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.38.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	go.opentelemetry.io/proto/otlp v1.9.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/sync v0.18.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251111163417-95abcf5c77ba // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251111163417-95abcf5c77ba // indirect
	google.golang.org/grpc v1.77.0 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.34.0 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
)

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/evanphx/json-patch v5.9.0+incompatible // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.22.3 // indirect
	github.com/go-openapi/jsonreference v0.21.3 // indirect
	github.com/go-openapi/swag v0.25.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/grafana/regexp v0.0.0-20240518133315-a468a5bfb3bc // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/miekg/dns v1.1.62 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/onsi/ginkgo/v2 v2.27.2
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/procfs v0.19.2 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/afero v1.10.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/cobra v1.10.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/subosito/gotenv v1.4.2 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/mod v0.30.0 // indirect
	golang.org/x/oauth2 v0.33.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/term v0.37.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	golang.org/x/tools v0.39.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.5.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.33.6 // indirect
	k8s.io/apiserver v0.33.6 // indirect
	k8s.io/component-base v0.33.6
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-aggregator v0.33.6 // indirect
	k8s.io/kube-openapi v0.0.0-20250701173324-9bd5c66d9911 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.7.0 // indirect
)

replace (
	github.com/banzaicloud/k8s-objectmatcher => github.com/banzaicloud/k8s-objectmatcher v1.6.1
	github.com/dgrijalva/jwt-go => github.com/golang-jwt/jwt/v4 v4.5.0
	github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
	github.com/golang/mock => github.com/golang/mock v1.4.4
	github.com/google/cel-go => github.com/google/cel-go v0.22.0
	github.com/imdario/mergo => github.com/imdario/mergo v0.3.16
	github.com/prometheus-operator/prometheus-operator => github.com/prometheus-operator/prometheus-operator v0.81.0
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring => github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.81.0
	k8s.io/api => k8s.io/api v0.33.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.33.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.33.6
	k8s.io/apiserver => k8s.io/apiserver v0.33.6
	k8s.io/client-go => k8s.io/client-go v0.33.6
	k8s.io/component-base => k8s.io/component-base v0.33.6
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.33.6
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.21.0
)
