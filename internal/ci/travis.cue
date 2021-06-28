package ci

import (
	json "github.com/SchemaStore/schemastore/src/schemas/json/travis"
	"strings"
	encjson "encoding/json"
	"strconv"
)

travisDir: *"." | string @tag(travisDir)

travis: [...{file: string, schema: (json.#Travis & {})}]
travis: []

travisSchema: {
	version: "~> 1.0"
	dist:    "focal"
	if: """
		(type = push && (branch = master || branch = develop)) || (type = pull_request && (branch = master))
		"""
	language: "go"
	services: ["docker"]
	"before_script": [
		"go get github.com/onsi/ginkgo/ginkgo",
		"docker pull docker.io/docker/dockerfile:experimental",
		"docker pull docker.io/docker/dockerfile-copy:v0.1.9",
		"export VERSION=`cd v2/tools/version && go run ./main.go`-${TRAVIS_COMMIT}",
		"docker login -u=\"${ROBOT_USER_NAME}\" -p=\"${ROBOT_PASS_PHRASE}\" quay.io",
	]
	go: _#goVersion
	env: global: ["IMAGE_REGISTRY=\(_#registry) DOCKER_CLI_EXPERIMENTAL=enabled DOCKER_BUILDKIT=1 QUAY_EXPIRATION=never BUILDX=false"]
	jobs: {
		include: [
			for k, v in _#archs {
				{
					stage: "push"
					arch:  v
					env:   "ARCH=\(v)"
				}
			},
			{
				stage: "manifest"
				if:    len(_#archs) > 1
				env:   #"ARCHS="\#(strings.Join(_#archs, " "))""#
				script: """
					echo "making manifest for $VERSION"
					make docker-manifest
					"""
			},
			{
				#args: {
					event_type: "bundle"
					client_payload: {
						sha:                 "$TRAVIS_COMMIT"
						branch:              "$TRAVIS_BRANCH"
						pull_request_branch: "$TRAVIS_PULL_REQUEST_BRANCH"
						pull_request:        "$TRAVIS_PULL_REQUEST"
					}
				}
				stage:  "bundle"
				if:     "(type = pull_request && head_branch =~ \(_#branchTarget)) || (type = push && branch =~ \(_#branchTarget))"
				script: """
					curl -X POST -H "Authorization: token ${GITHUB_TOKEN}" -H "Accept: application/vnd.github.v3+json" https://api.github.com/repos/redhat-marketplace/redhat-marketplace-operator/dispatches -d \(strconv.Quote(encjson.Marshal(#args)))
					"""
			},
		]
	}
	script: [
		"docker --version",
		"export VERSION=${VERSION}-${ARCH}",
		"echo  ${VERSION}",
		"echo \"Login to Quay.io docker account...\"",
		"""
    echo "run tests if not s390x because kubebuilder has no binaries for it"
    if [ "$(go env GOARCH)" = "amd64" ]; then
    \(_#installKubeBuilder.run)\n
    export PATH=$PATH:/usr/local/kubebuilder/bin
    make operator/test-ci-unit
    fi
    """,
		"echo \"Building the Red Hat Marketplace operator images for ${ARCH}...\"",
		"make save-licenses",
		"make docker-build",
		"make docker-push",
		"echo \"Docker Image push to quay.io is done !\"",
	]
}
