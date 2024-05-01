module github.com/redhat-marketplace/redhat-marketplace-operator/v2

go 1.20

require (
	emperror.dev/errors v0.8.1
	github.com/Masterminds/sprig/v3 v3.2.3
	github.com/banzaicloud/k8s-objectmatcher v1.8.0
	github.com/caarlos0/env/v6 v6.10.1
	github.com/cespare/xxhash v1.1.0
	github.com/foxcpp/go-mockdns v1.0.0
	github.com/go-logr/logr v1.4.1
	github.com/golang/mock v1.6.0
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/uuid v1.6.0
	github.com/google/wire v0.5.0
	github.com/goph/emperror v0.17.2
	github.com/gotidy/ptr v1.4.0
	github.com/imdario/mergo v0.3.16
	github.com/jpillora/backoff v1.0.0
	github.com/oliveagle/jsonpath v0.0.0-20180606110733-2e52cf6e6852
	github.com/onsi/gomega v1.29.0
	github.com/openshift/api v0.0.0-20240304080513-3e8192a10b13
	github.com/operator-framework/api v0.18.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator v0.66.0 // indirect
	github.com/prometheus/client_golang v1.16.0
	github.com/prometheus/common v0.44.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.16.0
	github.com/stretchr/testify v1.9.0
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.24.0
	golang.org/x/time v0.5.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/api v0.28.8
	k8s.io/apimachinery v0.28.8
	k8s.io/client-go v0.28.8
	k8s.io/utils v0.0.0-20240310230437-4693a0247e57
	sigs.k8s.io/controller-runtime v0.16.5
	sigs.k8s.io/yaml v1.4.0
)

require (
	github.com/modern-go/reflect2 v1.0.2
	go.uber.org/zap v1.26.0
	golang.org/x/exp v0.0.0-20230522175609-2e198f4a06a1
)

require (
	dario.cat/mergo v1.0.0
	github.com/blang/semver/v4 v4.0.0
	github.com/golang-jwt/jwt/v5 v5.2.0
	github.com/openshift/cluster-monitoring-operator v0.1.1-0.20240417033437-1839fb4d246a
	github.com/operator-framework/operator-lifecycle-manager v0.22.0
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.66.0
)

require (
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/antlr/antlr4/runtime/Go/antlr/v4 v4.0.0-20230305170008-8188dc5388df // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/aws/aws-sdk-go v1.50.0 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/docker/distribution v2.8.2+incompatible // indirect
	github.com/edsrzf/mmap-go v1.1.0 // indirect
	github.com/efficientgo/core v1.0.0-rc.2 // indirect
	github.com/emicklei/go-restful/v3 v3.11.3 // indirect
	github.com/evanphx/json-patch/v5 v5.7.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/analysis v0.21.4 // indirect
	github.com/go-openapi/errors v0.21.0 // indirect
	github.com/go-openapi/loads v0.21.2 // indirect
	github.com/go-openapi/runtime v0.26.0 // indirect
	github.com/go-openapi/spec v0.20.9 // indirect
	github.com/go-openapi/strfmt v0.22.0 // indirect
	github.com/go-openapi/validate v0.22.1 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/cel-go v0.16.1 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/pprof v0.0.0-20240117000934-35fc243c5815 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.16.2 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/metalmatze/signal v0.0.0-20210307161603-1c9aa721a97a // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/openshift/library-go v0.0.0-20230120214501-9bc305884fcb // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pelletier/go-toml/v2 v2.0.9 // indirect
	github.com/prometheus-community/prom-label-proxy v0.7.0 // indirect
	github.com/prometheus-operator/prometheus-operator/pkg/client v0.66.0 // indirect
	github.com/prometheus/alertmanager v0.26.0 // indirect
	github.com/prometheus/common/sigv4 v0.1.0 // indirect
	github.com/prometheus/prometheus v0.44.0 // indirect
	github.com/stoewer/go-strcase v1.2.1 // indirect
	go.etcd.io/etcd/api/v3 v3.5.13 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.13 // indirect
	go.etcd.io/etcd/client/v3 v3.5.13 // indirect
	go.mongodb.org/mongo-driver v1.13.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.49.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.47.0 // indirect
	go.opentelemetry.io/otel v1.24.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.22.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.22.0 // indirect
	go.opentelemetry.io/otel/metric v1.24.0 // indirect
	go.opentelemetry.io/otel/sdk v1.22.0 // indirect
	go.opentelemetry.io/otel/trace v1.24.0 // indirect
	go.opentelemetry.io/proto/otlp v1.0.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240415151819-79826c84ba32 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240415151819-79826c84ba32 // indirect
	google.golang.org/grpc v1.63.2 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	k8s.io/kms v0.28.8 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.1.9 // indirect
)

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.2.1 // indirect
	github.com/alecthomas/units v0.0.0-20231202071711-9a357b53e9c9 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/evanphx/json-patch v5.7.0+incompatible // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/zapr v1.2.4 // indirect
	github.com/go-openapi/jsonpointer v0.20.3 // indirect
	github.com/go-openapi/jsonreference v0.20.5 // indirect
	github.com/go-openapi/swag v0.22.10 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/grafana/regexp v0.0.0-20221122212121-6b5c0a4cb7fd // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/huandu/xstrings v1.4.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/miekg/dns v1.1.58 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/onsi/ginkgo/v2 v2.13.0
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/afero v1.10.0 // indirect
	github.com/spf13/cast v1.5.1 // indirect
	github.com/spf13/cobra v1.7.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/subosito/gotenv v1.4.2 // indirect
	go.uber.org/goleak v1.3.0 // indirect
	golang.org/x/crypto v0.22.0 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/oauth2 v0.17.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/term v0.19.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/tools v0.20.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.28.8 // indirect
	k8s.io/apiserver v0.28.8 // indirect
	k8s.io/component-base v0.28.8
	k8s.io/klog/v2 v2.120.1 // indirect
	k8s.io/kube-aggregator v0.28.8 // indirect
	k8s.io/kube-openapi v0.0.0-20230717233707-2695361300d9 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
)

replace (
	github.com/banzaicloud/k8s-objectmatcher => github.com/banzaicloud/k8s-objectmatcher v1.6.1
	github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
	github.com/golang/mock => github.com/golang/mock v1.4.4
	github.com/prometheus-operator/prometheus-operator => github.com/prometheus-operator/prometheus-operator v0.66.0
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring => github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.66.0
	k8s.io/api => k8s.io/api v0.28.8
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.28.8
	k8s.io/apimachinery => k8s.io/apimachinery v0.28.8
	k8s.io/apiserver => k8s.io/apiserver v0.28.8
	k8s.io/client-go => k8s.io/client-go v0.28.8
	k8s.io/component-base => k8s.io/component-base v0.28.8
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.28.8
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.16.5
)
