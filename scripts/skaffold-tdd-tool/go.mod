module github.com/redhat-marketplace/redhat-marketplace-operator/scripts/skaffold-tdd-tool

go 1.15

require (
	github.com/GoogleContainerTools/skaffold v1.16.0
	github.com/caarlos0/env/v6 v6.3.0
	github.com/fatih/color v1.10.0
	github.com/go-logr/logr v0.2.1 // indirect
	github.com/go-logr/zapr v0.2.0 // indirect
	github.com/google/go-cmp v0.5.2 // indirect
	github.com/googleapis/gnostic v0.5.3 // indirect
	github.com/imdario/mergo v0.3.10 // indirect
	github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/onsi/gomega v1.10.2 // indirect
	github.com/operator-framework/operator-marketplace v0.0.0-20201110032404-0e3bd3db36a6 // indirect
	github.com/redhat-marketplace/redhat-marketplace-operator v0.0.0-00010101000000-000000000000
	go.uber.org/goleak v1.1.10 // indirect
	go.uber.org/zap v1.15.0 // indirect
	google.golang.org/grpc v1.33.2
	google.golang.org/protobuf v1.25.0
	k8s.io/apiextensions-apiserver v0.19.2 // indirect
	k8s.io/utils v0.0.0-20200912215256-4140de9c8800 // indirect
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v14.2.0+incompatible // Required by OLM
	github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.6.0
	github.com/containerd/containerd v1.4.0-0 => github.com/containerd/containerd v1.4.0
	github.com/coreos/prometheus-operator => github.com/prometheus-operator/prometheus-operator v0.41.0
	github.com/docker/docker => github.com/docker/docker v17.12.0-ce-rc1.0.20190319215453-e7b5f7dbe98c+incompatible
	github.com/docker/docker v1.14.0-0.20190319215453-e7b5f7dbe98c => github.com/docker/docker v17.12.0-ce-rc1.0.20190319215453-e7b5f7dbe98c+incompatible
	github.com/operator-framework/operator-marketplace => github.com/operator-framework/operator-marketplace v0.0.0-20201110032404-0e3bd3db36a6
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v1.8.2-0.20200609102542-5d7e3e970602
	github.com/redhat-marketplace/redhat-marketplace-operator => ../..
	k8s.io/client-go => k8s.io/client-go v0.19.4 // Required by prometheus-operator
)
