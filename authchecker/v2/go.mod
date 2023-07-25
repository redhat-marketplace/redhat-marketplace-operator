module github.com/redhat-marketplace/redhat-marketplace-operator/authchecker/v2

go 1.19

require (
	github.com/go-logr/logr v1.2.4
	github.com/onsi/ginkgo/v2 v2.8.0
	github.com/onsi/gomega v1.25.0
	github.com/spf13/cobra v1.7.0
	github.com/stretchr/testify v1.8.2
	go.uber.org/zap v1.24.0
	k8s.io/apimachinery v0.27.1
	k8s.io/client-go v1.5.2
	sigs.k8s.io/controller-runtime v0.14.6
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/zapr v1.2.4 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/imdario/mergo v0.3.15 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/oauth2 v0.8.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/term v0.8.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/klog/v2 v2.100.1 // indirect
	k8s.io/utils v0.0.0-20230505201702-9f6742963106 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace (
	github.com/dgrijalva/jwt-go => github.com/golang-jwt/jwt/v5 v5.0.0
	k8s.io/api => k8s.io/api v0.25.9
	k8s.io/apimachinery => k8s.io/apimachinery v0.25.9
	k8s.io/client-go => k8s.io/client-go v0.25.9
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.13.1
)

replace github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
