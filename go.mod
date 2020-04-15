module github.ibm.com/symposium/redhat-marketplace-operator

go 1.13

require (
	cloud.google.com/go v0.53.0 // indirect
	github.com/Azure/go-autorest/autorest v0.9.3 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.8.1 // indirect
	github.com/Shyp/bump_version v0.0.0-20180222180749-d7594d2951e2 // indirect
	github.com/coreos/prometheus-operator v0.38.0
	github.com/fatih/structtag v1.2.0 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/google/uuid v1.1.1
	github.com/google/wire v0.4.0
	github.com/gotidy/ptr v1.0.1
	github.com/imdario/mergo v0.3.8
	github.com/launchdarkly/go-options v1.0.0
	github.com/mitchellh/mapstructure v1.2.2
	github.com/noqcks/gucci v0.0.4
	github.com/operator-framework/operator-marketplace v0.0.0-20200303235415-12497b0b9a6b
	github.com/operator-framework/operator-sdk v0.16.1-0.20200401233323-bca8959696d5
	github.com/prometheus/client_golang v1.5.1
	github.com/prometheus/common v0.9.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.6.2
	github.com/urfave/cli v1.22.2 // indirect
	golang.org/x/sys v0.0.0-20200223170610-d5e6a3e2c0ae // indirect
	golang.org/x/tools v0.0.0-20200410132612-ae9902aceb98 // indirect
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
