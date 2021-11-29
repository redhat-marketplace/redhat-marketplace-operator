module github.com/redhat-marketplace/redhat-marketplace-operator/v2/tools/version

go 1.16

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 v2.0.0-20210223143043-9d906966478f
	github.com/spf13/cobra v1.1.3
)

replace (
	github.com/dgrijalva/jwt-go => github.com/redhat-marketplace/jwt v3.2.1+incompatible
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 => ../..
	k8s.io/client-go => k8s.io/client-go v0.22.2
)
