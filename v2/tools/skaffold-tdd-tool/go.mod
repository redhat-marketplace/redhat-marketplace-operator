module github.com/redhat-marketplace/redhat-marketplace-operator/v2/tools/skaffold-tdd-tool

go 1.19

require (
	emperror.dev/errors v0.8.1
	github.com/GoogleContainerTools/skaffold v1.39.9
	// github.com/GoogleContainerTools/skaffold v1.16.0
	github.com/caarlos0/env/v6 v6.10.1
	github.com/gdamore/tcell/v2 v2.6.0
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 v2.0.0-20230512153729-85566ee3f06b // indirect
	github.com/rivo/tview v0.0.0-20230511053024-822bd067b165
	google.golang.org/grpc v1.55.0
	google.golang.org/protobuf v1.30.0
)

require github.com/redhat-marketplace/redhat-marketplace-operator/tests/v2 v2.0.0-20230512153729-85566ee3f06b

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.2.1 // indirect
	github.com/Masterminds/sprig/v3 v3.2.3 // indirect
	github.com/OneOfOne/xxhash v1.2.6 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.10.2 // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/gdamore/encoding v1.0.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20230510103437-eeec1cb781c3 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/huandu/xstrings v1.4.0 // indirect
	github.com/imdario/mergo v0.3.15 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/onsi/ginkgo/v2 v2.9.5 // indirect
	github.com/onsi/gomega v1.27.7 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.65.1 // indirect
	github.com/prometheus/client_golang v1.15.1 // indirect
	github.com/prometheus/common v0.43.0 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.14.0 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/oauth2 v0.8.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/term v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.9.3 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.27.6 // indirect
	k8s.io/apiextensions-apiserver v0.27.6 // indirect
	k8s.io/apimachinery v0.27.6 // indirect
	k8s.io/client-go v12.0.0+incompatible // indirect
	k8s.io/klog/v2 v2.100.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230501164219-8b0f38b5fd1f // indirect
	k8s.io/utils v0.0.0-20230505201702-9f6742963106 // indirect
	sigs.k8s.io/controller-runtime v0.15.2 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v14.2.0+incompatible // Required by OLM
	github.com/GoogleContainerTools/skaffold => github.com/GoogleContainerTools/skaffold v1.16.0
	github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.6.0
	github.com/banzaicloud/k8s-objectmatcher => github.com/banzaicloud/k8s-objectmatcher v1.6.1
	github.com/containerd/containerd v1.4.0-0 => github.com/containerd/containerd v1.4.0
	github.com/coreos/prometheus-operator => github.com/prometheus-operator/prometheus-operator v0.41.0
	github.com/dgrijalva/jwt-go => github.com/golang-jwt/jwt/v4 v4.5.0
	github.com/docker/docker => github.com/docker/docker v17.12.0-ce-rc1.0.20190319215453-e7b5f7dbe98c+incompatible
	github.com/docker/docker v1.14.0-0.20190319215453-e7b5f7dbe98c => github.com/docker/docker v17.12.0-ce-rc1.0.20190319215453-e7b5f7dbe98c+incompatible
	github.com/operator-framework/operator-marketplace => github.com/operator-framework/operator-marketplace v0.0.0-20201110032404-0e3bd3db36a6
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.16.0
	github.com/prometheus/common => github.com/prometheus/common v0.44.0
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v0.44.0
	github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2 => ../../../airgap/v2
	github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2 => ../../../reporter/v2
	github.com/redhat-marketplace/redhat-marketplace-operator/tests/v2 => ../../../tests/v2
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 => ../../
	k8s.io/api => k8s.io/api v0.27.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.27.6
	k8s.io/client-go => k8s.io/client-go v0.27.6
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.15.2
)
