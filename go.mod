module github.com/redhat-marketplace/redhat-marketplace-operator/v2

go 1.16

replace (
	github.com/banzaicloud/k8s-objectmatcher => github.com/banzaicloud/k8s-objectmatcher v1.6.1
	github.com/dgrijalva/jwt-go => github.com/redhat-marketplace/jwt v3.2.1+incompatible
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v1.8.2-0.20201015110737-0a7fdd3b7696
	k8s.io/api => k8s.io/api v0.23.0
	k8s.io/client-go => k8s.io/client-go v0.23.0
)

require (
	emperror.dev/errors v0.8.1
	github.com/banzaicloud/k8s-objectmatcher v0.0.0-00010101000000-000000000000
	github.com/canonical/go-dqlite v1.11.0
	github.com/go-co-op/gocron v1.13.0
	github.com/go-gormigrate/gormigrate/v2 v2.0.0
	github.com/go-logr/logr v1.2.3
	github.com/go-logr/zapr v1.2.3
	github.com/google/uuid v1.3.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.10.0
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.19.0
	github.com/pkg/errors v0.9.1
	github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2 v2.0.0-20220419162829-4f86dc8f9651
	github.com/spf13/cobra v1.4.0
	github.com/spf13/viper v1.11.0
	go.uber.org/zap v1.21.0
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/genproto v0.0.0-20220422154200-b37d22cd5731
	google.golang.org/grpc v1.46.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.2.0
	google.golang.org/protobuf v1.28.0
	gorm.io/driver/sqlite v1.3.2
	gorm.io/gorm v1.23.4
	sigs.k8s.io/controller-runtime v0.11.2
)
