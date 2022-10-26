# Current Operator version
UNAME_S := $(shell uname -s)
UNAME := $(shell echo `uname` | tr '[:upper:]' '[:lower:]')
OPENSHIFT_VERSIONS ?= v4.6-v4.10
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

OPERATOR_IMAGE_NAME = redhat-marketplace-deployer
OPERATOR_IMAGE_TAG ?= $(TAG)
OPERATOR_IMAGE ?= $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_IMAGE_TAG)

AUTHCHECK_IMAGE_NAME ?= redhat-marketplace-authcheck
AUTHCHECK_IMAGE_TAG ?= $(TAG)
AUTHCHECK_IMAGE := $(IMAGE_REGISTRY)/$(AUTHCHECK_IMAGE_NAME):$(AUTHCHECK_IMAGE_TAG)

BUNDLE_NAME ?= redhat-marketplace-deployer-manifest
BUNDLE_IMG ?= $(IMAGE_REGISTRY)/$(BUNDLE_NAME):$(VERSION)

DEV_INDEX_NAME ?= redhat-marketplace-deployer-dev-index
DEV_INDEX_IMG ?= $(IMAGE_REGISTRY)/$(DEV_INDEX_NAME)

OPM_BASE_NAME ?= opm-base
OPM_BASE_IMG ?= $(IMAGE_REGISTRY)/$(OPM_BASE_NAME)

namespace ?= openshift-redhat-marketplace

# Image URL to use all building/pushing image targets
IMG ?= $(OPERATOR_IMAGE)
IMAGE ?= $(OPERATOR_IMAGE)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
# CRD_OPTIONS ?= "crd:trivialVersions=true"
CRD_OPTIONS ?= "crd:crdVersions={v1}"

GOPATH=$(shell go env GOPATH)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.DEFAULT_GOAL := all

all: svu manager

clean:
	rm -f bundle/manifests/*.yaml bundle/metadata/*.yaml bundle/tests/*.yaml

# Run tests
.PHONY: test
test: fmt vet ginkgo
	$(GINKGO) -r -coverprofile=cover-unit.out --randomizeAllSpecs --randomizeSuites --cover --race --progress --trace --v ./pkg ./cmd ./internal ./apis ./controllers

.PHONY: test-ci-unit
test-ci-unit: ginkgo ## test-ci-unit runs all tests for CI builds
	$(GINKGO) -r -coverprofile=cover-unit.out --randomizeAllSpecs --randomizeSuites --cover --race --progress --trace --v ./pkg ./cmd ./internal ./apis ./controllers
	cat cover-unit.out | grep -v "_generated.go|zz_generated|testbin.go|wire_gen.go" > cover-unit.out

KUTTL_INT_TESTS ?= "(^register-test$$|^features-test$$)"

.PHONY: test-ci-int
test-ci-int:  ## test-ci-int runs all tests for CI builds
	SKIP_DEPLOY=true kubectl kuttl test --namespace openshift-redhat-marketplace --kind-context test --config ./kuttl-test.yaml ../tests/v2/e2e --test $(KUTTL_INT_TESTS) --suppress-log "events"

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize helm install
	cd config && $(HELM) template rhm-template ./helm-wrapper \
	-f ./helm-wrapper/values.yaml \
	--set operatorImage=$(OPERATOR_IMAGE) \
	--set authCheckImage=$(AUTHCHECK_IMAGE) | ./default/kustomize | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) object:headerFile="hack/boilerplate.go.txt" rbac:roleName=deployer-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet -stdmethods=false ./...

# Generate code
generate: controller-gen manifests
	go generate ./...

# Generate API
generate-api: code-generator
	@{ \
	set -e ;\
	API_TEMP_DIR=$$(mktemp -d) ;\
	$(CODEGEN_PKG)/generate-groups.sh \
		"defaulter,client,lister" \
    github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated \
    github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis \
    "marketplace:v1alpha1,v1beta1" \
    --output-base $$API_TEMP_DIR \
    --go-header-file hack/boilerplate.go.txt ; \
	cp -r "$$API_TEMP_DIR/github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated" "apis/marketplace" ;\
	}

QUAY_EXPIRATION?=never
IMAGE_PUSH ?= true
BUILDX ?= true
ARCH ?= amd64

ifeq ($(BUILDX),true)
DOCKERCMD=docker buildx
ifeq ($(IMAGE_PUSH),true)
ARGS=--push
else
ARGS=--load
endif
else
DOCKERCMD=docker
endif

build:
	$(call multiarch-build,redhat-marketplace-deployer)

# Build the docker image
docker-build:
	$(call docker-templated-build,$(IMAGE),Deployer,./deployer/v2/,./,redhat-marketplace-deployer,../..)

# Push the docker image
docker-push:
	docker push ${IMG}


docker-manifest:
	docker manifest create $(IMAGE) $(addprefix $(IMAGE)-,$(ARCHS))
	docker manifest push $(IMAGE)

CSV_YAML_CONFIG_FILE=config/manifests/bases/redhat-marketplace-deployer.clusterserviceversion.yaml
CSV_YAML_BUNDLE_FILE=bundle/manifests/redhat-marketplace-deployer.clusterserviceversion.yaml
ANNOTATIONS_YAML_BUNDLE_FILE=bundle/metadata/annotations.yaml
CREATED_TIME ?= $(shell date +"%FT%H:%M:%SZ")

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: clean manifests kustomize helm operator-sdk yq omt
	$(OPERATOR_SDK) generate kustomize manifests -q
	$(HELM) template rhm-template ./config/helm-wrapper -f ./config/helm-wrapper/values.yaml \
	--set operatorImage=$(OPERATOR_IMAGE) \
	--set authCheckImage=$(AUTHCHECK_IMAGE) \
	--post-renderer ./config/manifests/kustomize | $(OPERATOR_SDK) generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	$(YQ) eval -i '.annotations."com.redhat.openshift.versions" = "$(OPENSHIFT_VERSIONS)"' $(ANNOTATIONS_YAML_BUNDLE_FILE)
	$(YQ) eval -i '.spec.webhookdefinitions[].targetPort = 9443' $(CSV_YAML_BUNDLE_FILE)
	$(YQ) eval -i '.spec.webhookdefinitions[].containerPort = 9443' $(CSV_YAML_BUNDLE_FILE)
	$(YQ) eval -i ".metadata.annotations.containerImage = \"$(OPERATOR_IMAGE)\"" $(CSV_YAML_BUNDLE_FILE)
	$(YQ) eval -i ".metadata.annotations.createdAt = \"$(CREATED_TIME)\"" $(CSV_YAML_BUNDLE_FILE)
	$(YQ) eval -i '.metadata.annotations."olm.skipRange" = ">=1.0.0 <$(VERSION)"' $(CSV_YAML_BUNDLE_FILE)
	$(YQ) eval -i '.metadata.annotations.capabilities = "Seamless Upgrades"' $(CSV_YAML_BUNDLE_FILE)
	$(OMT) pinning pin ./bundle/manifests
	$(OPERATOR_SDK) bundle validate ./bundle
	rm -rf ./deploy && mkdir -p ./deploy ./deploy/crds

BUILD_ARCHS = $(subst $(space),$(comma),$(addprefix linux/,$(ARCHS)))

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	docker buildx build -f bundle.Dockerfile -t $(BUNDLE_IMG) \
	--label com.redhat.openshift.versions="v4.6" \
  --label com.redhat.delivery.operator.bundle=true \
	--push \
	.

bundle-deploy:
	docker push $(BUNDLE_IMG)

bundle-run: operator-sdk
	$(OPERATOR_SDK) run bundle $(BUNDLE_IMG) --namespace openshift-redhat-marketplace

bundle-run-install:
	$(MAKE) skaffold-build bundle bundle-build bundle-deploy bundle-run

bundle-run-uninstall:
	for NAME in marketplaceconfigs.marketplace.redhat.com subscriptions.operators.coreos.com csv operatorgroup catalogsources.operators.coreos.com ; do \
		kubectl delete $$NAME --all -n openshift-redhat-marketplace ; \
	done

BUNDLE_IMAGE_REGISTRY ?= $(IMAGE_REGISTRY)

bundle-stable:
	$(MAKE) bundle CHANNELS=stable DEFAULT_CHANNEL=stable IMAGE_REGISTRY=$(BUNDLE_IMAGE_REGISTRY)

VERSIONS_LIST ?= $(BUNDLE_IMG)

opm-builder-base: opm
	docker buildx build --push \
	--build-arg REGISTRY=$(IMAGE_REGISTRY) \
	--build-arg OPM_VERSION=$(OPM_VERSION) \
	--platform $(subst $(space),$(comma),$(addprefix linux/,$(ARCHS)))	 \
	-f hack/opm-builder.Dockerfile -t "$(OPM_BASE_IMG):$(OPM_VERSION)" \
	--progress plain \
	.

.PHONY:  bundle-dev-index-multiarch
 bundle-dev-index-multiarch: opm opm-builder-base
	$(OPM) index add -u docker --generate --bundles "$(foreach vers,$(VERSIONS_LIST),$(call get-image-sha,$(vers)))" -d dev-index.Dockerfile
	docker buildx build --push -f custom-index.Dockerfile -t "$(DEV_INDEX_IMG):$(VERSION)" \
	--platform $(subst $(space),$(comma),$(addprefix linux/,$(ARCHS))) \
	--build-arg REGISTRY=$(IMAGE_REGISTRY) \
	--build-arg OPM_VERSION=$(OPM_VERSION) \
	.

bundle-dev-index: opm
	$(OPM) index add -u docker --bundles "$(foreach vers,$(VERSIONS_LIST),$(call get-image-sha,$(vers)))" --tag "$(DEV_INDEX_IMG):$(VERSION)" --permissive
	docker push $(DEV_INDEX_IMG):$(VERSION)

bundle-uninstall:
	- kubectl delete catalogsources.operators.coreos.com redhat-marketplace-operator-catalog
	#- kubectl delete marketplaceconfigs.marketplace.redhat.com --all
	- kubectl delete subscription -l operators.coreos.com/redhat-marketplace-operator.openshift-redhat-marketplace
	- kubectl delete csv -l operators.coreos.com/redhat-marketplace-operator.openshift-redhat-marketplace

skaffold-build: skaffold
	cd .. && $(MAKE) save-licenses
	$(SKAFFOLD) build --default-repo $(IMAGE_REGISTRY) --tag $(TAG)

INSTALL_DEBUG ?= false

compile-helm: helm
	cd config && PATH="$(PWD)/../bin:${PATH}" $(HELM) template rhm-operator ./helm-wrapper \
	   --namespace $(namespace) \
	   -f ./helm-wrapper/values.yaml \
	   --set operatorImage=$(OPERATOR_IMAGE) \
	   --set authCheckImage=$(AUTHCHECK_IMAGE) \
     --post-renderer ./helm-wrapper/kustomize

install: kustomize compile-helm yq
	cd config && PATH="$(PWD)/../bin:${PATH}" $(HELM) template rhm-operator ./helm-wrapper \
	   --namespace $(namespace) \
	   -f ./helm-wrapper/values.yaml \
	   --set operatorImage=$(OPERATOR_IMAGE) \
	   --set authCheckImage=$(AUTHCHECK_IMAGE) \
     --post-renderer ./helm-wrapper/kustomize --debug=$(INSTALL_DEBUG) | kubectl apply -f -

uninstall: helm yq
	cd config && PATH="$(PWD)/../bin:${PATH}" $(HELM) template rhm-operator ./helm-wrapper \
	   --namespace $(namespace) \
	   -f ./helm-wrapper/values.yaml \
	   --set operatorImage=$(OPERATOR_IMAGE) \
	   --set authCheckImage=$(AUTHCHECK_IMAGE) \
     --post-renderer ./helm-wrapper/kustomize | $(YQ) eval-all 'select(.kind != "Namespace") | select(.kind != "CustomResourceDefinition")' -  | kubectl delete -f -

skaffold-dev: skaffold kustomize yq
	PATH="$(PWD)/../bin:${PATH}" $(SKAFFOLD) dev --default-repo $(IMAGE_REGISTRY) --cleanup=false

skaffold-run: skaffold yq
	PATH="$(PWD)/../bin:${PATH}" $(SKAFFOLD) run --default-repo $(IMAGE_REGISTRY) --tag $(TAG)

skaffold-delete: skaffold yq
	PATH="$(PWD)/../bin:${PATH}" $(SKAFFOLD) delete --default-repo $(IMAGE_REGISTRY)

current-version:
	@echo $(VERSION)

TAG ?= ""
CREDS ?= ""
TIMEOUT ?= 15
PIDs ?=

wait-and-publish:
	cd tools/connect ; \
	go run main.go wait-and-publish --timeout $(TIMEOUT) --tag $(TAG) $(PIDS)

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
	cd hack/certify && ./certify.sh
