module github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2

go 1.26.3

require (
	dario.cat/mergo v1.0.2
	emperror.dev/errors v0.8.1
	github.com/go-logr/logr v1.4.4
	github.com/google/uuid v1.6.0
	github.com/hashicorp/go-retryablehttp v0.7.8
	github.com/ohler55/ojg v1.21.0
	github.com/onsi/ginkgo/v2 v2.32.0
	github.com/onsi/gomega v1.42.1
	github.com/openshift/api v0.0.0-20260807110950-72ae4424ef35
	github.com/qntfy/kazaam/v4 v4.0.1
	github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2 v2.0.0-00010101000000-000000000000
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 v2.0.0-20250730210926-13a572b16a7b
	github.com/ungerik/go-pool v0.0.0-20140720100922-d102a2c7872a
	go.uber.org/zap v1.28.0
	golang.org/x/exp v0.0.0-20261109185158-a52e3aef5920
	k8s.io/apimachinery v0.36.6
	k8s.io/apiserver v0.36.6
	k8s.io/client-go v0.36.6
	sigs.k8s.io/controller-runtime v0.24.1
)

require (
	cel.dev/expr v0.26.0 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/fxamacker/cbor/v2 v2.10.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/swag/cmdutils v0.29.2 // indirect
	github.com/go-openapi/swag/conv v0.29.2 // indirect
	github.com/go-openapi/swag/fileutils v0.29.2 // indirect
	github.com/go-openapi/swag/jsonutils v0.29.2 // indirect
	github.com/go-openapi/swag/loading v0.29.2 // indirect
	github.com/go-openapi/swag/mangling v0.29.2 // indirect
	github.com/go-openapi/swag/netutils v0.29.2 // indirect
	github.com/go-openapi/swag/pools v0.29.2 // indirect
	github.com/go-openapi/swag/stringutils v0.29.2 // indirect
	github.com/go-openapi/swag/typeutils v0.29.2 // indirect
	github.com/go-openapi/swag/yamlutils v0.29.2 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gofrs/uuid v3.2.0+incompatible // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/google/cel-go v0.31.0 // indirect
	github.com/google/gnostic-models v0.7.1 // indirect
	github.com/google/pprof v0.0.0-20260604005048-7023385849c0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/pandatix/go-abnf v0.5.5 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/qntfy/jsonparser v1.0.2 // indirect
	github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2 v2.0.0-20240410195839-0cc064c06b45 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.72.0 // indirect
	go.opentelemetry.io/otel v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.47.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk v1.47.0 // indirect
	go.opentelemetry.io/otel/trace v1.47.0 // indirect
	go.opentelemetry.io/proto/otlp v1.11.1 // indirect
	go.yaml.in/yaml/v2 v2.4.5 // indirect
	go.yaml.in/yaml/v3 v3.0.6 // indirect
	golang.org/x/mod v0.43.0 // indirect
	golang.org/x/oauth2 v0.38.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/tools v0.52.1-0.20261106145633-d407941a339a // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20261110000943-09e21b237f2c // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261110000943-09e21b237f2c // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	k8s.io/streaming v0.36.6 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.37.0 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.4.2 // indirect
)

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.5.0 // indirect
	github.com/Masterminds/sprig/v3 v3.3.0 // indirect
	github.com/banzaicloud/k8s-objectmatcher v1.8.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.14.0 // indirect
	github.com/evanphx/json-patch v5.9.11+incompatible // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v1.0.2 // indirect
	github.com/go-openapi/jsonreference v1.0.3 // indirect
	github.com/go-openapi/swag v0.29.2 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/goph/emperror v0.17.2 // indirect
	github.com/gotidy/ptr v1.4.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.33.0 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/imdario/mergo v1.0.2 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/operator-framework/api v0.44.0
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.93.0
	github.com/prometheus/client_golang v1.25.0
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.73.1 // indirect
	github.com/prometheus/procfs v0.23.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.1.0
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.59.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/term v0.48.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	golang.org/x/time v0.16.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.5.0 // indirect
	google.golang.org/grpc v1.85.0
	google.golang.org/protobuf v1.36.12 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/api v0.36.6
	k8s.io/apiextensions-apiserver v0.36.6 // indirect
	k8s.io/component-base v0.36.6
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/kube-openapi v0.0.0-20260721132016-d427ff9ee9ad // indirect
	k8s.io/utils v0.0.0-20261023123148-ca04ba58ecab
	sigs.k8s.io/json v0.0.0-20260909141634-11ed52e25bc5 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

replace (
	github.com/imdario/mergo => github.com/imdario/mergo v0.3.16
	github.com/prometheus-operator/prometheus-operator => github.com/prometheus-operator/prometheus-operator v0.87.0
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring => github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.87.0
	github.com/qntfy/kazaam/v4 => github.com/willie68/kazaam/v4 v4.0.0-20230518091049-08094afa110e // timestamp fix
	github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2 => ../../airgap/v2
	github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2 => ../../metering/v2
	github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2 => ../../reporter/v2
	github.com/redhat-marketplace/redhat-marketplace-operator/tests/v2 => ../../tests/v2
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 => ../../v2
	k8s.io/api => k8s.io/api v0.36.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.36.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.36.6
	k8s.io/apiserver => k8s.io/apiserver v0.36.6
	k8s.io/client-go => k8s.io/client-go v0.36.6
	k8s.io/component-base => k8s.io/component-base v0.36.6
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.36.6
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.24.1
)
