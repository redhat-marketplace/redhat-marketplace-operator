module github.com/redhat-marketplace/redhat-marketplace-operator/v2/tools/version

go 1.15

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/redhat-marketplace/redhat-marketplace-operator v0.0.0-20201211175424-6b3ce5b64e99
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 v2.0.0-20210223143043-9d906966478f // indirect
	github.com/spf13/cobra v1.1.1
)

replace (
	github.com/redhat-marketplace/redhat-marketplace-operator/v2 => ../..
)
