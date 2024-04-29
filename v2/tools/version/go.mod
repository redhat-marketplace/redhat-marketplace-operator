module github.com/redhat-marketplace/redhat-marketplace-operator/v2/tools/version

go 1.20

require (
	github.com/Masterminds/semver/v3 v3.2.1
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 v2.0.0-20230512153729-85566ee3f06b
	github.com/spf13/cobra v1.7.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)

replace (
	github.com/banzaicloud/k8s-objectmatcher => github.com/banzaicloud/k8s-objectmatcher v1.6.1
	github.com/dgrijalva/jwt-go => github.com/golang-jwt/jwt/v4 v4.5.0
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 => ../..
	k8s.io/api => k8s.io/api v0.28.8
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.28.8
	k8s.io/apimachinery => k8s.io/apimachinery v0.28.8
	k8s.io/apiserver => k8s.io/apiserver v0.28.8
	k8s.io/client-go => k8s.io/client-go v0.28.8
	k8s.io/component-base => k8s.io/component-base v0.28.8
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.28.8
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.16.5
)
