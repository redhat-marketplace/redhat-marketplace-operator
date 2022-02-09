package ci

import (
	"list"
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
	{
		file:   "publish.yml"
		schema: publish
	},
	{
		file:   "release_status.yml"
		schema: release_status
	},
	{
		file:   "build_base.yml"
		schema: build_base_images
	},
	// {
	//  file:   "sync_branches.yml"
	//  schema: sync_branches
	// },
]
varPresetGitTag:         "${{ needs.preset.outputs.tag }}"
varPresetVersion:        "${{ needs.preset.outputs.version }}"
varPresetHash:           "${{ needs.preset.outputs.hash }}"
varPresetDockertag:      "${{ needs.preset.outputs.dockertag }}"
varPresetQuayExpiration: "${{ needs.preset.outputs.quayExpiration }}"

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
					id:   "tag_version"
					uses: "mathieudutour/github-tag-action@v5.5"
					with: {
						"release_branches": "release.*,hotfix.*,master"
						"github_token":     "${{ secrets.GITHUB_TOKEN }}"
						"tag_prefix":       ""
					}
				},
				_#step & {
					name: "Create a GitHub release"
					uses: "actions/create-release@v1"
					env: "GITHUB_TOKEN": "${{ secrets.GITHUB_TOKEN }}"
					with: {
						"tag_name":     "${{ steps.tag_version.outputs.new_tag }}"
						"release_name": "Release ${{ steps.tag_version.outputs.new_tag }}"
						body:           "${{ steps.tag_version.outputs.changelog }}"
					}
				},
			]
		}
	}
}

release_status: _#bashWorkflow & {
	name: "Publish Status"
	on: schedule: [{
		cron: "*/15 * * * *"
	}]
	jobs: {
		prs: _#job & {
			name:      "Get release PRs"
			"runs-on": _#linuxMachine
			steps: [
				_#findAllReleasePRs,
				_#step & {
					id: "set-matrix"
					run: """
						OUTPUT='${{ steps.findAllReleasePRs.outputs.data }}'
						EMPTY=$(echo $OUTPUT | jq -cr '.search.edges | length == 0')
						OUTPUT=$(echo $OUTPUT | jq -cr '[.search.edges[].node]' 2> /dev/null) || echo '[]'
						echo "::set-output name=emptymatrix::$EMPTY"
						echo "::set-output name=matrix::{\\"include\\":$OUTPUT}"
						"""
				},
			]
			outputs: {
				matrix:      "${{ steps.set-matrix.outputs.matrix}}"
				emptymatrix: "${{ steps.set-matrix.outputs.emptymatrix}}"
			}
		}
		status: _#job & {
			name:      "Check publish status"
			"runs-on": _#linuxMachine
			needs: ["prs"]
			if: "${{ needs.prs.outputs.emptymatrix == 'false' }}"
			strategy: matrix: "${{fromJson(needs.prs.outputs.matrix)}}"
			steps: [
				(_#findPRForComment & {
					_#args: {
						prNum: "${{ matrix.number }}"
					}
				}).res,
				_#checkoutCode & {
					with: {
						"fetch-depth": 0
						"ref":         "${{ steps.pr.outputs.prSha }}"
					}
				},
				_#installGo,
				_#cacheGoModules,
				_#installKubeBuilder,
				_#getVersion & {
					env: {
						"REF": "${{ steps.pr.outputs.prRef }}"
					}
				},
				_#getBundleRunID,
				_#checkoutCode,
				_#operatorImageStatuses,
				_#step & {
					id:   "pretty"
					name: "Format output"
					run:  ".github/workflows/scripts/process_pc_status.sh '${{ steps.operatorImageStatuses.outputs.imageStatus }}'"
				},
				_#step & {
					uses: "marocchino/sticky-pull-request-comment@v2"
					name: "Post comment"
					with: {
						header:   "imagestatus"
						recreate: "true"
						number:   "${{ matrix.number }}"
						message: """
## RH PC Status for tag: ${{ env.TAG }}

${{env.MD_TABLE}}
"""
					}
				},
			]
		}
	}
}

publish: _#bashWorkflow & {
	name: "Publish"
	on: ["issue_comment"]
	env: {
		"DOCKER_CLI_EXPERIMENTAL": "enabled"
	}
	jobs: {
		push: _#job & {
			name:      "Push Images to PC"
			if:        "${{ github.event.issue.pull_request && startsWith(github.event.comment.body, '/push-images') }}"
			"runs-on": _#linuxMachine
			steps: [
				_#hasWriteAccess,
				(_#findPRForComment & {
					_#args: {
						prNum: "${{ github.event.issue.number }}"
					}
				}).res,
				_#checkoutCode & {
					with: {
						"fetch-depth": 0
						"ref":         "${{ steps.pr.outputs.prSha }}"
					}
				},
				_#installGo,
				_#cacheGoModules,
				_#installKubeBuilder,
				_#getVersion & {
					env: {
						"REF": "${{ steps.pr.outputs.prRef }}"
					}
				},
				_#getBundleRunID,
				_#checkoutCode,
				_#scanCommand.res,
				_#addRocketToComment,
				_#addFailureComment,
			]
		}
		publish: _#job & {
			name:      "Publish Images"
			if:        "${{ github.event.issue.pull_request && startsWith(github.event.comment.body, '/publish-images')}}"
			"runs-on": _#linuxMachine
			steps: [
				_#hasWriteAccess,
				(_#findPRForComment & {
					_#args: {
						prNum: "${{ github.event.issue.number }}"
					}
				}).res,
				_#checkoutCode & {
					with: {
						"fetch-depth": 0
						"ref":         "${{ steps.pr.outputs.prSha }}"
					}
				},
				_#installGo,
				_#cacheGoModules,
				_#installKubeBuilder,
				_#getVersion & {
					env: {
						"REF": "${{ steps.pr.outputs.prRef }}"
					}
				},
				_#getBundleRunID,
				_#redhatConnectLogin,
				_#checkoutCode,
				_#publishOperatorImages,
				_#addRocketToComment,
				_#addFailureComment,
			]
		}
		"push-operator": _#job & {
			name:      "Push Operator"
			if:        "${{ github.event.issue.pull_request && startsWith(github.event.comment.body, '/push-operator')}}"
			"runs-on": _#linuxMachine
			steps: [
				_#hasWriteAccess,
				(_#findPRForComment & {
					_#args: {
						prNum: "${{ github.event.issue.number }}"
					}
				}).res,
				_#checkoutCode & {
					with: {
						"fetch-depth": 0
						"ref":         "${{ steps.pr.outputs.prSha }}"
					}
				},
				_#installGo,
				_#cacheGoModules,
				_#installKubeBuilder,
				_#getVersion & {
					env: {
						"REF": "${{ steps.pr.outputs.prRef }}"
					}
				},
				_#getBundleRunID,
				_#redhatConnectLogin,
				_#checkoutCode,
				_#retagManifestCommand,
				_#addRocketToComment,
				_#addFailureComment,
			]
		}
		"publish-operator": _#job & {
			name:      "Publish Operator"
			if:        "${{ github.event.issue.pull_request && startsWith(github.event.comment.body, '/operator') }}"
			"runs-on": _#linuxMachine
			steps: [
				_#hasWriteAccess,
				(_#findPRForComment & {
					_#args: {
						prNum: "${{ github.event.issue.number }}"
					}
				}).res,
				_#checkoutCode & {
					with: {
						"fetch-depth": 0
						"ref":         "${{ steps.pr.outputs.prSha }}"
					}
				},
				_#installGo,
				_#cacheGoModules,
				_#installKubeBuilder,
				_#getVersion & {
					env: {
						"REF": "${{ steps.pr.outputs.prRef }}"
					}
				},
				_#getBundleRunID,
				_#redhatConnectLogin,
				_#checkoutCode,
				_#publishOperator,
				_#addRocketToComment,
				_#addFailureComment,
			]
		}
	}
}

build_base_images: _#bashWorkflow & {
	name: "Build Bases"
	on: {
		workflow_dispatch: {}
		schedule: [{cron: "0 0 * * *"}] //nightly
	}
	jobs: {
		"base": _#job & {
			name:      "Build Base"
			"runs-on": _#linuxMachine
			env: {
				GO_VERSION: _#goVersion
			}
			strategy: matrix: {
				command: ["base", "data-service"]
			}
			env: {
				"IMAGE_REGISTRY": "quay.io/rh-marketplace"
			}
			steps: [
				_#checkoutCode,
				_#installGo,
				_#setupQemu,
				_#setupBuildX,
				_#quayLogin,
				_#step & {
					id:   "build"
					name: "Build images"
					env: {
						"DOCKERBUILDXCACHE": "/tmp/.buildx-cache"
						"PUSH":              "false"
					}
					run: """
						make base/${{ matrix.command }}
						"""
				},
				_#step & {
					id:   "push"
					name: "Push images"
					env: {
						"DOCKERBUILDXCACHE": "/tmp/.buildx-cache"
						"IMAGE_PUSH":        "true"
					}
					run: """
						make base/${{ matrix.command }}
						"""
				},
			]
		}

	}
}

// _#futureReleases: [{
//   base: "develop",
//   next: "release/2.5.0"
// },
// {
//   base: "release/2.5.0"
//   next: "release/2.6.0"
// },
//                   ]

// // This workflow syncs the next release with develop and future
// sync_branches: _#bashWorkflow & {
//  name: "Sync Next Release"
//  on: {
//   push: {
//    branches: [ "develop", _#nextRelease]
//   }
//  }
//  jobs: {

//   sync: {
//    name:      "Sync next release"
//    "runs-on": _#linuxMachine
//    if:        "${{ github.ref == 'refs/heads/\(_#nextRelease)' || github.ref == 'refs/heads/develop' }}"
//    steps: [
//     _#checkoutCode,
//     _#step & {
//      name: "pull-request-action"
//      if:   "${{ github.ref == 'refs/heads/develop' }}"
//      uses: "vsoch/pull-request-action@master"
//      env: {
//       "GITHUB_TOKEN":        "${{ secrets.GITHUB_TOKEN }}"
//       "PULL_REQUEST_BRANCH": _#nextRelease
//       "PULL_REQUEST_TITLE":  "chore: develop to \(_#nextRelease)"
//       "PULL_REQUEST_UPDATE": "true"
//      }
//     },
//     _#step & {
//      name: "pull-request-action"
//      if:   "${{ github.ref == 'refs/heads/\(_#nextRelease)' }}"
//      uses: "vsoch/pull-request-action@master"
//      env: {
//       "GITHUB_TOKEN":        "${{ secrets.GITHUB_TOKEN }}"
//       "PULL_REQUEST_BRANCH": "master"
//       "PULL_REQUEST_TITLE":  "Release ${{ github.ref }}"
//       "PULL_REQUEST_UPDATE": "true"
//      }
//     },

//    ]
//   }
//  }
// }

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
			steps: [
				_#checkoutCode & {
					with: "fetch-depth": 0
				},
				_#cancelPreviousRun,
				_#installGo,
				_#cacheGoModules,
				_#installKubeBuilder,
				_#step & {
					name: "Test"
					run:  "make operator/test"
				},
			]
		}
		"matrix-test": _#job & {
			name:      "Matrix Test"
			"runs-on": _#linuxMachine
			strategy: matrix: {
				project: ["authchecker", "metering", "reporter", "tests"]
			}
			steps: [
				_#checkoutCode,
				_#installGo,
				_#cacheGoModules,
				_#installKubeBuilder,
				_#step & {
					name: "Test"
					run:  "make ${{ matrix.project }}/test"
				},
			]
		}
		"images": _#job & {
			name:      "Build Images"
			"runs-on": _#linuxMachine
			env: {
				GO_VERSION: _#goVersion
			}
			strategy: matrix: {
				project: ["operator", "authchecker", "metering", "reporter", "airgap", "watcher"]
				include: [
					{
						project: "operator"
					},
					{
						project: "authchecker"
					},
					{
						project: "metering"
					},
					{
						project: "reporter"
					},
					{
						project: "airgap"
					},
					{
						project: "watcher"
					},
				]
			}
			steps: [
				_#checkoutCode & {
					with: {
						"fetch-depth": 0
					}
				},
				_#installGo,
				_#setupQemu,
				_#setupBuildX,
				_#cacheGoModules,
				_#installKubeBuilder,
				_#quayLogin,
				_#getVersion,
				_#step & {
					id:   "build"
					name: "Build and push images"
					env: {
						"DOCKERBUILDXCACHE": "/tmp/.buildx-cache"
						"IMAGE_PUSH":        "true"
					}
					run: """
						if [ "$IS_DEV" == "true" ]; then
							export ARCHS="amd64"
						fi

						make clean-licenses save-licenses ${{ matrix.project }}/docker-build
						"""
				},
			]
		}
		"deploy": _#job & {
			name:      "Deploy"
			"runs-on": _#linuxMachine
			needs: ["test", "matrix-test", "images"]
			outputs: {
				isDev:   "${{ steps.bundle.outputs.isDev }}"
				version: "${{ steps.bundle.outputs.version }}"
				tag:     "${{ steps.bundle.outputs.tag }}"
			}
			steps: [
				_#checkoutCode & {
					with: {
						"fetch-depth": 0
					}
				},
				_#installGo,
				_#setupQemu,
				_#setupBuildX,
				_#cacheGoModules,
				_#installKubeBuilder,
				_#quayLogin,
				_#redhatConnectLogin,
        _#redhatDockerLogin,
				_#getVersion,
				_#step & {
					id:   "bundle"
					name: "Build bundle"
					env: "IMAGE_TAG": "${{ steps.version.outputs.tag }}"
					run: """
						REF=`echo ${GITHUB_REF} | sed 's/refs\\/head\\///g' | sed 's/\\//-/g'`
						echo "building $BRANCH with dev=$IS_DEV and version=$VERSION"

						cd v2
						export TAG=$IMAGE_TAG
						\((_#makeLogGroup & {#args: {name: "Make Stable Bundle", cmd: "make bundle-stable"}}).res)

						export VERSION=$IMAGE_TAG
						\((_#makeLogGroup & {#args: {name: "Make Bundle Build", cmd: "make bundle-build"}}).res)
						\((_#makeLogGroup & {#args: {name: "Make Dev Index", cmd: "make bundle-dev-index-multiarch"}}).res)

						echo "::set-output name=isDev::$IS_DEV"
						echo "::set-output name=version::$VERSION"
						echo "::set-output name=tag::$TAG"
						"""
				},
				_#step & {
					uses: "actions/upload-artifact@v2"
					with: {
						name: "release-bundle-${{ steps.bundle.outputs.tag }}"
						path: "v2/bundle"
					}
				},
				_#step & {
					uses: "marocchino/sticky-pull-request-comment@v2"
					with: {
						header:   "devindex"
						recreate: "true"
						message: """
              Available to test on openshift using a catalogsource:

              ``` yaml
              apiVersion: operators.coreos.com/v1alpha1
              kind: CatalogSource
              metadata:
                name: rhm-test
                namespace: openshift-marketplace
              spec:
                displayName: RHM Test
                image: quay.io/rh-marketplace/redhat-marketplace-operator-dev-index:${{ env.TAG }}
                publisher: ''
                sourceType: grpc
              ```
              """
					}
				},
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
	id:   "buildx"
	with: {
		"config": ".github/buildkitd.toml"
	}
}

_#hasWriteAccess: _#step & {
	name: "Check if user has write access"
	uses: "lannonbr/repo-permission-check-action@2.0.0"
	with: permission:  "write"
	env: GITHUB_TOKEN: "${{ secrets.GITHUB_TOKEN }}"
}

_#setupQemu: _#step & {
	name: "Set up QEMU"
	uses: "docker/setup-qemu-action@v1"
	with: {
		image:     "tonistiigi/binfmt:qemu-v6.1.0"
		platforms: "all"
	}
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
		key:            "${{ runner.os }}-go-${{ github.sha }}"
		"restore-keys": "${{ runner.os }}-go-"
	}
}

_#cacheDockerBuildx: {
	#project: string
	res:      _#step & {
		name: "Cache Docker Buildx"
		uses: "actions/cache@v2"
		with: {
			path:           "/tmp/.buildx-cache"
			key:            "${{ runner.os }}-buildx-\(#project)-${{ github.ref }}-${{ github.sha }}"
			"restore-keys": "${{ runner.os }}-buildx-\(#project)-"
		}
	}
}

_#getBundleRunID: _#step & {
	name: "Get Latest Bundle Run"
	run: """
		WORKFLOW_ID=8480641
		BRANCH_BUILD=$(curl \\
		  -H "Accept: application/vnd.github.v3+json" \\
		  "${GITHUB_API_URL}/repos/${GITHUB_REPOSITORY}/actions/workflows/$WORKFLOW_ID/runs?branch=$REF&event=push" \\
		   | jq '.workflow_runs | max_by(.run_number)')

		if [ "$BRANCH_BUILD" == "" ]; then
		  echo "failed to get branch build"
		  exit 1
		fi

		status=$(echo $BRANCH_BUILD | jq -r ' pffastatus')
		conclusion=$(echo $BRANCH_BUILD | jq -r '.conclusion')

		if [ "$status" != "completed" ] && [ "$conclusion" != "success" ]; then
		  echo "$status and $conclusion were not completed and successful"
		  exit 1
		fi

		RUN_NUMBER=$(echo $BRANCH_BUILD | jq -r '.run_number')

		export TAG="${VERSION}-${RUN_NUMBER}"
		echo "setting tag to $TAG"
		echo "TAG=$TAG" >> $GITHUB_ENV
		"""
}

_#getVersion: _#step & {
	id:   "version"
	name: "Get Version"
	run: """
		make svu
		export VERSION="$(./bin/svu next --prefix '')"

		if [ "$REF" == "" ]; then
			REF="$GITHUB_REF"
		fi

		if [[ "$GITHUB_HEAD_REF" != "" ]]; then
			echo "Request is a PR $GITHUB_HEAD_REF is head; is base $GITHUB_BASE_REF is base"
			REF="$GITHUB_HEAD_REF"
		fi

		echo "Found ref $REF"

		if [[ "$VERSION" == "" ]]; then
			echo "failed to find version"
			exit 1
		fi

		if [[ "$REF" == *"release"* ||  "$REF" == *"hotfix"* || "$REF" == *"refs/head/master"* || "$REF" == *"refs/head/develop"* ]] ; then
		echo "using release version and github_run_number"
		export VERSION=${VERSION}-${GITHUB_RUN_NUMBER}
		export TAG="${VERSION}"
		export IS_DEV="false"
		else
		echo "using beta in version"
		export VERSION=${VERSION}-beta.${GITHUB_RUN_NUMBER}
		export TAG="${VERSION}"
		export IS_DEV="true"
		fi

		echo "Found version $VERSION"
		echo "::set-output name=version::$VERSION"
		echo "VERSION=$VERSION" >> $GITHUB_ENV
		echo "Found tag $TAG"
		echo "TAG=$TAG" >> $GITHUB_ENV
		echo "::set-output name=tag::$TAG"
		echo "IS_DEV=$IS_DEV" >> $GITHUB_ENV
		echo "::set-output name=isDev::$IS_DEV"
		echo "REF=$REF" >> $GITHUB_ENV
		"""
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
	run:  """
		os=$(go env GOOS)
		arch=$(go env GOARCH)
		version=\(_#kubeBuilderVersion)

		# download kubebuilder and extract it to tmp
		# https://github.com/kubernetes-sigs/kubebuilder/releases/download/v3.1.0/kubebuilder_darwin_amd64
		# https://github.com/kubernetes-sigs/kubebuilder/releases/download/v2.3.1/kubebuilder_2.3.1_darwin_amd64.tar.gz
		# 3.0 + versions will not be tar.gz
		curl -L https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${version}/kubebuilder_${version}_${os}_${arch}.tar.gz | tar -xz -C /tmp/

		# move to a long-term location and put it on your path
		# (you'll need to set the KUBEBUILDER_ASSETS env var if you put it somewhere else)
		sudo mv /tmp/kubebuilder_${version}_${os}_${arch} /usr/local/kubebuilder
		echo "/usr/local/kubebuilder/bin" >> $GITHUB_PATH
		"""
}

_#installOperatorSDK: _#step & {
	name: "Install operatorsdk"
	run: """
		version=v1.7.2
		export ARCH=$(case $(arch) in x86_64) echo -n amd64 ;; aarch64) echo -n arm64 ;; *) echo -n $(arch) ;; esac)
		export OS=$(uname | awk '{print tolower($0)}')
		export OPERATOR_SDK_DL_URL=https://github.com/operator-framework/operator-sdk/releases/download/${version}
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

_#archs: ["amd64", "ppc64le", "s390x", "arm64"]
_#registry:           "quay.io/rh-marketplace"
_#goVersion:          "1.16.8"
_#branchTarget:       "/^(master|develop|release.*|hotfix.*)$/"
_#pcUser:             "pcUser"
_#kubeBuilderVersion: "2.3.2"

_#image: {
	name:  string
	ospid: string
	pword: string
	url:   string
}

_#projectURLs: {
	operator:   "https://connect.redhat.com/projects/5e98b6fac77ce6fca8ac859c/images"
	reporter:   "https://connect.redhat.com/projects/5e98b6fc32116b90fd024d06/images"
	metering:   "https://connect.redhat.com/projects/5f36ea2f74cc50b8f01a838d/images"
	authcheck:  "https://connect.redhat.com/projects/5f62b71018e80cdc21edf22f/images"
	datasevice: "https://connect.redhat.com/projects/61649f78d3e2f8d3bcfe30d5/images"
	bundle:     "https://connect.redhat.com/projects/5f68c9457115dbd1183ccab6/images"
}

_#operator: _#image & {
	name:  "redhat-marketplace-operator"
	ospid: "ospid-c93f69b6-cb04-437b-89d6-e5220ce643cd"
	pword: "pcPassword"
	url:   _#projectURLs["operator"]
}

_#metering: _#image & {
	name:  "redhat-marketplace-metric-state"
	ospid: "ospid-9b9b0dbe-7adc-448e-9385-a556714a09c4"
	pword: "pcPasswordMetricState"
	url:   _#projectURLs["metering"]
}

_#reporter: _#image & {
	name:  "redhat-marketplace-reporter"
	ospid: "ospid-faa0f295-e195-4bcc-a3fc-a4b97ada317e"
	pword: "pcPasswordReporter"
	url:   _#projectURLs["reporter"]
}

_#authchecker: _#image & {
	name:  "redhat-marketplace-authcheck"
	ospid: "ospid-ffed416e-c18d-4b88-8660-f586a4792785"
	pword: "pcPasswordAuthCheck"
	url:   _#projectURLs["authcheck"]
}

_#dataservice: _#image & {
	name:  "redhat-marketplace-data-service"
	ospid: "ospid-61649f78d3e2f8d3bcfe30d5"
	pword: "pcPasswordAuthCheck"
	url:   _#projectURLs["datasevice"]
}

_#manifest: _#image & {
	name:  "redhat-marketplace-operator-manifest"
	ospid: "ospid-64f06656-d9d4-43ef-a227-3b9c198800a1"
	pword: "pcPasswordOperatorManifest"
	url:   _#projectURLs["bundle"]
}

_#images: [
	_#operator,
	_#metering,
	_#reporter,
	_#authchecker,
	_#dataservice,
]

_#registry:       "quay.io/rh-marketplace"
_#registryRHScan: "scan.connect.redhat.com"

_#copyImageArch: {
	#args: {
		to:    string
		from:  string
		pword: string
		arch:  string
	}
	res: """
echo "::group::Push \(#args.to)-\(#args.arch)"
skopeo --override-arch=\(#args.arch) --override-os=linux inspect docker://\(#args.to) --creds ${{secrets['\(_#pcUser)']}}:${{secrets['\(#args.pword)']}} > /dev/null
([[ $? == 0 ]] && echo "exists=true" || skopeo --override-arch=\(#args.arch) --override-os=linux copy docker://\(#args.from) docker://\(#args.to)-\(#args.arch) --dest-creds ${{secrets['\(_#pcUser)']}}:${{secrets['\(#args.pword)']}})
echo "::endgroup::"
"""
}

_#copyImage: {
	#args: {
		to:    string
		from:  string
		pword: string
	}
	res: """
echo "::group::Push \(#args.to)"
skopeo --override-os=linux inspect docker://\(#args.to) --creds ${{secrets['\(_#pcUser)']}}:${{secrets['\(#args.pword)']}} > /dev/null
([[ $? == 0 ]] && echo "exists=true" || skopeo copy docker://\(#args.from) docker://\(#args.to) --dest-creds ${{secrets['\(_#pcUser)']}}:${{secrets['\(#args.pword)']}})
echo "::endgroup::"
"""
}

_#scanImageWithArch: {
	#args: {
		ospid: string
		from:  string
		tag:   string
		arch:  string
	}
	res: """
echo "::group::Scan \(#args.from)"
id=$(curl -X GET "https://catalog.redhat.com/api/containers/v1/projects/certification/pid/\(#args.ospid)" -H  "accept: application/json" -H  "X-API-KEY: $REDHAT_TOKEN" | jq -r '._id')
digest=$(skopeo --override-os=linux inspect docker://\(#args.from):\(#args.tag) --raw | jq -r '.manifests[] | select(.platform.architecture == $arch) | .digest' --arg arch \(#args.arch))
curl -X POST "https://catalog.redhat.com/api/containers/v1/projects/certification/id/$id/requests/scans" \\
--header 'Content-Type: application/json' \\
--header "X-API-KEY: $REDHAT_TOKEN" \\
--data-raw "{\\"pull_spec\\": \\"\(#args.from)@$digest\\",\\"tag\\": \\"\(#args.tag)-\(#args.arch)\\"}"
echo "::endgroup::"
"""
}

_#scanImage: {
	#args: {
		ospid: string
		from:  string
		tag:   string
	}
	res: """
echo "::group::Scan \(#args.from)"
id=$(curl -X GET "https://catalog.redhat.com/api/containers/v1/projects/certification/pid/\(#args.ospid)" -H  "accept: application/json" -H  "X-API-KEY: $REDHAT_TOKEN" | jq -r '._id')
digest=$(skopeo --override-os=linux inspect docker://\(#args.from):\(#args.tag) | jq -r '.Digest')
curl -X POST "https://catalog.redhat.com/api/containers/v1/projects/certification/id/$id/requests/scans" \\
--header 'Content-Type: application/json' \\
--header "X-API-KEY: $REDHAT_TOKEN" \\
--data-raw "{\\"pull_spec\\": \\"\(#args.from)@$digest\\",\\"tag\\": \\"\(#args.tag)\\"}"
echo "::endgroup::"
"""
}

_#scanCommand: {
	#args: {
		fromTo: [ for k, v in _#images {
			ospid: "\(v.ospid)"
			from:  "\(_#registry)/\(v.name)"
			tag:   "$TAG"
		}]
		scanCommandList: [ for k, v in #args.fromTo {(_#scanImage & {#args: v}).res}]
	}
	res: _#step & {
		#shell: strings.Join(list.FlattenN(#args.scanCommandList, -1), "\n")
		id:     "mirror"
		name:   "Scan images"
		env: {
			"REDHAT_TOKEN": "${{ secrets.redhat_api_key }}"
		}
		run: ".github/workflows/scripts/scan_images.sh"
	}
}

_#retagCommand: {
	#args: {
		fromTo: [ for k, v in _#images {
			pword: "\(v.pword)"
			from:  "\(_#registry)/\(v.name):$TAG"
			to:    "\(_#registryRHScan)/\(v.ospid)/\(v.name):$TAG"
		}]
		retagCommandList: [ for #arch in _#archs {[ for k, v in #args.fromTo {(_#copyImageArch & {#args: v & {arch: #arch}}).res}]}]
	}
	res: _#step & {
		id:    "mirror"
		name:  "Mirror images"
		shell: "bash {0}"
		run:   strings.Join(list.FlattenN(#args.retagCommandList, -1), "\n")
	}
}

_#retagManifestCommand: _#step & {
	#args: {
		fromTo: [ for k, v in [_#manifest] {
			pword: "\(v.pword)"
			from:  "\(_#registry)/\(v.name):$TAG"
			to:    "\(_#registryRHScan)/\(v.ospid)/\(v.name):$TAG"
		}]
		manifestCopyCommandList: [ for k, v in #args.fromTo {(_#copyImage & {#args: v}).res}]
	}
	name:  "Copy Manifest"
	shell: "bash {0}"
	run:   strings.Join(#args.manifestCopyCommandList, "\n")
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

_#redhatDockerLogin: (_#registryLoginStep & {
	#args: {
		registry: "registry.redhat.io"
		user:     "${{secrets['REDHAT_IO_USER']}}"
		pass:     "${{secrets['REDHAT_IO_PASSWORD']}}"
	}
}).res

_#defineImage: {
	#args: {
		image: _#image
	}
	res: """
## getting image shas \(#args.image.name)
shas="$(skopeo inspect docker://\(_#registry)/\(#args.image.name):$TAG --raw | jq -r '.manifests[].digest' | xargs)"
for sha in $shas; do
export IMAGES="--images \(#args.image.url),${sha},$TAG $IMAGES"
done
"""
}

_#publishOperatorImages: _#step & {
	#args: {
		names: [ for v in _#projectURLs {
			"--images \(v),,$TAG"
		}]
	}
	name: "Publish Operator Images"
	env: {
		RH_USER:          "${{ secrets['REDHAT_IO_USER'] }}"
		RH_PASSWORD:      "${{ secrets['REDHAT_IO_PASSWORD'] }}"
		RH_CONNECT_TOKEN: "${{ secrets.redhat_api_key }}"
	}
	run: """
		make pc-tool
		./bin/partner-connect-tool publish --username $RH_USER --password $RH_PASSWORD  \(strings.Join(#args.names, " "))
		"""
}

_#operatorImageStatuses: _#step & {
	#args: {
		names: [ for v in _#projectURLs {
			"--images \(v),,$TAG"
		}]
	}
	id:   "operatorImageStatuses"
	name: "Fetch Operator Image Statuses"
	env: {
		RH_USER:          "${{ secrets['REDHAT_IO_USER'] }}"
		RH_PASSWORD:      "${{ secrets['REDHAT_IO_PASSWORD'] }}"
		RH_CONNECT_TOKEN: "${{ secrets.redhat_api_key }}"
	}
	run: """
		make pc-tool
		OUTPUT=$(./bin/partner-connect-tool status --username $RH_USER --password $RH_PASSWORD \(strings.Join(#args.names, " ")))
		echo "::set-output name=imageStatus::$OUTPUT"
		"""
}

_#publishOperator: _#step & {
	name: "Publish Operator"
	env: {
		RH_USER:          "${{ secrets['REDHAT_IO_USER'] }}"
		RH_PASSWORD:      "${{ secrets['REDHAT_IO_PASSWORD'] }}"
		RH_CONNECT_TOKEN: "${{ secrets.redhat_api_key }}"
	}
	run: """
make pc-tool
./bin/partner-connect-tool publish --username $RH_USER --password $RH_PASSWORD --is-operator-manifest=true --images \(_#manifest.url),,$TAG
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

_#githubGraphQLQuery: {
	_#args: {
		id: string
		with: {_, ...}
	}
	res: _#step & {
		uses: "octokit/graphql-action@v2.x"
		id:   _#args.id
		with: _#args.with
		env: "GITHUB_TOKEN": "${{ secrets.GITHUB_TOKEN }}"
	}
}

_#findPRForComment: {
	_#args: {
		prNum: string
	}
	res: _#step & {
		id:   "pr"
		name: "Get PR for Comment"
		run:  """
PR=$(curl -H "Accept: application/vnd.github.v3+json" \\
     "${GITHUB_API_URL}/repos/${GITHUB_REPOSITORY}/pulls/\( _#args.prNum )" )
echo "::set-output name=prSha::$(echo $PR | jq -r \".head.sha\")"
echo "::set-output name=prRef::$(echo $PR | jq -r \".head.ref\")"
"""
	}
}

_#findAllReleasePRs: (_#githubGraphQLQuery & {
	_#args: {
		id: "findAllReleasePRs"
		with: {
			query: """
				query {
					search(query: "repo:redhat-marketplace/redhat-marketplace-operator is:pr is:open head:hotfix head:release base:master label:ready", type: ISSUE, last: 100) {
						edges {
							node {
								... on PullRequest {
									id
									number
									baseRefName
									headRefName
								}
							}
						}
					}
				}
				"""
		}
	}
}).res

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

_#addRocketToComment: _#step & {
	uses: "peter-evans/create-or-update-comment@v1"
	with: {
		"comment-id": "${{github.event.comment.id}}"
		reactions:    "rocket"
	}
}

_#addFailureComment: _#step & {
	uses: "peter-evans/create-or-update-comment@v1"
	if:   "${{ failure() }}"
	with: {
		"comment-id": "${{github.event.comment.id}}"
		reactions:    "-1"
	}
}
