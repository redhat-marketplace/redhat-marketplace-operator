module github.com/redhat-marketplace/redhat-marketplace-operator

go 1.15

require (
	emperror.dev/errors v0.7.0
	github.com/Azure/go-autorest/autorest v0.11.2 // indirect
	github.com/Masterminds/semver/v3 v3.1.0
	github.com/Shyp/bump_version v0.0.0-20180222180749-d7594d2951e2
	github.com/banzaicloud/k8s-objectmatcher v1.3.0
	github.com/blang/semver v3.5.1+incompatible
	github.com/caarlos0/env/v6 v6.3.0
	github.com/cespare/xxhash v1.1.0
	github.com/coreos/prometheus-operator v0.40.0
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/fatih/structtag v1.2.0 // indirect
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32 // indirect
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/go-logr/logr v0.1.0
	github.com/golang/mock v1.4.3
	github.com/golangci/golangci-lint v1.27.0
	github.com/google/addlicense v0.0.0-20200906110928-a0294312aa76 // indirect
	github.com/google/go-cmp v0.5.1 // indirect
	github.com/google/uuid v1.1.1
	github.com/google/wire v0.4.0
	github.com/goph/emperror v0.17.2
	github.com/gophercloud/gophercloud v0.12.0 // indirect
	github.com/gotidy/ptr v1.0.1
	github.com/imdario/mergo v0.3.9
	github.com/jpillora/backoff v1.0.0
	github.com/launchdarkly/go-options v1.0.0
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/meirf/gopart v0.0.0-20180520194036-37e9492a85a8
	github.com/mikefarah/yq/v3 v3.0.0-20200415014842-6f0a329331f9
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/mapstructure v1.3.2
	github.com/onsi/ginkgo v1.13.0
	github.com/onsi/gomega v1.10.1
	github.com/openshift/api v0.0.0-20200205133042-34f0ec8dab87
	github.com/openshift/origin v0.0.0-20160503220234-8f127d736703
	github.com/operator-framework/api v0.3.8
	github.com/operator-framework/operator-marketplace v0.0.0-20200303235415-12497b0b9a6b
	github.com/operator-framework/operator-sdk v0.19.2
	github.com/pelletier/go-toml v1.8.0 // indirect
	github.com/petermattis/goid v0.0.0-20180202154549-b0b1615b78e5 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/alertmanager v0.21.0 // indirect
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/common v0.10.0
	github.com/sasha-s/go-deadlock v0.2.0
	github.com/sirupsen/logrus v1.6.0 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.6.2
	github.com/stretchr/testify v1.5.1
	github.com/tcnksm/ghr v0.13.0
	github.com/wadey/gocovmerge v0.0.0-20160331181800-b5bfa59ec0ad // indirect
	golang.org/x/net v0.0.0-20200822124328-c89045814202
	golang.org/x/sys v0.0.0-20200814200057-3d37ad5750ed // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	golang.org/x/tools v0.0.0-20200914163123-ea50a3c84940 // indirect
	gopkg.in/yaml.v2 v2.3.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	honnef.co/go/tools v0.0.1-2020.1.5 // indirect
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.18.6
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20200410145947-61e04a5be9a6
	k8s.io/kube-state-metrics v1.7.2
	k8s.io/kubectl v0.18.2
	k8s.io/utils v0.0.0-20200603063816-c1c6865ac451
	sigs.k8s.io/controller-runtime v0.6.1
	sigs.k8s.io/kind v0.9.0 // indirect
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v14.2.0+incompatible // Required by OLM
	github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.6.0
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v1.8.2-0.20200609102542-5d7e3e970602
	k8s.io/client-go => k8s.io/client-go v0.18.3 // Required by prometheus-operator
)
