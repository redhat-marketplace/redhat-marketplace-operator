module github.com/redhat-marketplace/redhat-marketplace-operator

go 1.13

require (
	cloud.google.com/go v0.53.0 // indirect
	github.com/Azure/go-autorest/autorest v0.9.3 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.8.1 // indirect
	github.com/Masterminds/semver/v3 v3.1.0
	github.com/Shyp/bump_version v0.0.0-20180222180749-d7594d2951e2
	github.com/banzaicloud/k8s-objectmatcher v1.3.0
	github.com/coreos/prometheus-operator v0.38.0
	github.com/fatih/structtag v1.2.0 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/golang/protobuf v1.4.0 // indirect
	github.com/google/addlicense v0.0.0-20200422172452-68a83edd47bc // indirect
	github.com/google/go-licenses v0.0.0-20200227160636-0fa8c766a591 // indirect
	github.com/google/licenseclassifier v0.0.0-20200402202327-879cb1424de0 // indirect
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
	github.com/otiai10/copy v1.1.1 // indirect
	github.com/prometheus/client_golang v1.5.1
	github.com/prometheus/common v0.9.1
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.6.2
	github.com/stretchr/testify v1.4.0
	github.com/tcnksm/ghr v0.13.0
	github.com/urfave/cli v1.22.2 // indirect
	golang.org/x/crypto v0.0.0-20200429183012-4b2356b1ed79 // indirect
	golang.org/x/net v0.0.0-20200425230154-ff2c4b7c35a0 // indirect
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a // indirect
	golang.org/x/sys v0.0.0-20200428200454-593003d681fa // indirect
	golang.org/x/tools v0.0.0-20200428211428-0c9eba77bc32 // indirect
	google.golang.org/appengine v1.6.6 // indirect
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
