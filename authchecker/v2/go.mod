module github.com/redhat-marketplace/redhat-marketplace-operator/authchecker/v2

go 1.16

require (
	emperror.dev/errors v0.8.0
	github.com/go-logr/logr v1.2.0
	github.com/spf13/cobra v1.2.1
	k8s.io/api v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.11.0
)

replace (
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v1.8.2-0.20201015110737-0a7fdd3b7696
	github.com/redhat-marketplace/redhat-marketplace-operator/tests/v2 => ../../tests/v2
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 => ../../v2
	k8s.io/api => k8s.io/api v0.23.0
	k8s.io/client-go => k8s.io/client-go v0.23.0
)

replace (
	github.com/dgrijalva/jwt-go => github.com/golang-jwt/jwt/v4 v4.4.0
	github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
)
