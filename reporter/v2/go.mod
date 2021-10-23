module github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2

go 1.16

require (
	emperror.dev/errors v0.8.0
	github.com/IBM/ibm-cos-sdk-go v1.7.0 // indirect
	github.com/cespare/xxhash v1.1.0
	github.com/go-logr/logr v0.4.0
	github.com/google/uuid v1.2.0
	github.com/google/wire v0.4.0
	github.com/gotidy/ptr v1.3.0
	github.com/meirf/gopart v0.0.0-20180520194036-37e9492a85a8
	github.com/mitchellh/go-homedir v1.1.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.16.0
	github.com/openshift/api v0.0.0-20200930075302-db52bc4ef99f
	github.com/operator-framework/api v0.3.25
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.46.0
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/common v0.26.0
	github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2 v2.0.0-00010101000000-000000000000
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 v2.0.0-00010101000000-000000000000
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.9.0
	golang.org/x/net v0.0.0-20210825183410-e898025ed96a
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f
	google.golang.org/grpc v1.40.0
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.10.2
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v1.8.2-0.20201015110737-0a7fdd3b7696
	github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2 => ../../airgap/v2
	github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2 => ../../metering/v2
	github.com/redhat-marketplace/redhat-marketplace-operator/tests/v2 => ../../tests/v2
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 => ../../v2
	k8s.io/api => k8s.io/api v0.19.4
	k8s.io/client-go => k8s.io/client-go v0.19.4
)
