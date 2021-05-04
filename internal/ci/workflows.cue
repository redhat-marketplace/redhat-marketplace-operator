package ci

import (
	"strings"
	"strconv"
	json "github.com/SchemaStore/schemastore/src/schemas/json/github"
	encjson "encoding/json"
)

workflowsDir: *"./" | string @tag(workflowsDir)

workflows: [...{file: string, schema: (json.#Workflow & {})}]
workflows: [
	{
		file:   "branch_build.yml"
		schema: branch_build
	},
]

varPresetGitTag:         "${{ needs.preset.outputs.tag }}"
varPresetVersion:        "${{ needs.preset.outputs.version }}"
varPresetHash:           "${{ needs.preset.outputs.hash}}"
varPresetDockertag:      "${{ needs.preset.outputs.dockertag}}"
varPresetQuayExpiration: "${{ needs.preset.outputs.quayExpiration}}"

unit_test: _#bashWorkflow & {
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

auto_tag: _#bashWorkflow & {
  name: "Tag Latest Release"
  on: push: branches: ["master"]
  jobs: {
    tag: _#job & {
    "runs-on": "ubuntu-latest"
    steps: [
			_#checkoutCode & {
				with: "fetch-depth": 0
			},
      _#step & {
        name: "Bump version and push tag"
        id: "tag_version"
        uses: "mathieudutour/github-tag-action@v5.5"
        with: {
          "release_branches" : "release.*,hotfix.*,master"
          "github_token": "${{ secrets.GITHUB_TOKEN }}"
          "tag_prefix" : ""
        }
      },
      _#step & {
        name: "Create a GitHub release"
        uses: "actions/create-release@v1"
        env: "GITHUB_TOKEN": "${{ secrets.GITHUB_TOKEN }}"
        with: {
          "tag_name": "${{ steps.tag_version.outputs.new_tag }}"
          "release_name": "Release ${{ steps.tag_version.outputs.new_tag }}"
          body: "${{ steps.tag_version.outputs.changelog }}"
        }
      },
    ]
    }
  }
}

branch_build: _#bashWorkflow & {
  name: "Branch Build"
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
    "test": _#job & {
			name:      "Test"
			"runs-on": _#linuxMachine
			outputs: {
				version: "${{ steps.version.outputs.version }}"
				isDev: "${{ steps.version.outputs.isDev }}"
				tag: "${{ steps.version.outputs.tag }}"
			}
			steps: [
				_#checkoutCode & {
					with: "fetch-depth": 0
				},
				_#installGo,
				_#cacheGoModules,
				_#installKubeBuilder,
				_#step & {
					name: "Test"
					run: "make test"
				},
        _#step & {
          id: "version"
          name: "Get Version"
					run: """
					if [[ "$GITHUB_REF" == *"refs/head/release"* ||  "$GITHUB_REF" == *"refs/head/hotfix"* ]] ; then
						export VERSION=$(echo ${GITHUB_REF} | gsed -e 's/refs\\/head\/\\(release\\|hotfix\\)\\///g')
						echo "Found version from branch"
					else
						make svu
						export VERSION="$(./bin/svu next)"
					fi

					if [[ "$VERSION" == "" ]]; then
						echo "failed to find version"
						exit 1
					fi

					if [[ "$GITHUB_REF" == *"refs/heads/release"* ||  "$GITHUB_REF" == *"refs/heads/hotfix"* ]] ; then
					echo "using release version and githb_run_number"
					export TAG="${VERSION}-${GITHUB_RUN_NUMBER}"
					export IS_DEV="false"
					else
					echo "using beta in version"
					export TAG="${VERSION}-beta-${GITHUB_RUN_NUMBER}"
					export IS_DEV="true"
					fi

					echo "Found version $VERSION"
					echo "::set-output name=version::$VERSION"
					echo "Found tag $TAG"
					echo "::set-output name=tag::$TAG"
					echo "Found dev $IS_DEV"
					echo "::set-output name=isDev::$IS_DEV"
					"""
        }
			]
		}
    "images" : _#job & {
      name:      "Build Images"
			"runs-on": _#linuxMachine
			needs: ["test"]
      env: {
        VERSION: "${{ needs.test.outputs.tag }}"
      }
      strategy: matrix: {
        project: ["base", "operator", "authchecker", "metering", "reporter"]
      }
			steps: [
				_#checkoutCode,
				_#installGo,
        _#setupQemu,
        _#setupBuildX,
				_#cacheGoModules,
				_#installKubeBuilder,
        _#installOperatorSDK,
				_#installYQ,
				_#quayLogin,
				_#step & {
					id: "build"
					name: "Build images"
					run: """
					make clean-licenses save-licenses ${{ matrix.project }}/docker-build
					"""
				},
      ]
    }
		"deploy": _#job & {
			name:      "Deploy"
			"runs-on": _#linuxMachine
			needs: ["test"]
      env: {
        VERSION: "${{ needs.test.outputs.version }}"
        IMAGE_TAG: "${{ needs.test.outputs.tag }}"
        IS_DEV: "${{ needs.test.outputs.isDev }}"
      }
			outputs: {
				isDev:   "${{ steps.bundle.outputs.isDev }}"
				version: "${{ steps.bundle.outputs.version }}"
				tag:     "${{ steps.bundle.outputs.tag }}"
			}
			steps: [
				_#checkoutCode,
				_#installGo,
        _#setupQemu,
        _#setupBuildX,
				_#cacheGoModules,
				_#installKubeBuilder,
        _#installOperatorSDK,
				_#installYQ,
				_#quayLogin,
				_#step & {
					id:   "bundle"
					name: "Build bundle"
					run:  """
						REF=`echo ${GITHUB_REF} | sed 's/refs\\/head\\///g' | sed 's/\\//-/g'`
						echo "building $BRANCH with dev=$IS_DEV and version=$VERSION"

						cd v2
						export TAG=${VERSION}-${GITHUB_SHA}
						\((_#makeLogGroup & {#args: {name: "Make Stable Bundle", cmd: "make bundle-stable"}}).res)

						export VERSION=$IMAGE_TAG
						\((_#makeLogGroup & {#args: {name: "Make Bundle Build", cmd: "make bundle-build"}}).res)
						\((_#makeLogGroup & {#args: {name: "Make Dev Index", cmd: "make bundle-dev-index-multiarch"}}).res)

						echo "::set-output name=isDev::$IS_DEV"
						echo "::set-output name=version::$VERSION"
						echo "::set-output name=tag::$TAG"
						"""
				},
			]
		}
    publish: _#job & {
			name:      "Publish Images"
			"runs-on": _#linuxMachine
			needs: ["deploy", "images"]
			env: {
				VERSION: "${{ needs.deploy.outputs.version }}"
				TAG:     "${{ needs.deploy.outputs.tag }}"
			}
			if: "needs.deploy.outputs.isDev == 'false'"
			steps: [
				_#checkoutCode & {
					with: ref: "${{github.event.client_payload.sha}}"
				},
				_#installGo,
				_#cacheGoModules,
				_#installOperatorSDK,
				_#retagCommand,
				_#redhatConnectLogin,
				_#waitForPublish,
				_#retagManifestCommand,
			]
		}
	}
}
bundle: _#bashWorkflow & {
	name: "Deploy Bundle"
	on: repository_dispatch: types: ["bundle"]
	env: {
		"IMAGE_REGISTRY": "quay.io/rh-marketplace"
		"BRANCH":         "${{github.event.client_payload.branch}}"
		"DEPLOY_SHA":     "${{github.event.client_payload.sha}}"
		"IS_PR":          "${{github.event.client_payload.pull_request}}"
	}
	jobs: {
		deploy: _#job & {
			name:      "Deploy Bundle"
			"runs-on": _#linuxMachine
			outputs: {
				version: "${{ steps.bundle.outputs.version }}"
				tag:     "${{ steps.bundle.outputs.tag }}"
			}
			steps: [
				_#checkoutCode & {
					with: ref: "${{github.event.client_payload.sha}}"
				},
				_#installGo,
        _#setupQemu,
        _#setupBuildX,
				_#cacheGoModules,
				_#installOperatorSDK,
				_#installYQ,
				_#quayLogin,
				_#step & {
					id:   "bundle"
					name: "Build bundle"
					run:  """
						if [ "$IS_PR" == "false" ] && [ "$BRANCH" != "" ] ; then
						echo "is a dev request"
						export IS_DEV="true"
						else
						echo "is a release request"
						export IS_DEV="false"
						fi

						cd v2
						export VERSION=$(cd ./tools/version && go run ./main.go)
						export TAG=${VERSION}-${DEPLOY_SHA}

						\((_#makeLogGroup & {#args: {name: "Make Stable Bundle", cmd: "make bundle-stable"}}).res)

						if [ "$IS_DEV" == "true" ] ; then
						echo "using branch in version"
						export VERSION="${VERSION}-${BRANCH}-${GITHUB_RUN_NUMBER}"
						else
						echo "using release version and githb_run_number"
						export VERSION="${VERSION}-${GITHUB_RUN_NUMBER}"
						fi

						\((_#makeLogGroup & {#args: {name: "Make Bundle Build", cmd: "make bundle-build"}}).res)
						\((_#makeLogGroup & {#args: {name: "Make Deploy", cmd: "make bundle-deploy"}}).res)
						\((_#makeLogGroup & {#args: {name: "Make Dev Index", cmd: "make bundle-dev-index-multiarch"}}).res)

						echo "::set-output name=version::$VERSION"
						echo "::set-output name=tag::$VERSION"
						"""
				},
			]
		}
		publish: _#job & {
			name:      "Publish Images"
			"runs-on": _#linuxMachine
			needs: ["deploy"]
			env: {
				VERSION: "${{ needs.deploy.outputs.version }}"
				TAG:     "${{ needs.deploy.outputs.tag }}"
			}
			if: "github.event.client_payload.pull_request != 'false'"
			steps: [
				_#checkoutCode & {
					with: ref: "${{github.event.client_payload.sha}}"
				},
				_#installGo,
				_#cacheGoModules,
				_#installOperatorSDK,
				_#retagCommand,
				_#redhatConnectLogin,
				_#waitForPublish,
				_#retagManifestCommand,
			]
		}
	}
}

_#bashWorkflow: json.#Workflow & {
	jobs: [string]: defaults: run: shell: "bash"
}

// TODO: drop when cuelang.org/issue/390 is fixed.
// Declare definitions for sub-schemas
_#on:   ((json.#Workflow & {}).on & {x:   _}).x
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

_#makeLogGroup: {
	#args: {
		name: string
		cmd:  string
	}
	res:
		"""
		echo "::group::\(#args.name)"
		\(#args.cmd)
		echo "::endgroup::"
		"""
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

_#setupBuildX: _#step & {
	name: "Set up docker buildx"
	uses: "docker/setup-buildx-action@v1"
  id: "buildx"
}

_#setupQemu: _#step & {
  name: "Set up QEMU"
  uses: "docker/setup-qemu-action@v1"
}

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
		VERSION=$(make operator/current-version)
		RESULT=$(git tag --list | grep -E "$VERSION")
		IS_TAGGED=false
		if [ "$RESULT" != "" ] ; then
		  IS_TAGGED=true
		"""
}

_#branchRefPrefix: "refs/heads/"

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
		curl -LO ${OPERATOR_SDK_DL_URL}/checksums.txt
		curl -LO ${OPERATOR_SDK_DL_URL}/checksums.txt.asc
		grep operator-sdk_${OS}_${ARCH} checksums.txt | sha256sum -c -
		chmod +x operator-sdk_${OS}_${ARCH} && sudo mv operator-sdk_${OS}_${ARCH} /usr/local/bin/operator-sdk
		curl -LO https://github.com/operator-framework/operator-registry/releases/download/v1.15.3/${OS}-${ARCH}-opm
		chmod +x ${OS}-${ARCH}-opm && sudo mv ${OS}-${ARCH}-opm /usr/local/bin/opm
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

_#goVersion: "1.15.11"
_#pcUser:    "pcUser"

_#operator: {
	name:  "redhat-marketplace-operator"
	ospid: "ospid-c93f69b6-cb04-437b-89d6-e5220ce643cd"
	pword: "pcPassword"
}

_#metering: {
	name:  "redhat-marketplace-metric-state"
	ospid: "ospid-9b9b0dbe-7adc-448e-9385-a556714a09c4"
	pword: "pcPasswordMetricState"
}

_#reporter: {
	name:  "redhat-marketplace-reporter"
	ospid: "ospid-faa0f295-e195-4bcc-a3fc-a4b97ada317e"
	pword: "pcPasswordReporter"
}

_#authchecker: {
	name:  "redhat-marketplace-authcheck"
	ospid: "ospid-ffed416e-c18d-4b88-8660-f586a4792785"
	pword: "pcPasswordAuthCheck"
}

_#images: [
	_#operator,
	_#metering,
	_#reporter,
	_#authchecker,
]

_#registry:       "quay.io/rh-marketplace"
_#registryRHScan: "scan.connect.redhat.com"

_#manifest: {
	name:  "redhat-marketplace-operator-manifest"
	ospid: "ospid-64f06656-d9d4-43ef-a227-3b9c198800a1"
	pword: "pcPasswordOperatorManifest"
}

_#repoFromTo: [ for k, v in _#images {
	pword: "\(v.pword)"
	from:  "\(_#registry)/\(v.name):$TAG"
	to:    "\(_#registryRHScan)/\(v.ospid)/\(v.name):$TAG"
}]

_#manifestFromTo: [ for k, v in [_#manifest] {
	pword: "\(v.pword)"
	from:  "\(_#registry)/\(v.name):$VERSION"
	to:    "\(_#registryRHScan)/\(v.ospid)/\(v.name):$VERSION"
}]

_#copyImage: {
	#args: {
		to:    string
		from:  string
		pword: string
	}
	res: """
				echo "::group::Push \(#args.to)"
				skopeo inspect docker://\(#args.to) --creds ${{secrets['\(_#pcUser)']}}:${{secrets['\(#args.pword)']}} > /dev/null
				([[ $? == 0 ]] && echo "exists=true" || skopeo copy --all docker://\(#args.from) docker://\(#args.to) --dest-creds ${{secrets['\(_#pcUser)']}}:${{secrets['\(#args.pword)']}})
				echo "::endgroup::"
				"""
}

_#retagCommandList: [ for k, v in _#repoFromTo {(_#copyImage & {#args: v}).res}]

_#retagCommand: _#step & {
	id:    "mirror"
	name:  "Mirror images"
	shell: "bash {0}"
	run:   strings.Join(_#retagCommandList, "\n")
}

_#manifestCopyCommandList: [ for k, v in _#manifestFromTo {(_#copyImage & {#args: v}).res}]

_#retagManifestCommand: _#step & {
	env: TAG: "${{ steps.deploy.outputs.version }}"
	name:  "Copy Manifest"
	shell: "bash {0}"
	run:   strings.Join(_#manifestCopyCommandList, "\n")
}

_#registryLoginStep: {
	#args: {
		user:     string
		pass:     string
		registry: string
	}
	res: _#step & {
		name: "Login to Docker Hub"
		uses: "docker/login-action@v1"
		with: {
			registry: "\(#args.registry)"
			username: "\(#args.user)"
			password: "\(#args.pass)"
		}
	}
}

_#quayLogin: (_#registryLoginStep & {
	#args: {
		registry: "quay.io/rh-marketplace"
		user:     "${{secrets['quayUser']}}"
		pass:     "${{secrets['quayPassword']}}"
	}
}).res

_#redhatConnectLogin: (_#registryLoginStep & {
	#args: {
		registry: "registry.connect.redhat.com"
		user:     "${{secrets['REDHAT_IO_USER']}}"
		pass:     "${{secrets['REDHAT_IO_PASSWORD']}}"
	}
}).res

_#waitForPublish: _#step & {
	name: "Wait for RH publish"
	env: {
		RH_CONNECT_TOKEN: "${{ secrets.redhat_api_key }}"
	}
	"continue-on-error": true
	run:
		"""
			cd v2
			make wait-and-publish PIDS="\(strings.Join([ for k, v in _#images {"--pid \(v.ospid)=$(skopeo inspect --override-os=linux --format \"{{.Digest}}\" docker://\(_#registry)/\(v.name):$TAG)"}], " "))"
			"""
}

#preset: _#job & {
	name:      "Preset"
	"runs-on": _#linuxMachine
	steps:     [
			_#turnStyleStep,
			_#checkoutCode,
			_#installGo,
			_#cacheGoModules] +
		_#setBranchOutput + [
			_#step & {
				name: "Get Vars"
				id:   "vars"
				run: """
					echo "::set-output name=version::$(make operator/current-version)"
					echo "::set-output name=tag::sha-$(git rev-parse --short HEAD)"
					echo "::set-output name=hash::$(make operator/current-version)-$(git rev-parse --short HEAD)"
					echo "::set-output name=dockertag::${TAGPREFIX}$(make operator/current-version)-${GITHUB_SHA::8}"
					echo "::set-output name=quayExpiration::${QUAY_EXPIRATION:-never}"
					"""
			},
		]
	outputs: {
		version:        "${{ steps.vars.outputs.version }}"
		tag:            "${{ steps.vars.outputs.tag }}"
		hash:           "${{ steps.vars.outputs.hash }}"
		dockertag:      "${{ steps.vars.outputs.dockertag }}"
		quayExpiration: "${{ steps.vars.outputs.quayExpiration }}"
	}
}

_#checkRunObject: {
	name:          string
	head_sha:      string
	check_run_id?: string
	status?:       "queued" | "in_progress" | "completed" | string
	conclusion?:   "action_required" | "cancelled" | "failure" | "neutral" | "success" | "skipped" | "stale" | "timed_out" | string
	output?: {
		title:   string
		summary: string
		text?:   string
	}
}

_#checkRunPatch: {
	status?:     "queued" | "in_progress" | "completed" | string
	conclusion?: "action_required" | "cancelled" | "failure" | "neutral" | "success" | "skipped" | "stale" | "timed_out" | string
	output?: {
		title:   string
		summary: string
		text?:   string
	}
}

_#setOutput: {
	#args: {
		name:  string
		value: string
	}
	res: #"echo "::set-output name=\#(#args.name):\#(#args.value)"#
}

_#setEnv: {
	#args: {
		name:  string
		value: string
	}
	res: #"echo \#(#args.name)=\#(#args.value) >> $GITHUB_ENV"#
}

_#findCheckRun: {
	_#args: {
		name:     string
		head_sha: string
	}
	res: _#step & {
		name: "Find checkRun with name \(_#args.name)"
		run:  """
			RESULT=$(curl \\
			-X POST \\
			-H "Authorization: Bearer ${{secrets.GITHUB_TOKEN}}" \\
			-H "Accept: application/vnd.github.v3+json" \\
			https://api.github.com/repos/$GITHUB_REPOSITORY/refs/\(_#args.head_sha)/check-runs)
			ID=$(echo $RESULT | jq '.check_runs[]? | select(.name == "\(_#args.name)") | .id')
			CHECKSUITE_ID=$(echo $RESULT | jq '.check_runs[]? | select(.name == "\(_#args.name)") | .check_suite.id')
			\((_#setEnv & {#args: {name: "checkrun_id", value: "$ID"}}).res)
			\((_#setEnv & {#args: {name: "checksuite_id", value: "$CHECKSUITE_ID"}}).res)
			"""
	}
}

_#githubCreateActionStep: {
	_#args: _#checkRunObject
	res:    _#step & {
		name: "Create checkRun with name \(_#args.name)"
		run:
			"""
			RESULT=$(curl -X POST \\
			-H "Authorization: Bearer ${{secrets.GITHUB_TOKEN}}" \\
			-H "Accept: application/vnd.github.v3+json" \\
			https://api.github.com/repos/$GITHUB_REPOSITORY/check-runs \\
			-d \(strconv.Quote(encjson.Marshal(_#args))) )
			ID=$(echo $RESULT | jq '.id')
			CHECKSUITE_ID=$(echo $RESULT | jq '.check_suite.id')
			\((_#setEnv & {#args: {name: "checkrun_id", value: "$ID"}}).res)
			\((_#setEnv & {#args: {name: "checksuite_id", value: "$CHECKSUITE_ID"}}).res)
			"""
	}
}

_#githubUpdateActionStep: {
	_#args: {
		check_run_id: string
		patch:        _#checkRunPatch
	}
	res: _#step & {
		name: "Update checkRun with id\(_#args.check_run_id)"
		run:
			"""
			curl -X PATCH \\
				-H "Authorization: Bearer ${{secrets.GITHUB_TOKEN}}" \\
				-H "Accept: application/vnd.github.v3+json" \\
				https://api.github.com/repos/$GITHUB_REPOSITORY/check-runs/\(_#args.check_run_id) \\
				-d \(strconv.Quote(encjson.Marshal(_#args.patch)))
			"""
	}
}

_#installYQ: _#step & {
	name: "Install YQ"
	run:  "sudo snap install yq"
}
