module github.com/redhat-marketplace/redhat-marketplace-operator/v2/tools/version

go 1.15

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 v2.0.0-20210223143043-9d906966478f
	github.com/spf13/cobra v1.1.1
)

replace (
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 => ../..
	k8s.io/client-go => k8s.io/client-go v0.19.4
)
