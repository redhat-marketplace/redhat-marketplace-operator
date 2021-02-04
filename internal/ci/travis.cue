package ci

import (
	json "github.com/SchemaStore/schemastore/src/schemas/json/travis"
	"strings"
)

travisDir: *"." | string @tag(travisDir)

travis: [...{file: string, schema: (json.#Travis & {})}]
travis: [
	{
		file:   ".travis.yml"
		schema: travisSchema
	},
]

_#archs: ["amd64", "ppc64le", "s390x"]
_#registry: "quay.io/rh-marketplace"

travisSchema: {
	dist:     "focal"
	language: "go"
	services: ["docker"]
	"before_script": [
		"go get github.com/onsi/ginkgo/ginkgo",
		"docker pull docker.io/docker/dockerfile:experimental",
		"docker pull docker.io/docker/dockerfile-copy:v0.1.9",
		"VERSION=`cd v2/tools && go run ./version/main.go`-${TRAVIS_BUILD_NUMBER}",
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
				stage:  "manifest"
				script: """
					if [ "x$VERSION" = "x" ]; then VERSION=${TRAVIS_COMMIT}; fi
					echo "making manifest for $VERSION"
					make docker-manifest
					"""
			},
      {
        stage: "retag"
        if: "branch = master OR branch = develop OR branch =~ /^(release|hotfix)\\/.*/"
        script: retagCommand
      }
		]
	}
	script: [
		"docker --version",
		"if [ \"x$VERSION\" = \"x\" ]; then VERSION=${TRAVIS_COMMIT}; fi",
		"VERSION=${VERSION}-${ARCH}",
		"echo  ${VERSION}",
		"echo \"Login to Quay.io docker account...\"",
		"docker login -u=\"${ROBOT_USER_NAME}\" -p=\"${ROBOT_PASS_PHRASE}\" quay.io",
    """
    if [ "$(go env GOARCH)" != "s390x" ]; then
    \(_#installKubeBuilder.run)\n
    export PATH=$PATH:/usr/local/kubebuilder/bin
    make operator/test-ci-unit
    fi
    """,
		"echo \"Building the Red Hat Marketplace operator images for ppc64l...\"",
		"make docker-build",
		"make docker-push",
		"echo \"Docker Image push to quay.io is done !\"",
	]
}

_#operatorRepoName:    "redhat-marketplace-operator"
_#meteringRepoName:    "redhat-marketplace-metric-state"
_#reporterRepoName:    "redhat-marketplace-reporter"
_#authcheckerRepoName: "redhat-marketplace-authchecker"

_#operatorRedHatOSPID:    "scan.connect.redhat.com/ospid-c93f69b6-cb04-437b-89d6-e5220ce643cd"
_#reporterRedHatOSPID:    "scan.connect.redhat.com/ospid-faa0f295-e195-4bcc-a3fc-a4b97ada317e"
_#meteringRedHatOSPID:    "scan.connect.redhat.com/ospid-9b9b0dbe-7adc-448e-9385-a556714a09c4"
_#authcheckerRedHatOSPID: "scan.connect.redhat.com/ospid-ffed416e-c18d-4b88-8660-f586a4792785"

_#ospids: [{
	from: "\(_#registry)/\(_#operatorRepoName):$VERSION"
	to:   "\(_#operatorRedHatOSPID)/\(_#operatorRepoName):$VERSION"
},
	{
		from: "\(_#registry)/\(_#reporterRepoName):$VERSION"
		to:   "\(_#reporterRedHatOSPID)/\(_#reporterRepoName):$VERSION"
	},
	{
		from: "\(_#registry)/\(_#meteringRepoName):$VERSION"
		to:   "\(_#meteringRedHatOSPID)/\(_#meteringRepoName):$VERSION"
	},
	{
		from: "\(_#registry)/\(_#authcheckerRepoName):$VERSION"
		to:   "\(_#authcheckerRedHatOSPID)/\(_#authcheckerRepoName):$VERSION"
	}]

retagCommand: strings.Join([ for k, v in _#ospids {
	"""
docker tag \(v.from) \(v.to)
docker push \(v.to)
"""
}], "\n")
