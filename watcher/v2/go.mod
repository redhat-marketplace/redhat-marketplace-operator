module github.com/redhat-marketplace/redhat-marketplace-operator/watcher/v2

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	github.com/google/wire v0.4.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/openshift/api v0.0.0-20200930075302-db52bc4ef99f
	github.com/operator-framework/api v0.3.25
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.46.0
	github.com/prometheus/client_golang v1.11.0
	github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2 v2.0.0-20211130000046-e489167aa34d
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 v2.0.0-00010101000000-000000000000
	github.com/sasha-s/go-deadlock v0.2.0
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	golang.org/x/net v0.0.0-20210525063256-abc453219eb5
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/kube-state-metrics v1.9.7
	sigs.k8s.io/controller-runtime v0.10.2
)

replace (
	github.com/dgrijalva/jwt-go => github.com/redhat-marketplace/jwt v3.2.1+incompatible
	github.com/operator-framework/operator-lifecycle-manager => github.com/operator-framework/operator-lifecycle-manager v0.17.0
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v1.8.2-0.20201015110737-0a7fdd3b7696
	github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2 => ../../metering/v2
	github.com/redhat-marketplace/redhat-marketplace-operator/tests/v2 => ../../tests/v2
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 => ../../v2
	k8s.io/api => k8s.io/api v0.22.2
	k8s.io/client-go => k8s.io/client-go v0.22.2
)
