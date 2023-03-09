module github.com/redhat-marketplace/redhat-marketplace-operator/v2/tools/version

go 1.19

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 v2.0.0-20210223143043-9d906966478f
	github.com/spf13/cobra v1.2.1
)

require (
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)

replace (
	github.com/banzaicloud/k8s-objectmatcher => github.com/banzaicloud/k8s-objectmatcher v1.6.1
	github.com/dgrijalva/jwt-go => github.com/golang-jwt/jwt/v4 v4.4.0
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 => ../..
	k8s.io/client-go => k8s.io/client-go v0.23.0
)
