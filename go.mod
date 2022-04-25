module github.com/redhat-marketplace/redhat-marketplace-operator/v2

go 1.17

require (
	emperror.dev/errors v0.8.1
	github.com/InVisionApp/go-health/v2 v2.1.2
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/Shyp/bump_version v0.0.0-20180222180749-d7594d2951e2
	github.com/banzaicloud/k8s-objectmatcher v1.8.0
	github.com/blang/semver v3.5.1+incompatible
	github.com/caarlos0/env/v6 v6.9.1
	github.com/canonical/go-dqlite v1.11.0 // indirect
	github.com/cespare/xxhash v1.1.0
	github.com/chromedp/cdproto v0.0.0-20220421002605-e483954745f2
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/foxcpp/go-mockdns v0.0.0-20210729171921-fb145fc6f897
	github.com/fsnotify/fsnotify v1.5.3 // indirect
	github.com/go-co-op/gocron v1.13.0
	github.com/go-gormigrate/gormigrate/v2 v2.0.0
	github.com/go-logr/logr v1.2.3
	github.com/go-logr/zapr v1.2.3 // indirect
	github.com/golang-jwt/jwt v3.2.1+incompatible // indirect
	github.com/golang/mock v1.6.0
	github.com/golangci/golangci-lint v1.45.2
	github.com/google/addlicense v1.0.0 // indirect
	github.com/google/uuid v1.3.0
	github.com/google/wire v0.5.0
	github.com/goph/emperror v0.17.2
	github.com/gotidy/ptr v1.4.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.10.0 // indirect
	github.com/imdario/mergo v0.3.12
	github.com/jpillora/backoff v1.0.0
	github.com/launchdarkly/go-options v1.2.0
	github.com/mattn/go-sqlite3 v2.0.3+incompatible // indirect
	github.com/mikefarah/yq/v2 v2.4.1 // indirect
	github.com/mikefarah/yq/v3 v3.0.0-20201202084205-8846255d1c37
	github.com/mitchellh/mapstructure v1.5.0
	github.com/oliveagle/jsonpath v0.0.0-20180606110733-2e52cf6e6852
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.19.0
	github.com/openshift/api v0.0.0-20200930075302-db52bc4ef99f
	github.com/openshift/cluster-monitoring-operator v0.1.1-0.20210130044457-b344b13b469f
	github.com/operator-framework/api v0.14.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator v0.44.0
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.55.1
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/common v0.28.0
	github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2 v2.0.0-20220419162829-4f86dc8f9651
	// github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2 v2.0.0-20220419162829-4f86dc8f9651 // indirect
	github.com/sasha-s/go-deadlock v0.3.1
	github.com/spf13/cobra v1.4.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.11.0
	github.com/stretchr/testify v1.7.1
	github.com/tcnksm/ghr v0.14.0
	github.com/wadey/gocovmerge v0.0.0-20160331181800-b5bfa59ec0ad
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	go.uber.org/zap v1.21.0 // indirect
	golang.org/x/net v0.0.0-20220421235706-1d1ef9303861
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac
	google.golang.org/genproto v0.0.0-20220421151946-72621c1f0bd3 // indirect
	google.golang.org/grpc v1.45.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.2.0 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	gopkg.in/yaml.v2 v2.4.0
	gorm.io/driver/sqlite v1.3.1 // indirect
	gorm.io/gorm v1.23.4
	k8s.io/api v0.23.5
	k8s.io/apiextensions-apiserver v0.23.5
	k8s.io/apimachinery v0.23.5
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.23.6
	k8s.io/utils v0.0.0-20211116205334-6203023598ed
	sigs.k8s.io/controller-runtime v0.11.2
	sigs.k8s.io/controller-tools v0.3.0 // indirect
	sigs.k8s.io/yaml v1.3.0
)

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/chromedp/chromedp v0.8.0 // indirect
	github.com/chromedp/sysutil v1.0.0 // indirect
	github.com/gobwas/httphead v0.1.0 // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/gobwas/ws v1.1.0 // indirect
	github.com/huandu/xstrings v1.3.1 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.0 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/thediveo/enumflag v0.10.1 // indirect
	golang.org/x/crypto v0.0.0-20220411220226-7b82a4e95df4 // indirect
	golang.org/x/sys v0.0.0-20220422013727-9388b58f7150 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
)

replace (
	github.com/dgrijalva/jwt-go => github.com/redhat-marketplace/jwt v3.2.1+incompatible
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v1.8.2-0.20201015110737-0a7fdd3b7696
	k8s.io/api => k8s.io/api v0.23.0
	k8s.io/client-go => k8s.io/client-go v0.23.0
)
