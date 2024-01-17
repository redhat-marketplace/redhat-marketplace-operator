# Current Operator version
UNAME_S := $(shell uname -s)
UNAME := $(shell echo `uname` | tr '[:upper:]' '[:lower:]')
OPENSHIFT_VERSIONS ?= v4.11-v4.14
CHANNELS ?= beta,stable
DEFAULT_CHANNEL ?= stable

include ../../utils.Makefile


# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium

TAG ?= $(VERSION)

OPERATOR_IMAGE_NAME = ibm-data-reporter-operator
OPERATOR_IMAGE_TAG ?= $(TAG)
OPERATOR_IMAGE ?= $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_IMAGE_TAG)

BUNDLE_NAME ?= ibm-data-reporter-operator-manifest
BUNDLE_IMG ?= $(IMAGE_REGISTRY)/$(BUNDLE_NAME):$(VERSION)

DEV_INDEX_NAME ?= ibm-data-reporter-operator-dev-index
DEV_INDEX_IMG ?= $(IMAGE_REGISTRY)/$(DEV_INDEX_NAME)

OPM_BASE_NAME ?= opm-base
OPM_BASE_IMG ?= $(IMAGE_REGISTRY)/$(OPM_BASE_NAME)

namespace ?= redhat-marketplace

IMG ?= $(OPERATOR_IMAGE)
IMAGE ?= $(OPERATOR_IMAGE)

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
# BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS) --extra-service-accounts ibm-data-reporter-operator-api

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: svu manager

clean:
	rm -f bundle/manifests/*.yaml bundle/metadata/*.yaml bundle/tests/*.yaml

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

#ok
.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

#ok
.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

#ok
.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

#ok, maybe need -stdmethods=false
.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

#this is different
.PHONY: test
test: manifests generate fmt vet envtest ginkgo ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GINKGO) -r -timeout 1m -coverprofile=cover-unit.out --randomize-all --randomize-suites --cover --show-node-events --v --trace ./...

##@ Build

.PHONY: build
build:
	$(call multiarch-build,ibm-data-reporter-operator)

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go

# If you wish built the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64 ). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(call docker-templated-build,$(IMAGE),DataReporter,./datareporter/v2/,./,ibm-data-reporter-operator,../..)

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push $(IMAGE)



##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	$(MAKE) kustomize-set
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -



#KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
#.PHONY: kustomize
#kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
#$(KUSTOMIZE): $(LOCALBIN)
#	test -s $(LOCALBIN)/kustomize || { curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); }

BUNDLE_IMAGE_REGISTRY ?= $(IMAGE_REGISTRY)
VERSIONS_LIST ?= $(BUNDLE_IMG)

opm-builder-base: opm
	docker buildx build --push \
	--build-arg REGISTRY=$(IMAGE_REGISTRY) \
	--build-arg OPM_VERSION=$(OPM_VERSION) \
	--platform $(subst $(space),$(comma),$(addprefix linux/,$(ARCHS)))	 \
	-f hack/opm-builder.Dockerfile -t "$(OPM_BASE_IMG):$(OPM_VERSION)" \
	--progress plain \
	.

CSV_YAML_BUNDLE_FILE=bundle/manifests/ibm-data-reporter-operator.clusterserviceversion.yaml
ANNOTATIONS_YAML_BUNDLE_FILE=bundle/metadata/annotations.yaml

.PHONY: bundle
bundle: clean manifests kustomize operator-sdk yq omt ## Generate bundle manifests and metadata, then validate generated files.
	$(MAKE) kustomize-set
	$(OPERATOR_SDK) generate kustomize manifests -q
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	$(YQ) eval -i '.annotations."com.redhat.openshift.versions" = "$(OPENSHIFT_VERSIONS)"' $(ANNOTATIONS_YAML_BUNDLE_FILE)
	$(YQ) eval -i ".metadata.annotations.containerImage = \"$(OPERATOR_IMAGE)\"" $(CSV_YAML_BUNDLE_FILE)
	$(YQ) eval -i ".metadata.annotations.createdAt = \"$(CREATED_TIME)\"" $(CSV_YAML_BUNDLE_FILE)
	$(YQ) eval -i '.metadata.annotations."olm.skipRange" = ">=1.0.0 <$(VERSION)"' $(CSV_YAML_BUNDLE_FILE)
	$(YQ) eval -i '.metadata.annotations.capabilities = "Seamless Upgrades"' $(CSV_YAML_BUNDLE_FILE)
	$(OMT) pinning pin ./bundle/manifests
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-build
bundle-build:
	docker buildx build -f bundle.Dockerfile -t $(BUNDLE_IMG) \
	--label com.redhat.openshift.versions="$(OPENSHIFT_VERSIONS)" \
  --label com.redhat.delivery.operator.bundle=true \
	--push \
	.

catalog-build: opm
	PATH="$(PWD)/../../bin:${PATH}" ../../v2/scripts/opm_build_catalog.sh $(BUNDLE_IMG) ./README.md

.PHONY: bundle-dev-index-multiarch
 bundle-dev-index-multiarch: opm opm-builder-base
	$(OPM) index add -u docker --generate --bundles "$(foreach vers,$(VERSIONS_LIST),$(call get-image-sha,$(vers)))" -d dev-index.Dockerfile
	docker buildx build --push -f custom-index.Dockerfile -t "$(DEV_INDEX_IMG):$(VERSION)" \
	--platform $(subst $(space),$(comma),$(addprefix linux/,$(ARCHS))) \
	--build-arg REGISTRY=$(IMAGE_REGISTRY) \
	--build-arg OPM_VERSION=$(OPM_VERSION) \
	.

.PHONY: bundle-stable
bundle-stable:
	$(MAKE) bundle CHANNELS=stable DEFAULT_CHANNEL=stable IMAGE_REGISTRY=$(BUNDLE_IMAGE_REGISTRY)

skaffold-build: skaffold
	cd .. && $(MAKE) save-licenses
	$(SKAFFOLD) build --default-repo $(IMAGE_REGISTRY) --tag $(TAG)

skaffold-dev: skaffold kustomize yq
	PATH="$(PWD)/../bin:${PATH}" $(SKAFFOLD) dev --default-repo $(IMAGE_REGISTRY) --cleanup=false --cache-artifacts=false

skaffold-run: skaffold yq
	PATH="$(PWD)/../bin:${PATH}" $(SKAFFOLD) run --default-repo $(IMAGE_REGISTRY) --tag $(TAG)

skaffold-delete: skaffold yq
	PATH="$(PWD)/../bin:${PATH}" $(SKAFFOLD) delete --default-repo $(IMAGE_REGISTRY)

current-version:
	@echo $(VERSION)

kustomize-set:
	cd config/default && \
	${KUSTOMIZE} edit set namespace $(namespace) && \
	cd ../.. && \
	cd config/manager && \
	$(KUSTOMIZE) edit set image $(OPERATOR_IMAGE_NAME)=$(IMAGE_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(IMAGE_TAG) && \
	$(KUSTOMIZE) edit set annotation \
	operatorImage:$(IMAGE_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(IMAGE_TAG) \
	rbacProxyImage:$(KUBE_RBAC_PROXY_IMAGE)

# Pin images in bundle
.PHONY: bundle-pin-images
bundle-pin-images:
	docker run \
	--pull always \
	-v ${HOME}/.docker:/dockercfg \
	-v $(shell pwd)/bundle/manifests:/manifests quay.io/operator-framework/operator-manifest-tools:latest \
	pinning pin -a /dockercfg/config.json /manifests

# Run certification test
.PHONY: test-certify
test-certify: bundle bundle-pin-images
	./../../hack/certify/catsource.sh && \
	./../../hack/certify/certify.sh ibm-data-reporter-operator

# export TWISTLOCK_URL and TWISTLOCK_CREDENTIALS in local environment
# TWISTLOCK_CREDENTIALS is w3id:w3password or w3id:apiKey
# apiKey can be generated by (tt api-key show)
security-scan: install-twistlock-tool
	$(IBM_TWISTLOCK_TOOL) images local-scan \
	--url $(TWISTLOCK_URL) \
	--user $(TWISTLOCK_CREDENTIALS) \
	--control-group eal_eal-006818 \
	${IMAGE}