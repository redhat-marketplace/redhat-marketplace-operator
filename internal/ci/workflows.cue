package ci

import (
	"github.com/SchemaStore/schemastore/src/schemas/json"
	encjson "encoding/json"
)

workflowsDir: *"./" | string @tag(workflowsDir)

workflows: [...{file: string, schema: (json.#Workflow & {})}]
workflows: [
	{
		file:   "test.yml"
		schema: unitTest
	},
]

varPresetGitTag:         "${{ steps.vars.outputs.tag }}"
varPresetVersion:        "${{ steps.vars.outputs.version }}"
varPresetHash:           "${{ steps.vars.outputs.hash}}"
varPresetDockertag:      "${{ steps.vars.outputs.dockertag}}"
varPresetQuayExpiration: "${{ steps.vars.outputs.quayExpiration}}"

#varSteps: [
	_#setBranchPrefixForDev,
	_#setBranchPrefixForFix,
	_#setBranchPrefixForFeature,
	_#step & {
		name: "Get Vars"
		id:   "vars"
		run: """
			echo "::set-output name=version::$(make current-version)"
			echo "::set-output name=tag::sha-$(git rev-parse --short HEAD)"
			echo "::set-output name=hash::$(make current-version)-$(git rev-parse --short HEAD)"
			echo "::set-output name=dockertag::${TAGPREFIX}$(make current-version)-${GITHUB_SHA::8}"
			echo "::set-output name=quayExpiration::${QUAY_EXPIRATION:-never}"
			"""
	},
]

unitTest: _#bashWorkflow & {
	name: "Test"
	on: {
		push: {
			branches: [
				"master",
				"release/**",
				"hotfix/**",
				"develop",
				"feature/**",
				"bugfix/**",
			]
		}
	}
	env: {
		"IMAGE_REGISTRY": "quay.io/rh-marketplace"
	}
	jobs: {
		"test-unit": _#job & {
			name:      "Test"
			"runs-on": _#linuxMachine
			env: {
				"OPERATOR_IMAGE":     "${IMAGE_REGISTRY}/redhat-marketplace-operator:${{ needs.build.outputs.dockertag }}"
				"OPERATOR_IMAGE_TAG": "${{ needs.build.outputs.dockertag }}"
				"TAG":                "${IMAGE_REGISTRY}/redhat-marketplace-operator:${{ needs.preset.outputs.dockertag }}"
			}
			steps: [
				_#checkoutCode,
				_#installGo,
				_#cacheGoModules,
				_#installKubeBuilder,
				_#step & {
					name: "Test"
					run: """
						make test
						"""
				},
			]
		}
	}
}

deploy: _#bashWorkflow & {

}

test: _#bashWorkflow & {
	name: "Test"
	on: {
		push: {
			branches: ["**"] // any branch (including '/' namespaced branches)
			"tags-ignore": ["v*"]
		}
	}

	jobs: {
		start: {
			"runs-on": _#linuxMachine
			if:        "${{ \(_#isCLCITestBranch) }}"
			steps: [
				_#startCLBuild,
			]
		}
		test: {
			strategy:  _#testStrategy
			"runs-on": "${{ matrix.os }}"
			steps: [
				_#installGo,
				_#checkoutCode,
				_#cacheGoModules,
				_#goGenerate,
				_#goTest,
				_#goTestRace,
				_#goReleaseCheck,
				_#pullThroughProxy,
				_#failCLBuild,
			]
		}
		mark_ci_success: {
			"runs-on": _#linuxMachine
			if:        "${{ \(_#isCLCITestBranch) }}"
			needs:     "test"
			steps: [
				_#passCLBuild,
			]
		}
		delete_build_branch: {
			"runs-on": _#linuxMachine
			if:        "${{ \(_#isCLCITestBranch) && always() }}"
			needs:     "test"
			steps: [
				_#step & {
					run: """
						\(_#tempCueckooGitDir)
						git push https://github.com/cuelang/cue :${GITHUB_REF#\(_#branchRefPrefix)}
						"""
				},
			]
		}
	}

	// _#isCLCITestBranch is an expression that evaluates to true
	// if the job is running as a result of a CL triggered CI build
	_#isCLCITestBranch: "startsWith(github.ref, '\(_#branchRefPrefix)ci/')"

	// _#isMaster is an expression that evaluates to true if the
	// job is running as a result of a master commit push
	_#isMaster: "github.ref == '\(_#branchRefPrefix)master'"

	_#pullThroughProxy: _#step & {
		name: "Pull this commit through the proxy on master"
		run: """
			v=$(git rev-parse HEAD)
			cd $(mktemp -d)
			go mod init mod.com
			GOPROXY=https://proxy.golang.org go get -d cuelang.org/go@$v
			"""
		if: "${{ \(_#isMaster) }}"
	}

	_#startCLBuild: _#step & {
		name: "Update Gerrit CL message with starting message"
		run:  (_#gerrit._#setCodeReview & {
			#args: message: "Started the build... see progress at ${{ github.event.repository.html_url }}/actions/runs/${{ github.run_id }}"
		}).res
	}

	_#failCLBuild: _#step & {
		if:   "${{ \(_#isCLCITestBranch) && failure() }}"
		name: "Post any failures for this matrix entry"
		run:  (_#gerrit._#setCodeReview & {
			#args: {
				message: "Build failed for ${{ runner.os }}-${{ matrix.go-version }}; see ${{ github.event.repository.html_url }}/actions/runs/${{ github.run_id }} for more details"
				labels: {
					"Code-Review": -1
				}
			}
		}).res
	}

	_#passCLBuild: _#step & {
		name: "Update Gerrit CL message with success message"
		run:  (_#gerrit._#setCodeReview & {
			#args: {
				message: "Build succeeded for ${{ github.event.repository.html_url }}/actions/runs/${{ github.run_id }}"
				labels: {
					"Code-Review": 1
				}
			}
		}).res
	}

	_#gerrit: {
		// _#setCodeReview assumes that it is invoked from a job where
		// _#isCLCITestBranch is true
		_#setCodeReview: {
			#args: {
				message: string
				labels?: {
					"Code-Review": int
				}
			}
			res: #"""
			curl -f -s -H "Content-Type: application/json" --request POST --data '\#(encjson.Marshal(#args))' -b ~/.gitcookies https://cue-review.googlesource.com/a/changes/$(basename $(dirname $GITHUB_REF))/revisions/$(basename $GITHUB_REF)/review
			"""#
		}
	}
}

test_dispatch: _#bashWorkflow & {

	name: "Test Dispatch"
	on: ["repository_dispatch"]
	jobs: {
		start: {
			if:        "${{ startsWith(github.event.action, 'Build for refs/changes/') }}"
			"runs-on": _#linuxMachine
			steps: [
				_#step & {
					name: "Checkout ref"
					run:  """
						\(_#tempCueckooGitDir)
						git fetch https://cue-review.googlesource.com/cue ${{ github.event.client_payload.ref }}
						git checkout -b ci/${{ github.event.client_payload.changeID }}/${{ github.event.client_payload.commit }} FETCH_HEAD
						git push https://github.com/cuelang/cue ci/${{ github.event.client_payload.changeID }}/${{ github.event.client_payload.commit }}
						"""
				},
			]
		}
	}
}

release: _#bashWorkflow & {

	name: "Release"
	on: push: tags: ["v*"]
	jobs: {
		goreleaser: {
			"runs-on": _#linuxMachine
			steps: [{
				name: "Checkout code"
				uses: "actions/checkout@v2"
			}, {
				name: "Unshallow" // required for the changelog to work correctly.
				run:  "git fetch --prune --unshallow"
			}, {
				name: "Run GoReleaser"
				env: GITHUB_TOKEN: "${{ secrets.ACTIONS_GITHUB_TOKEN }}"
				uses: "docker://goreleaser/goreleaser:latest"
				with: args: "release --rm-dist"
			}]
		}
		docker: {
			name:      "docker"
			"runs-on": _#linuxMachine
			steps: [{
				name: "Check out the repo"
				uses: "actions/checkout@v2"
			}, {
				name: "Set version environment"
				run: """
					CUE_VERSION=$(echo ${GITHUB_REF##refs/tags/v})
					echo \"CUE_VERSION=$CUE_VERSION\"
					echo \"CUE_VERSION=$(echo $CUE_VERSION)\" >> $GITHUB_ENV
					"""
			}, {
				name: "Push to Docker Hub"
				env: {
					DOCKER_BUILDKIT: 1
					GOLANG_VERSION:  1.14
					CUE_VERSION:     "${{ env.CUE_VERSION }}"
				}
				uses: "docker/build-push-action@v1"
				with: {
					tags:           "${{ env.CUE_VERSION }},latest"
					repository:     "cuelang/cue"
					username:       "${{ secrets.DOCKER_USERNAME }}"
					password:       "${{ secrets.DOCKER_PASSWORD }}"
					tag_with_ref:   false
					tag_with_sha:   false
					target:         "cue"
					always_pull:    true
					build_args:     "GOLANG_VERSION=${{ env.GOLANG_VERSION }},CUE_VERSION=v${{ env.CUE_VERSION }}"
					add_git_labels: true
				}
			}]
		}
	}
}

rebuild_tip_cuelang_org: _#bashWorkflow & {

	name: "Push to tip"
	on: push: branches: ["master"]
	jobs: push: {
		"runs-on": _#linuxMachine
		steps: [{
			name: "Rebuild tip.cuelang.org"
			run:  "curl -f -X POST -d {} https://api.netlify.com/build_hooks/${{ secrets.CuelangOrgTipRebuildHook }}"
		}]
	}
}

_#bashWorkflow: json.#Workflow & {
	jobs: [string]: defaults: run: shell: "bash"
}

// TODO: drop when cuelang.org/issue/390 is fixed.
// Declare definitions for sub-schemas
_#job:  ((json.#Workflow & {}).jobs & {x: _}).x
_#step: ((_#job & {steps:                 _}).steps & [_])[0]

// We need at least go1.14 for code generation
_#codeGenGo: "1.14.9"

_#linuxMachine:   "ubuntu-20.04"
_#macosMachine:   "macos-10.15"
_#windowsMachine: "windows-2019"

_#testStrategy: {
	"fail-fast": false
	matrix: {
		// Use a stable version of 1.14.x for go generate
		"go-version": ["1.13.x", _#codeGenGo, "1.15.x"]
		os: [_#linuxMachine, _#macosMachine, _#windowsMachine]
	}
}

_#cancelPreviousRun: _#step & {
	name: "Cancel Previous Run"
	uses: "styfle/cancel-workflow-action@0.4.1"
	with: "access_token": "${{ github.token }}"
}

_#installGo: _#step & {
	name: "Install Go"
	uses: "actions/setup-go@v2"
	with: "go-version": _#goVersion
}

_#goVersion: "1.15.6"

_#checkoutCode: _#step & {
	name: "Checkout code"
	uses: "actions/checkout@v2"
}

_#cacheGoModules: _#step & {
	name: "Cache Go modules"
	uses: "actions/cache@v2"
	with: {
		path:           "~/go/pkg/mod"
		key:            "${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}"
		"restore-keys": "${{ runner.os }}-\(_#goVersion)-go-"
	}
}

_#goGenerate: _#step & {
	name: "Generate"
	run:  "go generate ./..."
	// The Go version corresponds to the precise version specified in
	// the matrix. Skip windows for now until we work out why re-gen is flaky
	if: "matrix.go-version == '\(_#codeGenGo)' && matrix.os != '\(_#windowsMachine)'"
}

_#goTest: _#step & {
	name: "Test"
	run:  "go test ./..."
}

_#goTestRace: _#step & {
	name: "Test with -race"
	run:  "go test -race ./..."
}

_#goReleaseCheck: _#step & {
	name: "gorelease check"
	run:  "go run golang.org/x/exp/cmd/gorelease"
}

_#loadGitTagPushed: _#step & {
	name: "Get if gittag is pushed"
	id:   "tag"
	run: """
		VERSION=$(make current-version)
		RESULT=$(git tag --list | grep -E "$VERSION")
		IS_TAGGED=false
		if [ "$RESULT" != "" ] ; then
		  IS_TAGGED=true
		"""
}

_#branchRefPrefix: "refs/heads/"

_#tempCueckooGitDir: """
	mkdir tmpgit
	cd tmpgit
	git init
	git config user.name cueckoo
	git config user.email cueckoo@gmail.com
	git config http.https://github.com/.extraheader "AUTHORIZATION: basic $(echo -n cueckoo:${{ secrets.CUECKOO_GITHUB_PAT }} | base64)"
	"""

_#setBranchOutput: [
	_#setBranchPrefixForDev,
	_#setBranchPrefixForFix,
	_#setBranchPrefixForFeature,
]

_#setBranchPrefixForDev: (_#vars._#setBranchPrefix & {
	#args: {
		name:      "dev"
		if:        "github.event_name == 'push' && github.ref == 'refs/heads/develop'"
		tagPrefix: "dev-"
	}
}).res

_#setBranchPrefixForFix: (_#vars._#setBranchPrefix & {
	#args: {
		name:           "fix"
		if:             "github.event_name == 'push' && startsWith(github.ref,'refs/heads/bugfix/')"
		tagPrefix:      "bugfix-${NAME}-"
		quayExpiration: "1w"
	}
}).res

_#setBranchPrefixForFeature: (_#vars._#setBranchPrefix & {
	#args: {
		name:           "feature"
		if:             "github.event_name == 'push' && startsWith(github.ref,'refs/heads/feature/')"
		tagPrefix:      "feat-${NAME}-"
		quayExpiration: "1w"
	}
}).res

_#installKubeBuilder: _#step & {
	name: "Install Kubebuilder"
	run: """
		os=$(go env GOOS)
		arch=$(go env GOARCH)

		# download kubebuilder and extract it to tmp
		curl -L https://go.kubebuilder.io/dl/2.3.1/${os}/${arch} | tar -xz -C /tmp/

		# move to a long-term location and put it on your path
		# (you'll need to set the KUBEBUILDER_ASSETS env var if you put it somewhere else)
		sudo mv /tmp/kubebuilder_2.3.1_${os}_${arch} /usr/local/kubebuilder
		echo "/usr/local/kubebuilder/bin" >> $GITHUB_PATH
		"""
}

_#installOperatorSDK: _#step & {
	name: "Install operatorsdk"
	run: """
		export ARCH=$(case $(arch) in x86_64) echo -n amd64 ;; aarch64) echo -n arm64 ;; *) echo -n $(arch) ;; esac)
		export OS=$(uname | awk '{print tolower($0)}')
		export OPERATOR_SDK_DL_URL=https://github.com/operator-framework/operator-sdk/releases/latest/download
		curl -LO ${OPERATOR_SDK_DL_URL}/operator-sdk_${OS}_${ARCH}
		gpg --recv-keys 052996E2A20B5C7E
		curl -LO ${OPERATOR_SDK_DL_URL}/checksums.txt
		curl -LO ${OPERATOR_SDK_DL_URL}/checksums.txt.asc
		gpg -u "Operator SDK (release) <cncf-operator-sdk@cncf.io>" --verify checksums.txt.asc
		grep operator-sdk_${OS}_${ARCH} checksums.txt | sha256sum -c -
		chmod +x operator-sdk_${OS}_${ARCH} && sudo mv operator-sdk_${OS}_${ARCH} /usr/local/bin/operator-sdk
		"""
}

_#vars: {
	// _#setBranchPrefix will set the branch prefix vars
	_#setBranchPrefix: {
		#args: {
			name:           string
			if:             string
			tagPrefix:      string
			quayExpiration: string | *""
		}
		res: _#step & {
			name: #"Setting tags for \#(#args.name)"#
			if:   #"\#(#args["if"])"#
			run:  #"""
      echo "TAGPREFIX=\#(#args.tagPrefix)" >> $GITHUB_ENV
      if [ "\#(#args.quayExpiration)" != "" ]; then
        echo "QUAY_EXPIRATION=\#(#args.quayExpiration)" >> $GITHUB_ENV
      fi
      """#
		}
	}
}

_#turnStyleStep: _#step & {
	name: "Turnstyle"
	uses: "softprops/turnstyle@v1"
	with: "continue-after-seconds": 45
	env: "GITHUB_TOKEN":            "${{ secrets.GITHUB_TOKEN }}"
}
