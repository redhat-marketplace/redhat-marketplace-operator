module github.ibm.com/symposium/redhat-marketplace-operator

go 1.13

require (
	github.com/Masterminds/semver/v3 v3.1.0
	github.com/Shyp/bump_version v0.0.0-20180222180749-d7594d2951e2
	github.com/banzaicloud/k8s-objectmatcher v1.2.2
	github.com/coreos/prometheus-operator v0.38.0
	github.com/go-logr/logr v0.1.0
	github.com/google/uuid v1.1.1
	github.com/google/wire v0.4.0
	github.com/gotidy/ptr v1.0.1
	github.com/imdario/mergo v0.3.8
	github.com/launchdarkly/go-options v1.0.0
	github.com/mikefarah/yq/v3 v3.0.0-20200415014842-6f0a329331f9
	github.com/mitchellh/mapstructure v1.1.2
	github.com/noqcks/gucci v0.0.4
	github.com/operator-framework/api v0.1.1
	github.com/operator-framework/operator-marketplace v0.0.0-20200303235415-12497b0b9a6b
	github.com/operator-framework/operator-sdk v0.16.1-0.20200401233323-bca8959696d5
	github.com/prometheus/client_golang v1.5.1
	github.com/prometheus/common v0.9.1
	github.com/redhat-marketplace/redhat-marketplace-operator v0.0.0-20200430020916-8854a3377b4a
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.6.2
	github.com/stretchr/testify v1.4.0
	github.com/tcnksm/ghr v0.13.0
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kube-openapi v0.0.0-20191107075043-30be4d16710a
	sigs.k8s.io/controller-runtime v0.5.2
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.17.4 // Required by prometheus-operator
)
