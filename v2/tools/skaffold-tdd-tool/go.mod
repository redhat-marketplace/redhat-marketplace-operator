module github.com/redhat-marketplace/redhat-marketplace-operator/v2/tools/skaffold-tdd-tool

go 1.16

require (
	emperror.dev/errors v0.8.0
	github.com/GoogleContainerTools/skaffold v1.16.0
	github.com/caarlos0/env/v6 v6.4.0
	github.com/gdamore/tcell/v2 v2.0.1-0.20201017141208-acf90d56d591
	github.com/redhat-marketplace/redhat-marketplace-operator/tests/v2 v2.0.0-20210729050326-8246afe36a7e
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 v2.0.0-20210729050326-8246afe36a7e // indirect
	github.com/rivo/tview v0.0.0-20201118063654-f007e9ad3893
	google.golang.org/grpc v1.40.0
	google.golang.org/protobuf v1.27.1
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v14.2.0+incompatible // Required by OLM
	github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.6.0
	github.com/containerd/containerd v1.4.0-0 => github.com/containerd/containerd v1.4.0
	github.com/coreos/prometheus-operator => github.com/prometheus-operator/prometheus-operator v0.41.0
	github.com/dgrijalva/jwt-go => github.com/redhat-marketplace/jwt v3.2.1+incompatible
	github.com/docker/docker => github.com/docker/docker v17.12.0-ce-rc1.0.20190319215453-e7b5f7dbe98c+incompatible
	github.com/docker/docker v1.14.0-0.20190319215453-e7b5f7dbe98c => github.com/docker/docker v17.12.0-ce-rc1.0.20190319215453-e7b5f7dbe98c+incompatible
	github.com/operator-framework/operator-marketplace => github.com/operator-framework/operator-marketplace v0.0.0-20201110032404-0e3bd3db36a6
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v1.8.2-0.20200609102542-5d7e3e970602
	github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2 => ../../../airgap/v2
	github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2 => ../../../reporter/v2
	github.com/redhat-marketplace/redhat-marketplace-operator/tests/v2 => ../../../tests/v2
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 => ../../
	k8s.io/api => k8s.io/api v0.22.2
	k8s.io/client-go => k8s.io/client-go v0.22.2 // Required by prometheus-operator
)
